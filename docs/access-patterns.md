# Access Patterns

Every query the system actually runs, derived from `internal/store/store.go` — not guessed
in advance. This should have come before the schema (Track A), but Loop 0 built the tables
first and is catching up here now that real queries exist to read from.

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

---

#### #1 Queue for a session, ordered

```
Trigger:  GET /sessions/{id}/queue — receptionist screen, refreshed often (every
          few seconds, or on every state change elsewhere)
Equality: session_id, state
Range:    none
Sort:     priority DESC, queued_at ASC
Returns:  N rows
Budget:   < 50ms — this is on-screen, interactive
```

**No supporting index exists.** This is the one pattern in Loop 0 that needs a new index —
see "Indexes" below.

---

#### #2 Session lookup (for a walk-in)

```
Trigger:  POST /sessions/{id}/walkins — every walk-in request, to read clinic_id
          and ends_at before inserting
Equality: id (primary key)
Range:    none
Sort:     none
Returns:  1 row
Budget:   < 10ms
```

Already fast — `id` is the primary key, which Postgres indexes automatically.

---

#### #3 Next token number for a session

```
Trigger:  POST /sessions/{id}/walkins — every walk-in request, to compute
          MAX(token_no) + 1 (naive, intentionally races — see ADR 0001)
Equality: session_id
Range:    none
Sort:     none (aggregate)
Returns:  aggregate (1 row)
Budget:   < 10ms
```

Already covered — `UNIQUE (session_id, token_no)` creates its own index on exactly
`(session_id, token_no)`, which this query can use to find the max quickly.

---

#### #4 Appointment state lookup (for a transition)

```
Trigger:  POST /appointments/{id}/transition — every transition request, to read
          the current state before calling domain.Transition
Equality: id (primary key)
Range:    none
Sort:     none
Returns:  1 row
Budget:   < 10ms
```

Already fast — primary key.

---

#### #5 Appointment update (writing a transition)

```
Trigger:  POST /appointments/{id}/transition — the UPDATE ... RETURNING that
          actually writes the new state (and resets queued_at if the new state
          is waiting)
Equality: id (primary key)
Range:    none
Sort:     none
Returns:  1 row
Budget:   < 10ms
```

Already fast — primary key.

---

## Not yet built

The original Loop 0 plan (before real endpoints existed) expected two more patterns —
"today's sessions for a clinic" and "doctors for a clinic" — but no endpoint reads either
of those yet (there's no `GET /sessions` or `GET /clinics/{id}/doctors` list route). Not
documenting index needs for queries that don't exist yet; add the pattern here when the
endpoint that needs it actually gets built.

## Indexes

One new index, for #1:

```sql
CREATE INDEX idx_appointments_queue
    ON appointments (session_id, state, priority DESC, queued_at ASC);
```

Column order follows the recipe: equality columns first (`session_id`, `state`), then the
sort columns in their exact `ORDER BY` order and direction (`priority DESC, queued_at ASC`).
With this index, Postgres can satisfy the whole query — filter and sort — from the index
alone, without a separate sort step over the matching rows.
