# Loop 0 — Scope and Build Plan

**Goal:** a running Go server that can create a session, issue a walk-in token,
transition an appointment's state, and show the queue — tested against a real Postgres.

No auth. No UI. No notifications. No deploy. Those are Loops 1–3.

**Loop 0 is done when:** `make up && make test` is green, `make run` serves the four
endpoints, the curl flow works end to end, and the concurrency bug has been observed
and written down (not fixed).

---

## Progress

| # | Step | State |
|---|---|---|
| 1 | Postgres up via Docker Compose + Makefile | **done** |
| 2 | Secrets into `.env`, `.env.example` committed | **done** |
| 3 | Go skeleton: layout, config, slog, pgxpool, `/healthz`, graceful shutdown | **done** |
| 4 | `goose` migrations + schema v1 | **done** |
| 5 | `internal/domain`: types, states, `Transition(from, to) error` + unit tests | **done** |
| 6 | `internal/store`: the SQL | **current** |
| 7 | `internal/http`: the four endpoints | todo |
| 8 | Integration tests against real Postgres (testcontainers-go) | todo |
| 9 | Break it: 20 concurrent walk-ins, observe, do not fix | todo |

Steps are in dependency order. Do not jump.

---

## Environment

| | |
|---|---|
| OS | macOS (Darwin, arm64, Apple Silicon) |
| Go | 1.26.4 |
| Module | `github.com/veerbal/opdq` |
| Docker | 29.5.3, Compose v5.1.4 |
| Postgres | `postgres:18.6-alpine`, container `postgres_db`, host port 5432 |
| Credentials | user `opd`, password `12345678`, db `opd_db` |

---

## Design decisions already settled

Derived from the storyline, not copied from the roadmap's schema.

- **Four tables:** `clinics`, `doctors`, `sessions`, `appointments`.
  The session exists because "starts 17:00, capacity 20, delayed 40 min" needs an owner
  that is neither the doctor (his morning must not move) nor the date (it holds both sittings).

  ```
  clinics       (id, name)
  doctors       (id, clinic_id, name)
  sessions      (id, clinic_id, doctor_id, session_date, starts_at, ends_at,
                 capacity, delay_min, status, version)
  appointments  (id, clinic_id, session_id, token_no, patient_name, contact,
                 queued_at, priority, state, version)
  ```

  Nothing else on `doctors` yet — no speciality, no photo, no availability.
  Relations first; the rest when something actually needs it.

- **The rule for what to build now vs later: is it cheap or expensive to add afterwards?**
  - `doctors` as a table instead of a text column — *cheap* later. One migration.
  - `clinic_id` on every table — *expensive* later. Every query, every index, every test,
    and Loop 5's RLS is impossible without it. One forgotten filter leaks another clinic's
    data. So it goes in on day one, even with a single clinic.

- **`clinic_id` is deliberately redundant on `sessions` and `appointments`.**
  It could be reached by joining through `doctors`, but copying it removes two joins from
  the hottest read, and Loop 5's RLS policies need it present on the row itself.
  The copy is kept honest by the database, not by application code:

  ```sql
  FOREIGN KEY (clinic_id, session_id) REFERENCES sessions (clinic_id, id)
  ```

  A row claiming to belong to clinic 7 while sitting in clinic 5's session cannot be inserted.
  **Rule: redundancy is fine only when the database itself guarantees it stays true.**

- **Three columns doing three different jobs.** They collapse into one on a boring day
  and pull apart the moment a human does something human:
  - `token_no` — identity. Printed on paper. Never changes.
  - `queued_at` — place in line. A true fact. Reset whenever someone re-enters `waiting`.
  - `priority` — line jumping. 0 normal, 1 emergency.

- **Queue order:** `ORDER BY priority DESC, queued_at ASC`.
  Two emergencies sort among themselves by arrival — no third column needed.

- **`UNIQUE (session_id, token_no)`** — the domain rule enforced by the database rather
  than by application discipline. A lock is code you can forget to write; a constraint is not.

- **State machine — 4 states, 5 legal transitions, 7 refused:**

  ```
  waiting ──────▶ in_consultation ──────▶ done
     │  ▲               │
     │  │               └── send back ──▶ waiting   (report not ready)
     ▼  │ re-check-in
   absent
  ```

  | from | to | allowed |
  |---|---|---|
  | waiting | in_consultation | yes |
  | waiting | absent | yes |
  | in_consultation | done | yes |
  | in_consultation | waiting | yes (send back) |
  | absent | waiting | yes (re-check-in, resets `queued_at`) |
  | *everything else* | | **no** |

  Written as an allowlist: anything not listed is refused. A denylist silently permits
  every state added later.

