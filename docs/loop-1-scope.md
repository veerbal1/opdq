# Loop 1 — Scope and Build Plan

**One clinic, walk-ins only, live on the real internet.**

Loop 0 produced a skeleton that only `curl` could talk to. Loop 1 turns it into something a
receptionist could actually run tomorrow's OPD on — with real staff logins, real screens, a
real domain with TLS, real backups, and one real restore.

**Loop 1 is done when:** a clinic *could* run tomorrow's OPD on it. Deployed on a real
domain with TLS, backed up nightly, and restored once from that backup with nothing lost.
You will not actually give it to a clinic.

The roadmap calls the deploy+restore part **non-negotiable**. Most candidates skip it.
That is the difference. Do not skip it.

---

## Progress

| # | Step | State |
|---|---|---|
| 1 | Schema additions: `staff_users`, `auth_sessions`, `appointment_events`, `public_id`, `avg_consult_sec` | done |
| 2 | Events wired into every transition + append-only enforced by DB grants | done |
| 3 | Staff auth: bcrypt, server-side sessions, cookies, middleware | done |
| 4 | CSRF tokens | done |
| 5 | React app (Vite), embedded in the Go binary and served from one origin | done |
| 6 | Receptionist console | current |
| 7 | Board page `/board/{session_id}` | todo |
| 8 | Patient page `/q/{public_id}` + derived ETA + QR | todo |
| 9 | Dockerfile + Caddy + deploy to a real VPS with a real domain | todo |
| 10 | Nightly `pg_dump` to object storage | todo |
| 11 | Break it: kill mid-request, drop the DB, restore, verify | todo |
| 12 | ADR + runbook | todo |

Steps are in dependency order. Do not jump. Steps 9–11 are the ones that matter most and
are the ones people skip — they are not optional.

---

## Carried over from Loop 0 (already built, do not rebuild)

- `clinics`, `doctors`, `sessions`, `appointments` tables with `clinic_id` on every row,
  composite foreign keys, `UNIQUE (session_id, token_no)`, `CHECK` on `state` and `priority`
- `idx_appointments_queue ON appointments (session_id, state, priority DESC, queued_at ASC)`
- `internal/domain` — 4 states, `Transition(from, to) error`, allowlist, 16-case table test
- `internal/store` — `CreateClinic`, `CreateDoctor`, `CreateSession`, `CreateWalkIn`,
  `TransitionAppointment`, `QueueForSession`
- `internal/handler` — JSON endpoints, `writeJSON` / `writeError`, error mapping in one place
- `internal/config` — env, validated at startup
- `cmd/server` — slog JSON, pgxpool, graceful shutdown on SIGTERM (this now earns its keep)
- `contact_channel` + `contact_address` (generic Contact, not a bare phone column)
- Migrations via `goose`, integration tests via `testcontainers-go`, `go test -race`
- **The walk-in token race is still there and still unfixed** — see `docs/adr/0001`.
  Loop 1 does not touch it. That is Loop 2.

---

## Environment

| | |
|---|---|
| OS (dev) | macOS (Darwin, arm64) |
| Go | 1.26.4 |
| Module | `github.com/veerbal/opdq` |
| Postgres | `postgres:18.6-alpine`, container `postgres_db`, host port 5432 |
| Local creds | user `opd`, password `12345678`, db `opd_db` |
| Deploy target | not chosen yet — decide in Step 9 |

**Traps already hit in Loop 0, still true:**
- Postgres 18+ mounts at `/var/lib/postgresql`, not `/var/lib/postgresql/data`
- The file is `Makefile`, not `MakeFile` (macOS is case-insensitive, Linux CI is not)
- `✔ Started` ≠ running. Ladder: `docker compose ps` → `docker ps -a` → `docker logs`

---

## New dependencies for Loop 1

### Go

```
golang.org/x/crypto/bcrypt        password hashing
github.com/skip2/go-qrcode        QR PNG for the walk-in slip
```

`html/template`, `crypto/rand`, `crypto/subtle`, `embed`, `encoding/base64` are stdlib.

### Frontend — Vite + React, NOT Next.js

Decided deliberately. He already knows React, so the screens go faster than with templates.
**Next.js was rejected:** it needs a Node server running beside the Go binary — two runtimes,
two containers, two deploys, two places to crash, and cross-origin auth (`SameSite`, CORS
credentials, preflight) that would add days to Step 3 while teaching nothing about Go or Postgres.

The chosen shape avoids all of that:

```
web/                    Vite + React source
web/dist/               npm run build output
  ↓  embed.FS
Go binary serves the SPA and the API from ONE origin
```

One origin means the session cookie just works — no CORS, no preflight, no `SameSite=None`.
Caddy proxies exactly one thing.

**No CSS framework, no component library, no state-management library.** `fetch` and
`useState` are enough for three screens. The UI gets exactly as much effort as a real
receptionist needs and not one hour more. Ugly and working beats pretty and late — the
backend is what is being graded.

Polling, not websockets: board 5s, patient 10s, via `setInterval` in a `useEffect`.
SSE arrives in Loop 4, and only because polling will have started to hurt.

---

## Design decisions to settle before writing code

These are the questions Loop 1 forces. Decide each one deliberately; several have a trap.

### 1. Naming collision — this one will bite

`sessions` **already means an OPD sitting** in this codebase. Login sessions are a completely
different thing.

> **Name the auth table `auth_sessions`. Never `sessions`.**

A fast model, or a tired human, will reach for `sessions` out of habit and produce a mess
that is expensive to untangle. This is the single most likely mistake in Loop 1.

### 2. Store a hash of the session token, not the token

The cookie holds a random 256-bit token. The database stores **`sha256(token)`**, not the
token itself.

Why: if someone reads the database — a leaked backup, a SQL injection, a curious contractor —
raw tokens are live logins for every signed-in user. Hashes are useless to them.

This costs one line and removes an entire class of incident. Passwords get bcrypt (slow, salted,
because they are low-entropy and human-chosen); session tokens get plain SHA-256 (fast, because
they are already 256 bits of randomness and need no stretching).

### 3. `public_id` is how a patient is identified — there is no patient login

`appointments.public_id` — random 128-bit, unguessable. The patient page is `/q/{public_id}`
and needs no authentication at all. Whoever holds the link is the patient.

- It must be **random**, never sequential. A sequential id lets anyone enumerate every patient.
- It is a **separate column**, not the primary key. (Track A: a random UUID as the PK of a hot
  table fragments the index. As a secondary unique column it is fine.)
