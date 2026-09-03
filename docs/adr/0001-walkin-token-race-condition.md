# ADR 0001 — Walk-in token race condition (observed, not fixed)

## Status

Observed. Deliberately not fixed. The fix belongs to Loop 2.

## Context

`store.CreateWalkIn` assigns a token number with:

```sql
SELECT COALESCE(MAX(token_no), 0) + 1 FROM appointments WHERE session_id = $1
```

...followed by a separate `INSERT`. Two requests can both run the `SELECT` before either
has committed its `INSERT`, so both compute the same next token number. This was known and
intentional — see `docs/loop-0-scope.md`, Step 6: "**Naive on purpose.** It will lose races."

## What we did

`internal/handler/race_test.go` (`TestConcurrentWalkIns`) creates one clinic, one doctor, one
session, then fires **20 concurrent** `POST /sessions/{id}/walkins` requests at that same
session using goroutines + `sync.WaitGroup`, via `httptest` against the real handler/store/
Postgres stack (`testcontainers-go`).

Run with: `go test -race ./internal/handler/... -run TestConcurrentWalkIns -v`

## What happened

**4 succeeded, 16 failed**, every failure with the same error:

```
create walk-in: insert: ERROR: duplicate key value violates unique constraint
"appointments_session_id_token_no_key" (SQLSTATE 23505)
```

The successful requests got tokens `1, 2, 3, 4` — not because only 4 tokens were valid, but
because collisions happened in waves: a batch of requests raced for token `1`, one won, the
rest were rejected outright (no retry); a smaller batch that happened to run their `SELECT`
after token `1` committed raced for token `2`, and so on.

`UNIQUE (session_id, token_no)` did exactly its job — no duplicate-token row was ever written,
so the data stayed correct. But 16 out of 20 legitimate walk-in requests were dropped with an
error instead of getting a token. Correct data, bad availability.

**Re-run, different numbers:** a second run produced 3 succeeded / 17 failed, not 4/16. The
exact split is not fixed — it depends on goroutine scheduling, which varies run to run. That
variance is itself the signature of a race: a deterministic split would suggest something
else (a bug in the test, not a race) was being measured.

## A note on `-race`

`go test -race` reported **no Go-level data race**. This surprised us at first — the whole
point was to see a race. But `-race` (Go's own race detector) only catches unsynchronized
access to *Go memory* shared between goroutines. Here, each goroutine makes its own
independent call through the connection pool; there is no shared Go variable being read and
written without a lock. The race is happening entirely on the **Postgres side** — two
transactions reading the same `MAX(token_no)` before either commits is a database-level
lost-update problem, invisible to a language-level race detector. We had to *observe* it
(run the test, read the actual error), not just trust `-race` to flag it.

## Decision — do not fix yet

The fix is one of:
- Row lock (`SELECT ... FOR UPDATE` on the session row, so token assignment serializes)
- Retry loop (on unique-violation, re-read `MAX` and retry the insert)
- A comparison of Postgres isolation levels for this specific access pattern

All three are Loop 2 scope. Fixing now, before seeing the failure, would have made the fix
feel theoretical. This ADR is the record that the failure was seen first, with real numbers,
so Loop 2's fix has something concrete to point at and verify against (re-run
`TestConcurrentWalkIns` after the fix — expect 20 succeeded, 0 failed).