- **ETA is derived on read, never stored.**
  `eta(token) = max(session_start + delay, now) + (waiting ahead) × avg_consult_sec`.
  One write, many reads. (No ETA endpoint in Loop 0 — this is here so nobody stores it.)

- **Capacity is soft for walk-ins, hard for online booking.**
  A receptionist is a human standing there who can say "full hai, do ghante lagenge."
  Online booking at 2am has nobody. Hard limits belong where human judgement is absent.
  Online booking is Loop 2, so **Loop 0 enforces no capacity at all.**

- **`appointment_events` is the truth; `appointments.state` is a fast copy of it.**
  Append-only, INSERT-only at the DB grant level. Both writes in one transaction.
  *Design settled, but the table is built in Loop 1 — Loop 0 has no actor to record.*

---

## Design work still to do

- [ ] **Access-pattern list** → `docs/access-patterns.md`. Every query the system will run.
      Track A says this comes *before* the tables; we did it backwards and must catch up.
      Format per pattern — the fields are the index recipe, in index-column order:

      ```
      #N   name
           Trigger:  who fires it, how often
           Equality: columns compared with =
           Range:    columns compared with <, >, BETWEEN
           Sort:     ORDER BY
           Returns:  1 row / N rows / aggregate
           Budget:   latency target
      ```

      At minimum: queue for a session ordered · appointment by id · next token number for a
      session · today's sessions for a clinic · session by id · doctors for a clinic.

- [ ] **Indexes** — one per access pattern, each justified by a named query.
      Composite column order: equality first, then range, then sort.

- [ ] **Column types and constraints** (rules, not the DDL — he writes that):
      - `timestamptz` always, never `timestamp`
      - `NOT NULL` by default; nullable only where genuinely unknown
        (`started_at`, `completed_at` before they happen)
      - `bigint generated always as identity` for internal PKs
      - a separate random `public_id` only where a value leaves the system
      - `CHECK` on `state` and `priority`
      - foreign keys on, including the composite ones
      - `UNIQUE (session_id, token_no)`
      - naming: snake_case, plural tables, `_at` for timestamps, `_id` for references
      - **`sessions` and `doctors` each need `UNIQUE (clinic_id, id)`** so the composite
        foreign keys have a target to point at

- [ ] **Error design** — sentinel errors in `domain`:
      `ErrIllegalTransition`, `ErrSessionNotFound`, `ErrAppointmentNotFound`.
      Mapped to HTTP status in **exactly one place**. One error response shape everywhere.

---

# Build plan

## Step 1 — Postgres up ✅ done

`compose.yaml` + `Makefile` (`up`, `down`, `logs`, `psql`).
Postgres `18.6-alpine`, healthcheck via `pg_isready`, named volume.

**Traps already hit:**
- Postgres 18+ mounts at `/var/lib/postgresql`, **not** `/var/lib/postgresql/data`.
- The file must be `Makefile`, not `MakeFile` — macOS is case-insensitive, Linux CI is not.
- `✔ Started` does not mean running. Ladder: `docker compose ps` → `docker ps -a` → `docker logs`.

---

## Step 2 — Secrets into `.env` ✅ done

1. `.env` — real values. Gitignored.
2. `.env.example` — same keys, fake values. **Committed**, so a fresh clone knows what is needed.
3. `compose.yaml` — replace hardcoded values with `${POSTGRES_USER}` etc.
   Compose auto-loads `.env` from the same directory; no extra config needed.
4. `.env` also carries the URL the Go app will use:

   ```
   DATABASE_URL=postgres://opd:12345678@localhost:5432/opd_db?sslmode=disable
   ```

   Shape: `postgres://user:password@host:port/dbname?options`.
   One URL rather than five variables because that is what `pgx` takes, and what a managed
   database hands you in production. `sslmode=disable` is local-only; Loop 1 uses `require`.

**Rule:** the repo holds the *name* of a secret, never its *value*.

**Done when:** `make down && make up` still comes up healthy with nothing hardcoded in `compose.yaml`.

---

## Step 3 — Go skeleton

**Target:** `make run` starts the server, connects to Postgres, logs "connected",
answers `GET /healthz`, and shuts down cleanly on Ctrl+C.

### Layout

```
cmd/server/main.go        wiring only — zero business logic
internal/config/          read env, validate, fail fast
internal/domain/          the rules. Pure Go. No DB, no HTTP.
internal/store/           Postgres. All SQL lives here.
internal/http/            handlers. Request in, JSON out.
migrations/
```

Dependency arrow points **one way**:

```
http  ───▶  store  ───▶  domain
                          ▲  domain imports nothing of ours
```

`domain` must not import `store`, `pgx`, or `net/http`. If it does, its tests suddenly need
a database, and the fastest tests in the project become the slowest.