- `github.com/google/uuid` is already in `go.mod`.

### 4. ETA is derived on read, never stored

```
eta(token) = max(session_start + delay, now) + (waiting ahead) × avg_consult_sec
```

Add `sessions.avg_consult_sec` with a sensible default (`480` = 8 minutes). Loop 1 uses the
fixed value. A rolling average computed from real `started_at → completed_at` durations comes
much later — do not build it now.

Setting a 40-minute delay is **one column update**. It must never write to the waiting
appointment rows. If a change to `delay_min` causes N row updates, the design is wrong.

### 5. Timezone

`timestamptz` stores an instant; it does not remember which wall clock a human meant.
The board must show `10:00 AM`, not a UTC instant.

Simplest correct choice for Loop 1: a `timezone TEXT NOT NULL DEFAULT 'Asia/Kolkata'` column
on `clinics`, and render every time in that zone. Hardcoding IST in the templates works today
and becomes a bug the day a second clinic exists — and Loop 5 adds a second clinic.

### 6. What is NOT in Loop 1

The receptionist can **set** a delay. Nobody is **notified** about it. Notifications are the
entire subject of Loop 3 and need the outbox to exist first.

Pages **poll** (board 5s, patient 10s). Not SSE. SSE is Loop 4 and only earns its place once
polling demonstrably hurts.

---

# Build plan

## Where the new code goes

Loop 0's layout stays. Loop 1 adds packages, it does not reshuffle. The dependency arrow
still points one way and `domain` still imports nothing of ours.

```
internal/auth/        password hashing, token generation, session create/lookup/delete,
                      RequireAuth middleware, CSRF check
internal/store/       new queries only — staff_users, auth_sessions, appointment_events
internal/handler/     new handlers; auth ones grouped in their own file
internal/web/         embed.go  (//go:embed all:dist)
web/                  the Vite + React app
cmd/seed/             one-shot seeder: clinic + doctor + first admin user
```

**Do not put SQL in `internal/auth`.** It calls `store`, like everything else. The moment
auth grows its own queries the "all SQL lives in one package" rule is gone, and nobody will
put it back.

### Seeding — needed before you can log in even once

There is no signup endpoint and never will be. Something has to create the first row:

```
make seed   →   go run ./cmd/seed
                creates one clinic (with a timezone), one doctor,
                and one admin staff_user from env vars
```

Take the password from an environment variable, never a flag — flags land in shell history
and in `ps` output. Make it idempotent so running it twice is harmless.

---

## Step 1 — Schema additions

One migration, or a small series. New tables and columns:

```
staff_users
  id             bigint identity PK
  clinic_id      bigint NOT NULL  → clinics
  email          text   NOT NULL
  password_hash  text   NOT NULL
  role           text   NOT NULL CHECK (role IN ('admin','receptionist'))
  created_at     timestamptz NOT NULL DEFAULT now()
  UNIQUE (email)          -- GLOBAL, not (clinic_id, email) — see note below

-- Why email is globally unique, not unique per clinic:
-- at the login form there is no clinic_id yet — the user is not authenticated, so there is
-- nothing to scope the lookup by. Making it (clinic_id, email) would force a clinic picker or
-- a subdomain on the login page, which is Loop 5's problem, not Loop 1's. One clinic, one
-- global email. Loop 5 revisits this and writes an ADR when it does.

auth_sessions                    -- NOT "sessions"
  id           bigint identity PK
  token_hash   bytea  NOT NULL UNIQUE      -- sha256 of the cookie value
  user_id      bigint NOT NULL → staff_users
  clinic_id    bigint NOT NULL            -- copied from staff_users to avoid a join on
                                          -- every request; keep it honest with a composite
                                          -- FK (clinic_id, user_id) -> staff_users(clinic_id, id),
                                          -- the same rule the schema already follows elsewhere
  csrf_token   text   NOT NULL
  created_at   timestamptz NOT NULL DEFAULT now()
  expires_at   timestamptz NOT NULL
  INDEX on (expires_at)                    -- for the cleanup job

appointment_events               -- append-only, the truth
  id              bigint identity PK
  clinic_id       bigint NOT NULL
  appointment_id  bigint NOT NULL → appointments
  from_state      text                     -- NULL on creation
  to_state        text   NOT NULL
  actor_id        bigint          → staff_users, NULL for system actions
  at              timestamptz NOT NULL DEFAULT now()
  reason          text
  INDEX on (appointment_id, at)
```

Column additions:

```
appointments.public_id        uuid NOT NULL UNIQUE DEFAULT gen_random_uuid()
appointments.started_at       timestamptz            -- nullable: has not happened yet
appointments.completed_at     timestamptz            -- nullable
appointments.version          int NOT NULL DEFAULT 1
sessions.avg_consult_sec      int NOT NULL DEFAULT 480
clinics.timezone              text NOT NULL DEFAULT 'Asia/Kolkata'
```

**Do NOT add `checked_in_at`.** No state in Loop 1's machine produces it — a walk-in is
created directly as `waiting`. It becomes meaningful in Loop 2 when online booking introduces
`booked → checked_in`. A column nothing writes is a column that will be wrong.

**One more migration nobody expects:** Step 6 has a "Close session" button, but the existing
`sessions_status_check` only allows `('open', 'cancelled')`. Extend it to
`('open', 'closed', 'cancelled')`, the same way migration `20260901114225` extended it before.
`closed` = the day finished normally; `cancelled` = the sitting never happened.

**`staff_users.role`** is stored but **unused in Loop 1** — nothing branches on it yet.
Admin-vs-receptionist permissions are Loop 5. The column exists now only because adding it
later means touching every existing row.

**Backfill note — corrected by what actually happened.** The plan said `public_id` needs three
statements (add column, backfill, then constraint) because `NOT NULL UNIQUE DEFAULT
gen_random_uuid()` in one statement would fail. **It did not fail.** One statement worked and
every row got a distinct UUID.

The real rule is about the default's *volatility*, not the statement count. A non-volatile
default (`DEFAULT 0`) is stored once in the catalog and existing rows are never touched — every
row would share one value and `UNIQUE` would fail. `gen_random_uuid()` is VOLATILE, so Postgres
falls back to a full table rewrite and evaluates the function **per row**, which is exactly what
`UNIQUE` needs.