`internal/` so nothing outside this module can import it. Do **not** over-nest —
`internal/domain/appointment/state/` is the opposite failure mode. Split a package only
when it has two reasons to change.

### Dependencies

```
github.com/jackc/pgx/v5          // pgxpool
```

`log/slog` and `net/http` are stdlib — no framework.

### config

Read `DATABASE_URL` and `PORT` (default `8080`). Validate at startup.
If `DATABASE_URL` is missing, print a clear message and `os.Exit(1)` — **do not** start and
fail later on the first request. A server that boots into a broken state is worse than one
that refuses to boot.

### logging

`log/slog` with the JSON handler, set as the default logger. Structured fields, never
`fmt.Printf`. No contact details in logs, ever — that rule starts now, not in Loop 5.

### database

`pgxpool.New(ctx, databaseURL)`, then `pool.Ping(ctx)` with a timeout so a wrong URL fails
in seconds rather than hanging. **Every** DB call gets a `context` with a timeout.

### HTTP server

Go 1.22+ `http.ServeMux` with method patterns (`"POST /sessions"`). No router library.
Set `ReadHeaderTimeout` on `http.Server` — the default is no timeout, which is a slowloris
invitation.

### graceful shutdown

`signal.NotifyContext` for SIGINT/SIGTERM, then `srv.Shutdown(ctx)` with a timeout, then
`pool.Close()`. In-flight requests finish; new ones are refused.

This looks pointless on a laptop. It is the reason Loop 7's rolling deploys drop zero
requests — Kubernetes sends SIGTERM and waits. Written now, paid off later.

### Makefile

Add `run`. Also add `test` now, with `-race` in it from the very first run:

```
go test -race ./...
```

**Done when:** `make run` logs a connect line, `curl localhost:8080/healthz` returns 200,
Ctrl+C exits without a stack trace, and a wrong `DATABASE_URL` fails immediately with a
readable message.

**Trap hit:** `go run` reports `make: *** [run] Error 1` on Ctrl+C even when the program's
own graceful shutdown succeeded cleanly — `go run` catches SIGINT itself and surfaces a
non-zero status regardless of the child binary's own exit. Confirmed harmless by building
and running the compiled binary directly, which exits clean. Not an issue in production —
the compiled binary is what actually runs there, not `go run`.

**Trap hit:** a native Homebrew Postgres (`postgresql@17`) was already listening on `5432`,
so the Go app connected to it instead of the Docker container and failed with
`role "opd" does not exist` even though the container was healthy and correct. Diagnosed with
`lsof -nP -iTCP:5432 -sTCP:LISTEN`, fixed with `brew services stop postgresql@17`. Two Postgres
instances cannot share one port — check for this before suspecting the container.

---

## Step 4 — Migrations and schema v1

`github.com/pressly/goose/v3`. SQL files in `migrations/`, versioned, with
`-- +goose Up` / `-- +goose Down`.

Write the DDL from the "column types and constraints" rules above, and from the
access-pattern list — **not** by copying the roadmap's schema block.

Tables for Loop 0: `clinics`, `doctors`, `sessions`, `appointments`.
Not `appointment_events` (Loop 1), not `outbox` or `notification_log` (Loop 3).

Makefile: `migrate-up`, `migrate-down`, `migrate-status`.

**Done when:** `\d appointments` in `make psql` shows every constraint you intended, and
`migrate-down` followed by `migrate-up` is clean.

---

## Step 5 — Domain

Pure Go, no imports of ours. This is the package with the fastest tests and the highest
value per line.

- `State` type with the four constants
- `Transition(from, to State) error` — an allowlist map or switch, returning
  `ErrIllegalTransition` for anything not listed
- The appointment and session types
- Sentinel errors

**Test:** table-driven over **all 16 pairs** — 5 legal, 11 illegal (including same→same).
Every illegal edge asserted, not just the happy path. The illegal cases are the point.

**Done when:** `go test -race ./internal/domain/...` passes in milliseconds without Docker.

---

## Step 6 — Store

All SQL here. Signatures take and return `domain` types, never `pgx` rows.

- `CreateSession`
- `CreateWalkIn` — next token via `SELECT MAX(token_no) + 1 WHERE session_id = $1`,
  then `INSERT`. **Naive on purpose.** It will lose races. That is Step 9.
- `TransitionAppointment` — read current state, call `domain.Transition`, write.
  `absent → waiting` and `in_consultation → waiting` also reset `queued_at = now()`.
- `QueueForSession` — `WHERE session_id = $1 AND state = 'waiting'
  ORDER BY priority DESC, queued_at ASC`

`context` with a timeout on every call. Wrap errors with context.

---

## Step 7 — HTTP