The three-step version is still right in production, for a different reason: that rewrite takes
an `ACCESS EXCLUSIVE` lock on the whole table. On one row it is instant; on ten million it locks
out every reader and writer for minutes. Split it to avoid **downtime**, not to avoid an error.

**Done when:** `\d` in `make psql` shows every table and constraint; `migrate-down` then
`migrate-up` is clean.

### As built

Five migrations, all with a verified `down → up` round-trip:

```
20260903084408_add_staff_users
20260903085101_add_auth_sessions
20260903090036_add_appointment_events
20260903090842_add_loop1_columns
20260903091250_add_closed_session_status
```

**`appointments` needed a new `UNIQUE (clinic_id, id)`** — not in the plan. `appointment_events`
is the first table to point *at* `appointments`, and a composite FK needs a unique constraint on
its target. In Loop 0 `appointments` was the end of the chain, so nothing had required it. The
failure was explicit: `there is no unique constraint matching given keys for referenced table
"appointments"`. Added in the same migration, above the `CREATE TABLE`.

**`staff_users.name` added** (not in the plan). It fails the usual "expensive to add later" test —
a later `ADD COLUMN name text` backfills fine, no value has to be guessed. It earns its place on
the other rule instead: it has a reader in Loop 1. The seeder writes it and the console header
shows it, so it is not a speculative column. Contrast `checked_in_at`, which is still excluded
because nothing in Loop 1 writes it.

**`appointment_events.session_id` deliberately not added.** Derivable through `appointment_id`,
and backfillable later in one `UPDATE ... FROM`. No Loop 1 query looks up events by session; the
audit viewer that would is Loop 5. `clinic_id` *is* stored despite being equally derivable,
because Loop 5's RLS policies read columns off the row itself and cannot join.

**Known broken, left alone:** the `Down` of Loop 0's `20260901114225_update_session_status`
fails against real data — it restores the old four-state `CHECK` while a live row still holds
`'open'`. Rolling back past that migration therefore requires an empty table, which production
never has. The honest fixes are either a deliberate lossy remap in the `Down`
(`open → scheduled`) or a forward-only migration policy. Neither is Loop 1's budget; Loop 1's
own five migrations all reverse cleanly, and that was the property worth checking.

---

## Step 2 — Events wired in, and append-only enforced by the database

### 2a. Write an event in the same transaction as every state change

`store.TransitionAppointment` currently does a bare `UPDATE`. It must become a transaction:

**A note before you write this, because the scope table below looks like it forbids it:**
`SELECT ... FOR UPDATE` here is fine and correct. What Loop 2 owns is the *token-assignment*
race — `MAX(token_no)+1` followed by an `INSERT`. Locking a row you are about to `UPDATE`,
so you do not read a stale state and overwrite someone else's change, is ordinary transaction
work and belongs here. Do not confuse the two.

```
BEGIN
  SELECT current state FOR UPDATE      -- read-then-write needs the lock
  domain.Transition(from, to)          -- refuse illegal moves before touching anything
  UPDATE appointments SET state, timestamps, version = version + 1
  INSERT INTO appointment_events (from_state, to_state, actor_id, reason)
COMMIT
```

Ya dono, ya koi nahi. The `state` column is a fast copy; the events table is the truth.
If they can drift, the truth is worthless.

`CreateWalkIn` also writes an event: `from_state = NULL, to_state = 'waiting'`.

Timestamps to set on transition: `started_at` on entering `in_consultation`,
`completed_at` on entering `done`, `queued_at = now()` on re-entering `waiting`
(already built in Loop 0 — keep it).

### 2b. Prove the table cannot be edited

This is the part almost everyone skips, and it is the part that is interesting in an interview.

Create a **second, restricted database role** that the application connects as:

```sql
-- migrations run as the owner
-- the app connects as opd_app
CREATE ROLE opd_app LOGIN PASSWORD '...';
GRANT SELECT, INSERT, UPDATE, DELETE ON <normal tables> TO opd_app;
GRANT SELECT, INSERT ON appointment_events TO opd_app;   -- INSERT only. No UPDATE. No DELETE.
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO opd_app;
```

This means **two connection strings**:

```
DATABASE_URL           = the restricted app role      (used by the server)
MIGRATE_DATABASE_URL   = the owner role               (used by goose)
```

`config.Load()` must read both. The Makefile's `migrate-*` targets use the second one.
The integration tests must create both roles in the testcontainer, and the test suite must
connect as the restricted one — otherwise the test proves nothing.

**Then write the test that proves it:**

```
UPDATE appointment_events SET to_state = 'done' WHERE id = 1
  → must fail with a permission error, not succeed
DELETE FROM appointment_events WHERE id = 1
  → must fail
```

A test that asserts a *denial* is worth more than ten that assert success. Application code
can be bypassed by the next endpoint someone adds; a grant cannot.

**Done when:** every transition writes exactly one event, and the two denial tests pass.

### As built

**`TransitionAppointment` and `CreateWalkIn` are both transactions now.** Same shape in each:
`tx, err := s.pool.Begin(ctx)`, then `defer tx.Rollback(ctx)` immediately, then every query
through `tx` instead of `s.pool`, then `tx.Commit(ctx)` last. The deferred rollback is safe
after a successful commit — pgx returns `ErrTxClosed` and does nothing — which is what makes
the idiom work across all five error paths without a `Rollback` call at each one.

`TransitionAppointment` takes two new parameters, `actorID *int64` and `reason string`, and
`CreateWalkIn` takes `actorID *int64`. Both are `nil`/`""` from the handlers today; Step 3
fills them from the session context. Added now rather than later so the signature changes once.

**`SELECT ... FOR UPDATE` in `TransitionAppointment`, deliberately not in `CreateWalkIn`.**
The transition is read-then-*update* on one row, so locking it is ordinary transaction work.
The walk-in is read-then-*insert* (`MAX(token_no)+1`), and locking there would silently fix
the Loop 2 race. Wrapping it in a transaction does **not** fix that race — two `READ COMMITTED`
transactions still read the same `MAX` — which is exactly why it was safe to wrap.

**`config.Load()` does not read `MIGRATE_DATABASE_URL`** — a deviation from the plan above.
Only the Makefile's `migrate-*` targets use the owner URL; the server binary never touches it.
Putting it in `Config` would add a field nothing reads. Same rule that kept `checked_in_at` and
`appointment_events.session_id` out.

**The restricted role is created by hand, its grants by migration.** `CREATE ROLE ... PASSWORD`
never enters a migration file, because migrations are committed and git never forgets a
password once it has seen one. Role creation is provisioning (once per environment, from
`$APP_USER` / `$APP_DB_PASSWORD`); grants are schema (they change whenever a table is added),
so they live in `20260903111144_grant_app_role.sql`.

The two passwords must differ. If `opd_app` and `opd` share one, anyone who reads
`DATABASE_URL` out of the app's environment gets owner access by editing the username.

**Tests connect as `opd_app`, and that required a second pool.** `TRUNCATE` is its own
privilege — not implied by `DELETE` — and the app role does not have it, correctly, because
the application never truncates anything. So `TestMain` builds two pools: `testPool`
(`opd_app`, what the handler and store use) and `testAdminPool` (owner, used only by
`resetDB`). Test fixtures are admin work, exactly like migrations. `TestMain` also has to
`CREATE ROLE opd_app` *before* `goose.Up`, or the grants migration fails on a missing role.

Note the trap this avoids: the container's default user is a **superuser**, and superusers
bypass grants entirely. A test suite connected as that user cannot prove a denial.

**Done when — met:**
- `TestFullFlow` now reads `appointment_events` directly and asserts exactly two rows:
  `NULL → waiting` from the walk-in, then `waiting → in_consultation` from the transition.
  Verified it fails when the event insert is commented out.
- `TestAppointmentEventsAreAppendOnly` asserts `UPDATE` and `DELETE` both come back with
  `permission denied`. Postgres checks privileges at plan time, so this works on an empty
  table — no fixture needed.

Together the two tests pin the permission exactly: the first proves `INSERT` works, the second
proves `UPDATE`/`DELETE` do not. Either one alone would leave half of it unproven.


---

## Step 3 — Staff auth

### Passwords

`golang.org/x/crypto/bcrypt`, **cost 12** (the library default of 10 is low for 2026 hardware).
Never log a password, never put one in an error message, never return the hash in JSON.

No signup endpoint. Staff are created by a seeder or an admin CLI —
`make seed-admin EMAIL=... PASSWORD=...`. Self-serve signup is explicitly out of scope forever.

### Login flow

```
POST /api/login  { email, password }        -- CSRF-exempt: this is the request that
                                            -- issues the token, so it cannot carry one
  1. look up staff_users by email
  2. bcrypt.CompareHashAndPassword
  3. on failure: same generic message and roughly the same latency for
     "no such user" and "wrong password" — otherwise the response time tells an
     attacker which emails exist
  4. on success:
       token   = 32 random bytes from crypto/rand, base64url encoded
       INSERT auth_sessions (sha256(token), user_id, clinic_id, csrf_token, expires_at)
       Set-Cookie: session=<token>
```

### Cookie flags — every one of these matters

```
HttpOnly            JavaScript cannot read it, so an XSS bug cannot steal the session
Secure              HTTPS only (must be togglable off for local http development)
SameSite=Lax        the browser will not send it on cross-site POSTs — most of CSRF, free
Path=/
Max-Age             matches expires_at
```

`Secure` must come from config, not be hardcoded — with it always on, local `http://localhost`
login silently fails and you will lose an hour.

### Middleware

`RequireAuth` wraps every staff route: read cookie → sha256 → look up `auth_sessions` →
check `expires_at` → load user → put user and `clinic_id` into the request `context`.

**Every store call in a staff route takes `clinic_id` from that context, never from the
request body.** A `clinic_id` in a form field is an invitation to read another clinic's data.
This is the wall; RLS in Loop 5 is the second wall, not the first.

Logout deletes the row (not just the cookie) — the token must die server-side.
Add a periodic cleanup for expired rows, or delete lazily on lookup.

**A SPA gets 401, not a redirect.** `RequireAuth` returns **`401` with the standard JSON
error shape** — never a `302` to `/login`. A redirect inside a `fetch` produces an HTML login
page where the caller expected JSON, and the failure is baffling to debug. The React router
sees the 401 and navigates; that decision belongs to the client.

**Done when:** login sets a cookie, a protected endpoint returns 401 without it, logout kills
the row server-side (not just the cookie), and a tampered cookie value is rejected.

---

## Step 4 — CSRF

`SameSite=Lax` already blocks most of this. Add tokens anyway — defence in depth, and it is
what an interviewer will ask about.

A `csrf_token` is generated per session at login and stored on the `auth_sessions` row.
Every mutating handler compares the incoming value against it with
**`crypto/subtle.ConstantTimeCompare`** — not `==`, because string comparison returns early
on the first differing byte and leaks the answer through timing.

How it reaches the browser: `GET /api/me` returns the CSRF token, the app keeps it in memory, and the
single `fetch` wrapper in `api.ts` attaches it as an `X-CSRF-Token` header on every mutating
request. **One place, not per call site** — a token attached by hand at each call site will be
forgotten at one of them, and that one is the hole.

Note that a JSON API is already awkward to attack this way: a plain HTML form cannot send
`Content-Type: application/json`, so requiring that content type on every mutation is a second
cheap layer. Do not treat it as a substitute for the token.

**Done when:** a POST without the header is rejected with 403, and every real screen still works.

### As built (Steps 3 and 4)

Built together on purpose: doing CSRF after the tests had already been converted for cookies
would have meant a second pass over every request in every test file.

**Passwords.** `internal/auth/password.go` — `HashPassword` / `CheckPassword`, bcrypt cost 12.
No pepper: it only helps against a database-only leak, cannot be rotated without every user's
plaintext, and permanently locks out every account if it is ever lost. OWASP treats it as
optional and wants it in a KMS, not an `.env`. Not worth the key-management story for one clinic.

`auth.DummyHash` is a real cost-12 hash compared against when **no user was found**, so that a
failed login costs the same ~250ms either way. Without it the two paths are ~1ms and ~250ms, and
an attacker enumerates valid emails by timing alone. `TestLoginFailuresAreIndistinguishable`
asserts the two responses are byte-identical.

**Tokens.** `internal/auth/token.go` — 32 bytes from `crypto/rand` (never `math/rand`),
`base64.RawURLEncoding` so the value is cookie-safe with no `=` padding. 64 bits would already be
enough; 32 bytes costs 32 extra bytes on the wire and nothing in the database, where everything
is a 32-byte SHA-256 either way.

**Expiry is checked in SQL** (`AND expires_at > now()`), not in Go. One clock — the database's —
instead of however many app servers there are, each with its own drift.