Four endpoints, JSON in and out:

```
POST /sessions
  { "clinic_id":1, "doctor_id":1, "session_date":"2026-08-31",
    "starts_at":"10:00", "ends_at":"13:00", "capacity":30 }
  → 201 { "id":8 }

POST /sessions/{id}/walkins
  { "patient_name":"Pooja", "contact":"98...", "priority":0 }
  → 201 { "id":42, "token_no":1, "state":"waiting" }

POST /appointments/{id}/transition
  { "to":"in_consultation" }
  → 200 { "id":42, "state":"in_consultation" }
  → 409 on an illegal transition

GET /sessions/{id}/queue
  → 200 [ { "token_no":4, "patient_name":"Vikram", "priority":1 }, ... ]

GET /healthz
  → 200
```

One error response shape everywhere. Domain errors mapped to HTTP status in exactly one
place — not scattered through the handlers.

Handlers decode, call store, encode. No business logic in handlers.

---

## Step 8 — Integration tests

`testcontainers-go` starts a **real** Postgres per test run. No mocks, no sqlite.
The bugs worth catching live in Postgres's behaviour, and a mock cannot have them.

Run migrations against the container, then exercise each endpoint end to end.
Reset state between tests (truncate, or a fresh schema per test).

`go test -race ./...` — always. Never run the suite without `-race` in this project.

---

## Step 9 — Break it

Fire **20 concurrent** walk-in requests at one session.

`SELECT MAX(token_no)+1` then `INSERT` will hand the same number to several goroutines.
`UNIQUE (session_id, token_no)` will reject the losers with:

```
ERROR: duplicate key value violates unique constraint "appointments_session_id_token_no_key"
```

**Watch it happen. Record how many succeeded, how many failed, and the exact error.**
Write it into `docs/adr/` or a build-log note.

**Do not fix it.** The retry loop, the row lock, the isolation-level comparison — all of it
is Loop 2, and Loop 2 only means anything if this failure was seen first.

---

## Already covered in conversation — do not re-teach unless asked

He derived all of this himself, by being asked what breaks. Several items belong to later
loops; he knows the theory, which is not the same as having built it.

- Why `appointments` is the seed table, and that a row **changes over time** — a state, not a flag
- `token_no` vs `queued_at` vs `priority`: identical on a boring day, they pull apart the
  moment a human does something human (chai break, emergency, sent back for a report)
- Why `sessions` exists — the delay had to belong to a sitting, not a doctor or a date
- ETA derived on read, never stored: one write, many reads
- A constraint is a guarantee; a lock is code you can forget to write
- An idempotency key identifies **the button press**, not the person — born with the form,
  not with the submit
- `COUNT`-then-`INSERT` can never work: a transaction cannot see another's uncommitted rows
- Lock the **parent** row, because the colliding rows do not exist yet
- Lock cost = how many contend × how long they hold. 30 tokens per session ⇒ cheap
- Never make a network call inside a transaction or while holding a lock
- Transactional outbox: memory forgets, the database does not. Intent written in the same
  transaction as the fact
- There is no exactly-once over a network. `effectively-once = at-least-once + idempotency`
- Chose at-least-once deliberately — a duplicate message beats a lost one
- `SKIP LOCKED` so two workers take different rows instead of queueing behind each other
- Lease via `next_attempt_at`: one column doing both retry backoff and crash recovery
- An outbox row is an **intent**, not the truth — re-read current state before sending,
  which also debounces the delay message for free
- The state machine as an allowlist; a denylist silently permits every state added later
- Events are the truth, `state` is a fast copy; both written in one transaction
- Cheap-vs-expensive-to-add-later as the rule for what to build now
- Composite foreign keys — taught via school `(class, roll_no)` → marks
- Capacity is soft where a human stands, hard where nobody does
- A Makefile makes discipline automatic — `-race` can never be forgotten
- `Up` is not `ready`; that is what `pg_isready` and the healthcheck are for
- `✔ Started` does not mean running. The ladder: `docker compose ps` → `docker ps -a` → `docker logs`

---

## Explicitly NOT in Loop 0

He already knows the theory for several of these. **Knowing is not building.**

| Thing | Loop |
|---|---|
| `appointment_events` table, auth, HTMX pages, board, patient page, deploy, TLS, backups, restore | 1 |
| `SELECT ... FOR UPDATE`, isolation levels, the race fix, retry loops, idempotency keys, online booking, capacity enforcement, `booked`/`cancelled` states | 2 |
| Outbox table, worker, dedupe keys, lease, backoff, `SKIP LOCKED` | 3 |
| SSE, Prometheus, tracing, pprof | 4 |
| RLS, partitioning, rate limiting, retention | 5 |