**Deviation, deliberate: `==` instead of `subtle.ConstantTimeCompare` for the CSRF check.**
Veerbal's call after the trade-off was laid out. The theoretical attack is byte-at-a-time timing
recovery of the token; over the internet, jitter is milliseconds and the signal is nanoseconds,
so it is close to unexploitable in practice. Recorded here because a reader would otherwise
assume it was an oversight. Swapping it back is one line.

**`clinic_id` now comes from the session on every path — and that meant deleting endpoints.**
Once login existed, three routes were taking the tenant from the caller:

```
POST /clinics                      a logged-in receptionist could create clinics
POST /clinics/{id}/doctors         {id} was whatever the caller typed
POST /sessions {clinic_id: ...}    straight from the request body
```

`POST /clinics` was deleted outright — the seeder creates the clinic, Loop 1 has exactly one, and
the Loop 5 version of this endpoint will need a platform-admin actor, a first admin user created
alongside it, and a timezone. None of which this one did. It was not a head start on that; it was
an open hole. `POST /clinics/{id}/doctors` became `POST /doctors`, and `CreateSessionRequest`
lost its `clinic_id` field.

Path parameters had the same hole one level down: `POST /sessions/99/walkins` where session 99
belongs to another clinic. The fix is in the **store**, not the handler — `CreateWalkIn`,
`TransitionAppointment` and `QueueForSession` each take a `clinicID` and put it in the `WHERE`:

```sql
SELECT ends_at FROM sessions WHERE id = $1 AND clinic_id = $2
```

A handler-level `if session.ClinicID != sess.ClinicID` would work too, and would be forgotten on
the next endpoint someone adds. A `WHERE` clause cannot be forgotten, because without it the
query does not exist. Same reasoning as the append-only grant in Step 2: make the wrong thing
impossible rather than checking for it.

**Cross-tenant misses return 404, never 403.** A `403` would confirm the row exists, letting a
caller map out which session ids are real. Zero rows is zero rows — "does not exist" and "not
yours" are the same answer.

**`COOKIE_SECURE` comes from config.** `Secure: true` on `http://localhost` makes the browser
silently refuse the cookie, with no error anywhere — login just never works. Note the app is
plain HTTP in production too (Caddy terminates TLS), so it cannot detect this itself; local
HTTPS via `mkcert` would match production *less*, not more.

**Routes are not under `/api` yet.** Step 5 needs that prefix for the SPA fallback and will
rename them in one pass, along with the test paths.

### Test changes this forced

- `TestMain` builds `testPool` (as `opd_app`) and `testAdminPool` (owner); `loginTestUser` seeds
  a clinic and an admin through the admin pool, logs in over HTTP, and returns
  `{cookie, csrf}`. `authedRequest` attaches both, so converting a request was a one-line swap
  rather than three added lines.
- **A latent bug surfaced:** both test files hard-coded `starts_at`/`ends_at` on a fixed date.
  They passed the day they were written and began failing the next, because `CreateWalkIn`
  rejects an ended session. Now derived from `time.Now()`.
- **`TestConcurrentWalkIns` was green and proving nothing.** After `RequireAuth` landed, all 20
  requests died at auth, and its only assertion — `success + failed == 20` — still held. It now
  has a `default:` branch that fails on any status other than 201 or 500, plus `success >= 1`.
  With that, it reports 4 succeeded / 16 failed again — the ADR 0001 numbers, from requests that
  actually reach `CreateWalkIn`.
- `auth_test.go` adds six denial tests: missing cookie, tampered cookie, missing CSRF header,
  wrong CSRF token, logout deletes the row (and the captured cookie stops working), and the two
  login failure modes being byte-identical.

**Done when — met:** login sets an `HttpOnly; SameSite=Lax` cookie; a protected route is 401
without it; a tampered cookie is 401; logout returns 204 and the `auth_sessions` row is gone
(verified live: 4 → 5 → 4); a POST without `X-CSRF-Token` is 403 while every real screen path
still works.

### Still open in Step 3's neighbourhood

- No cleanup of expired `auth_sessions` rows. The index on `expires_at` exists for it; the job
  does not. Lookups are already safe (`expires_at > now()` in the query), so this is disk, not
  correctness.
- `reason` is threaded through `TransitionAppointment` but nothing sends one yet — the console
  (Step 6) is what will.


---

## The API surface Loop 1 needs

The React app talks only to JSON. Loop 0's endpoints stay; these are added.

```
POST   /api/login              { email, password }        → sets cookie
POST   /api/logout                                        → deletes the auth_sessions row
GET    /api/me                                            → current user + clinic, or 401

GET    /api/doctors                                       → doctors of the logged-in clinic
GET    /api/sessions?date=YYYY-MM-DD                      → that day's sessions
POST   /api/sessions/{id}/delay      { delay_min, version }
POST   /api/sessions/{id}/close      { version }

GET    /api/sessions/{id}/queue                           → from Loop 0
POST   /api/sessions/{id}/walkins                         → from Loop 0
POST   /api/appointments/{id}/transition                  → from Loop 0

GET    /api/board/{session_id}                            → PUBLIC, no auth
GET    /api/q/{public_id}                                 → PUBLIC, no auth
GET    /api/q/{public_id}/qr.png                          → PUBLIC, QR image
```

Everything under `/api/` except `/api/board`, `/api/q` and `/api/login` goes through
`RequireAuth`. **`clinic_id` always comes from the session context, never from the request
body** — a `clinic_id` in JSON is an invitation to read another clinic's data.

`GET /api/me` exists because a SPA has no way to know whether its cookie is still valid
except by asking. Call it once on load and gate the router on the answer.

### As built — the API surface is complete

Every endpoint above exists and is covered by tests. Differences from the plan:

**`POST /clinics` was deleted rather than moved under `/api`.** With login in place it was an
open hole: any signed-in receptionist could create clinics. The seeder makes the one clinic Loop
1 has, and the Loop 5 version of this endpoint will need a platform-admin actor and a first admin
user created alongside it — nothing this one did. `POST /clinics/{id}/doctors` became
`POST /api/doctors`, with the clinic taken from the session.

**`GET /api/sessions/{id}/queue` returns `waiting` **and** `in_consultation`.** The plan implied
waiting only, and building the console proved that wrong within a minute: pressing "Call" made
the patient vanish from the screen, leaving no row to press "Done" on. Ordering puts the patient
with the doctor first:

```sql
ORDER BY CASE WHEN state = 'in_consultation' THEN 0 ELSE 1 END,
         priority DESC, queued_at ASC
```

This costs the index's free sort — `idx_appointments_queue` no longer satisfies the `ORDER BY`,
so a `Sort` node returns to the plan. At 20–50 rows that is microseconds, and the index still
earns its keep on the `WHERE`. Worth knowing rather than discovering later.

**`delay` and `close` share one guarded update.** Both go through `store.updateSession`, which
puts the caller's `version` in the `WHERE` and bumps it on success. Zero rows updated is
`ErrVersionConflict` → **409**, and it deliberately does not distinguish "somebody else changed
it" from "that session is not yours" — both mean *your picture is stale, look again*.

`TestSetDelayRejectsStaleVersion` walks the two-receptionist case: both read version 1, the first
write succeeds and advances to 2, the second is refused, and the test then re-reads to prove the
first write is still the one that stuck. A 409 that quietly lost the earlier value would pass a
status-code-only assertion.

**The three public endpoints take no clinic scoping at all**, and that is the point:
- `GET /api/board/{id}` — a waiting-room screen has nobody to log in. Returns `now_serving`, the
  next five token numbers, the doctor's name and the delay. **Never a patient name**; the room is
  public.
- `GET /api/q/{public_id}` — the unguessable id *is* the credential. Every state gets an honest
  answer (`waiting` / with the doctor / visit complete / marked absent), not just the happy path,
  and an unknown id is a bare `404 not found` with nothing else said.
- `GET /api/q/{public_id}/qr.png` — server-rendered PNG of the patient's own link
  (`skip2/go-qrcode`), so the console can show it on the tablet. It refuses to print a QR for a
  link that leads nowhere.

**ETA is computed in one function, on the server** (`handler.eta`):

```
from = max(session_start + delay_min, now)
eta  = from + (patients ahead) × avg_consult_sec
```

The `max` matters: a sitting that began at 11:30 must not hand out ETAs in the past at 17:00.
"Ahead" is the queue's own ordering written as a comparison — `priority > mine OR (priority =
mine AND queued_at < mine)` — so the number can never disagree with the list the receptionist
sees. Nothing is stored, and nothing is computed in the browser: two phones must not be able to
show different answers, and a phone's clock cannot be trusted.

`appointments.public_id` now comes back from `POST /api/sessions/{id}/walkins`, which is how the
console will build the QR and the link.

---

## Step 5 — React app, embedded and served by Go

### Layout

```
web/
  package.json
  vite.config.ts
  index.html
  src/
    main.tsx
    api.ts            one fetch wrapper — credentials, CSRF, error shape, in ONE place
    pages/
      Login.tsx
      Console.tsx     receptionist
      Board.tsx       public
      Patient.tsx     public
  dist/               build output, gitignored
internal/web/embed.go  //go:embed dist
```

### Serving it from Go

`//go:embed all:dist` on an `embed.FS`, served with `http.FileServerFS`.

**SPA fallback:** any path that is not `/api/...` and does not match a real file must return
`index.html`, so a browser refresh on `/board/8` does not 404. This is the one bit of routing
logic Go has to own.

**Two gotchas that will cost an hour each:**

1. **`go build` fails if `web/dist/` does not exist.** `//go:embed` is resolved at compile
   time. `dist/` is gitignored, so a fresh clone cannot build until `npm run build` has run.
   Fix it in the Makefile — `build` depends on `ui` — and commit a `dist/.gitkeep` so the
   directory exists.
2. **`//go:embed dist` silently skips files starting with `_` or `.`** Vite emits
   `dist/assets/...` which is fine, but use **`all:dist`** so nothing is quietly dropped.

### Dev loop

Vite dev server on `:5173`, with a proxy so `/api` goes to the Go server on `:8080`.
From the browser's point of view everything is one origin, so the cookie works in development
exactly as it does in production. **Do not add CORS middleware** — if you find yourself
needing it, the proxy is misconfigured, and CORS would be papering over the real problem.

### Makefile

```
ui       → cd web && npm run build
build    → ui, then go build
dev-ui   → cd web && npm run dev
```

**Done when:** `make build && make run` serves the React app and the API from `:8080`,
and refreshing on a deep link does not 404.

### As built (Step 5)

**Routes moved under `/api/`** first, because the SPA fallback rule cannot be written without
that prefix: *anything starting with `/api/` is the API, everything else is `index.html`.*
`/healthz` deliberately stayed at the root — uptime monitors and load balancers expect it there,
and Step 9 puts Caddy in front.

**Vite output goes to `internal/web/dist`, not `web/dist`.** `//go:embed` can only reach inside
its own directory, so the choice was either a Go file living in the React project or the build
output landing in the Go tree. The second keeps each language in its own place:

```ts
build: { outDir: '../internal/web/dist', emptyOutDir: true }
```

`emptyOutDir` is required once the output leaves the Vite project root — Vite refuses to clear a
directory outside it without being told.

**`//go:embed all:dist`** — the `all:` prefix matters. Without it `//go:embed` silently skips
files whose names begin with `_` or `.`, which a bundler can emit at any time. Silently, with a
working build and a missing asset.

**`dist/.gitkeep` and `build: ui`.** `//go:embed` resolves at compile time, so a fresh clone
cannot build until the directory exists — but the build output itself must not be committed. Two
halves: `.gitignore` gets `internal/web/dist/*` plus `!internal/web/dist/.gitkeep` (`dist` alone
would not work — git never looks inside an ignored directory, so the negation would never be
reached), and the Makefile's `build` target depends on `ui`, so `go build` is never reached with
an empty `dist`.

**The SPA fallback** (`internal/handler/spa.go`) is a `fs.Stat` and nothing more: if the path
names a real embedded file, serve it; otherwise rewrite the path to `/` and serve `index.html`
with a **200**, not a 404 — a 404 would stop the browser executing the bundle. Go knows nothing
about `/board` or `/console`; adding React pages needs no Go change.

**A catch-all `/api/` route returning JSON 404 — this one is not optional.** Without it,
`mux.Handle("/", spaHandler())` swallows mistyped API paths and answers a `fetch` with HTML and
a 200. The symptom is `Unexpected token '<', "<!doctype "... is not valid JSON` in the browser,
which says nothing about the actual cause. Go 1.22's most-specific-pattern matching does the
rest: `/api/me` beats `/api/`, which beats `/`.

**Dev proxy, not CORS.** `server.proxy` in `vite.config.ts` forwards `/api` from `:5173` to
`:8080`, so the browser sees one origin in development exactly as in production. The frontend
therefore writes `fetch('/api/me')` — relative, no host, no `VITE_API_URL`, and no
`credentials: 'include'` (same-origin requests carry cookies by default). The same line works in
both environments. If CORS middleware ever looks necessary, the proxy is misconfigured.

**Verified:** `/`, `/board/8` and `/kuch-bhi` return HTML 200; a hashed asset returns
`text/javascript`; `/healthz` 200; `/api/me` JSON 401; `/api/nope` and `/api/sesions/8/queue`
(deliberate typo) JSON 404. `/board/8` renders in a browser — served by the Go binary, no Node
process running.

The screens themselves are Steps 6–8; Step 5 is only the serving.


---

## Step 6 — Receptionist console

The main screen. Everything the roadmap's storyline needs, on one page:

- Pick / open today's session for a doctor
- **Add walk-in** → name + contact → shows the token and a QR of that patient's `/q/` link
- The live queue, `priority DESC, queued_at ASC`, showing who is in consultation
- Per row: **Call**, **Done**, **No-show**, **Re-check-in**
- **Insert emergency** (a walk-in with `priority = 1`)
- **Set delay** (minutes)
- **Close session**

### Three rules — these are the whole point of this screen

**1. Commands are absolute, never relative.**
The button sends `transition appointment 42 → in_consultation`, never "call next". Two
receptionists both click; the second is a harmless no-op instead of skipping a patient.
Do not add a "Next" button, however tempting it looks in React.

**2. Everything else guards on `version`.**
Set delay and close session send the `version` they read. `UPDATE ... WHERE version = $1`.
Zero rows → the API returns **409**, and the UI shows **"queue moved, reload"**.
Never retry silently — the whole point is that the user learns the world changed.

**3. Refetch after every mutation.**
Do not patch local React state to guess what the server did. `POST`, then refetch the queue.
At this scale it costs nothing, and it means the screen can never disagree with the database.
Optimistic UI is exactly the wrong instinct on a screen whose job is to be correct.

**Done when:** the full storyline runs through the UI — walk-in, call, no-show, re-check-in
landing at the *back* of the line, emergency at the *front*, delay set, session closed.

### Progress so far

The auth shell is built and verified in a browser; the console's own content is not.

```
src/api.ts        one fetch wrapper — CSRF header, ApiError{status}, 204 handling
src/auth.tsx      AuthProvider (calls me() once on mount), useAuth, RequireAuth
src/pages/Login.tsx     form, redirects to /console when already signed in
src/pages/Console.tsx   signed-in user + Log out, nothing else yet
```

**The CSRF token lives in a module variable in `api.ts`, deliberately not `localStorage`** —
so a page refresh loses it, which is exactly why `GET /api/me` returns it. That endpoint answers
two questions at once: *is this cookie still good?* and *here is your token again*. A SPA cannot
read an `HttpOnly` cookie, so the only way to know whether it is signed in is to ask.

`RequireAuth` therefore needs three states, not two — `loading`, signed-in, signed-out. Without
the `loading` state the console renders before `/me` resolves, the receptionist clicks a button,
and the request goes out with no CSRF token and comes back 403. `Login` has the mirror of this:
`loading` first, then redirect to `/console` if a session already exists, otherwise the form.

**Verified in the browser, not just by tests:**
- `document.cookie` is empty while signed in — `HttpOnly` doing its job. The cookie is visible in
  devtools (Application → Cookies) with `HttpOnly ✓`, `SameSite Lax`, `Secure` blank (local),
  7-day expiry, and a 43-character value — 32 random bytes as base64url.
- `sha256(cookie value)` computed by hand matches `token_hash` in `auth_sessions` exactly. The
  browser holds the token, the database holds only its hash, and the hash cannot be run backwards.
- Deleting the row in `psql` logs the browser out on the next request **while the cookie is still
  in the browser**. That is the whole argument for server-side sessions over JWTs, demonstrated
  rather than asserted.
- Refresh keeps the session (the `/me` round trip) and logout still works afterwards — which
  proves the CSRF token really was re-fetched, since logout is a CSRF-protected POST.

**Blocked on four endpoints** the console needs and Loop 0 never built:

```
GET  /api/doctors                    doctors of the signed-in clinic
GET  /api/sessions?date=YYYY-MM-DD   that day's sittings
POST /api/sessions/{id}/delay        { delay_min, version }
POST /api/sessions/{id}/close        { version }
```

Building these first so the console is written once instead of twice.

**Traps hit:**
- `AuthProvider` was left out of `main.tsx` while `RequireAuth` was imported. TypeScript passed —
  whether a provider is above you in the tree is a runtime fact, not a type. The explicit
  `throw new Error('useAuth must be used inside AuthProvider')` is what made it findable; without
  it `useContext` returns `null` and the failure surfaces later as a property access on null.
- A component named `Console` collides with the browser's built-in `Console` **type**, so a
  missing import reports "only refers to a type, but is being used as a value" instead of
  "cannot find name".
- Cookies are scoped by **host, not port**. Another project's cookies on `localhost:3000` are
  visible to this app on `:5173`. Two local apps using the same cookie name will overwrite each
  other and produce logouts with no apparent cause.
- `StrictMode` runs `useEffect` twice in development, so `/api/me` appears twice in the network
  tab. Not a bug.


---

## Step 7 — Board page

`/board/{session_id}` — **public, no login.** This is a TV in a waiting room; nobody types
a password into it.

Big type. "Now serving 12". The next few tokens. Nothing else — it is read from across a room.

Refetch every 5 seconds. Clear the interval on unmount, or you will leak a timer per mount
and hit it again in Loop 4 when you go hunting for goroutine and listener leaks.

**Show token numbers, never patient names.** A waiting room screen is a public place.

Handle a fetch failure by keeping the last good value on screen rather than blanking it —
a TV showing stale numbers is far better than a TV showing an error.

**Done when:** it opens in a browser tab and updates on its own. That tab *is* the TV;
there is no hardware in this project.

---

## Step 8 — Patient page, ETA, and the QR

`/q/{public_id}` — public, unguessable link, refetch every 10 seconds.

Shows: **"Now serving 12 · You are 19 · about 55 minutes"**

```
position = count of waiting appointments ordered ahead of this one
           (priority DESC, queued_at ASC)
eta      = max(session_start + delay, now) + position × avg_consult_sec
```

**Both computed on the server, on read. Nothing stored, and nothing computed in React** —
the browser's clock cannot be trusted and two phones must never disagree.

Handle every state honestly, not just `waiting`:
`in_consultation` → "you are with the doctor", `done` → "visit complete",
`absent` → "you were marked absent, please see reception".
A page that only handles the happy path is where trust dies.

An unknown `public_id` returns **404 with a generic message** — never "no such appointment,
but here is what we do have".

**QR:** after each walk-in the console shows a QR of the full `/q/{public_id}` URL so the
patient scans it straight off the tablet. Generated server-side as a PNG
(`github.com/skip2/go-qrcode`) — do not pull in a JS QR library for this.

**Done when:** two phones on two different tokens show two different, correct positions and
ETAs, and both ETAs move when the receptionist sets a delay.

---

## Step 9 — Deploy to a real VPS, with a real domain and real TLS

**This is the step that separates this project from a tutorial. Do not skip it.**

### Dockerfile

**Three stages now:** bun (`bun install --frozen-lockfile && bun run build`) → go (embeds
`dist`, static build) → minimal runtime. **The plan above said node/`npm ci`** — the project
moved to bun during Step 5 because npm installs were slow, so the first stage is
`oven/bun`, the lockfile is `bun.lock`, and the Makefile's `ui`/`dev-ui` targets call `bun run`.
The node stage must finish before the go stage copies `web/dist` in, or `//go:embed` fails at compile time.

- Static build so the runtime image needs no libc
- Run as a **non-root user**
- Include CA certificates (needed for TLS calls out) and `tzdata` (needed for `Asia/Kolkata` —
  a `distroless`/`scratch` image without it silently falls back to UTC and every displayed
  time is wrong by 5.5 hours)
- `.dockerignore` so `.env` and `.git` never enter the image

### Compose on the server

```
caddy  →  app  →  postgres
```

Caddy gets automatic TLS from Let's Encrypt with a two-line Caddyfile. Requirements:
a real domain with an A record pointing at the VPS, and ports **80 and 443** open —
80 is required for the ACME challenge, so do not close it.

### Server basics

Any cheap VPS (Hetzner / DigitalOcean / Contabo, ₹500–1500/month). Then:

- A non-root user with sudo; SSH by key only; password auth **off**
- Firewall: 22, 80, 443 only
- Postgres **not** exposed to the internet — no host port mapping, only the compose network
- Real secrets in the server's `.env`, never in git. Generate a new DB password;
  `12345678` does not travel to production
- `restart: unless-stopped` on every service

### Config differences from local

`sslmode=require`, `Secure` cookies on, a real `PORT`, real credentials.
Same binary, different environment. If a code change is needed to deploy, the config is wrong.

**Done when:** `https://yourdomain/...` serves the board page with a valid certificate from a
phone on mobile data — not just from your laptop on the same network.

---

## Step 10 — Backups

A backup you have never restored is not a backup. It is a hope.

- Nightly `pg_dump` (custom format, `-Fc`) on a schedule — host cron or a small sidecar
- Upload to object storage **off the same box**: Backblaze B2, Cloudflare R2, S3.
  A backup sitting on the disk that dies with the server is worth nothing
- Timestamped filenames, and a retention rule (keep 7 daily, 4 weekly)
- Restrict the storage credentials to write-only where the provider allows it
- **Log and check the exit code.** A silently failing backup job is the classic disaster —
  everything looks fine for six months and then it does not

**Done when:** a dump appears in object storage on its own overnight, without you touching it.

---

## Step 11 — Break it

Three drills. Each one produces a runbook entry describing what actually happened, not what
you expected to happen.

**1. Kill mid-request.**
Put load on it, then `docker kill` the app container. Confirm: no half-written data, no
appointment in a state with no matching event. Restart, confirm it recovers on its own.

**2. Graceful shutdown actually drains.**
Send `SIGTERM` (not `kill -9`) during traffic. In-flight requests should finish; new ones
should be refused. This is the Loop 0 code finally being tested — and it is what makes
Loop 7's rolling deploys drop zero requests.

**3. The restore drill — the one that counts.**

Do this **on the deployed VPS, after Step 10's backups have actually run at least once** —
not on the laptop mid-development. There is no real clinic and no real patient, so there is
genuinely nothing to lose. That is the whole reason this project can afford the drill.

- Note the exact row counts of `appointments` and `appointment_events`
- **Drop the database.** Actually drop it. A drill you flinched at teaches nothing
- Restore from last night's dump in object storage
- Compare the counts. Time the whole thing with a stopwatch

**Write down the number.** "My restore takes 4 minutes" is an interview answer.
"I have backups" is not.

**Done when:** the database has been destroyed and brought back, and the runbook was written
from what actually happened — including whatever went wrong the first time.

---

## Step 12 — ADR and runbooks

`docs/adr/0002-...` onward. One page each: context, options considered, decision, consequences.
Loop 1 has at least these decisions worth recording:

- Server-side sessions in Postgres instead of JWTs (and why: instant revocation, no secret
  rotation problem, and the DB round trip is cheap at this scale)
- Storing a hash of the session token instead of the token
- Append-only history enforced by DB grants rather than by application code
- Patient identity as an unguessable link rather than a login

`docs/runbooks/` — one file per failure mode, written from the drills:
"restore from backup", "certificate did not renew", "app container will not start",
"disk full".

Tick the progress table at the top of **this** file as each step lands, the same way
`docs/loop-0-scope.md` was maintained. Record the real numbers — restore time, page load,
image size — on a numbers page; the roadmap grades those as heavily as the code.

---

## Explicitly NOT in Loop 1

| Thing | Loop |
|---|---|
| Online booking, public booking page, capacity enforcement, idempotency keys, isolation-level comparison, **the walk-in token race fix** (`MAX(token_no)+1` → `INSERT`) | 2 |
| Outbox, notification worker, dedupe keys, **telling patients about the delay**, SMS/WhatsApp/SMTP | 3 |
| SSE, Prometheus, Grafana, OpenTelemetry, pprof, k6 | 4 |
| A second clinic, RLS, partitioning, rate limiting, retention/anonymization, audit viewer | 5 |
| Redis, Kafka | 6 |
| Kubernetes, CI/CD, HPA | 7 |

The delay button changes the ETA everyone sees. **It sends nothing to anyone.** That is
correct for Loop 1 — notification needs the outbox, and the outbox is Loop 3.

The race from ADR 0001 stays broken through all of Loop 1. Resist fixing it. Loop 2 only
means something because the failure was seen first and left alone.
