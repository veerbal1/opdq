# ADR 0002 — Receptionist is the queue operator (no doctor screen in Loop 1)

## Status

Decided. Doctors do not log in during Loop 1. Revisit when `avg_consult_sec` becomes a
rolling average — Loop 4.

## Context

Loop 1 builds three screens: the receptionist console, the public board, and the public
patient page. None of them belongs to a doctor. The question came up while designing
`staff_users`: should `role` include `'doctor'`, and should doctors get a login?

In a real clinic the doctor is often the person who knows a consult just ended. The
receptionist is at the front desk, not in the consult room.

## What deferring actually costs

Not much, and that is the point. Adding a doctor screen later needs:

- `'doctor'` added to the `staff_users.role` CHECK — one line of migration
- a `staff_users.doctor_id` column linking the login to the existing `doctors` row — one line
- one more React route — a thing already known how to build

`appointment_events.actor_id` needs **no change at all**. It points at `staff_users` and
carries no assumption that the actor is a receptionist. The schema does not need space
"left" for this; the space is already there.

So the usual argument — *cheap to add later, expensive to retrofit* — comes out clearly on
the side of deferring.

## Decision

Defer. The reason is **budget, not schema**.

Building the doctor screen costs roughly a day and teaches nothing new: the same React, the
same JSON API, the same auth middleware already being built for the console. That day
belongs to Steps 9–11 — first deploy to a real VPS, first TLS certificate, first backup, and
the restore drill where the database is actually dropped and brought back. One of those is an
interview answer. The other is a fourth CRUD screen.

Loop 1 is not wrong about how clinics work, either. Small clinics really do run this way:

```
patient walks out of the consult room
   → receptionist sees it
   → presses "Done"
   → presses "Call" for the next token
```

The roadmap agrees — `mark done / no-show` sits in the receptionist's job list. The doctor's
only view in the roadmap is a phone page saying "I am running 30 minutes late", and that
arrives in a later loop.

## Consequence — and this is the part worth remembering

`completed_at` will record **the moment the receptionist noticed**, not the moment the consult
actually ended. This is a measurement bias, not a bug, and it is not constant:

| Clinic state | Gap between real end and recorded end |
|---|---|
| Empty | ~10 seconds — irrelevant |
| Busy | minutes — the receptionist is mid walk-in, mid phone call |

The bias grows with load. And `completed_at` is not a decorative column: `started_at →
completed_at` is exactly the measurement Loop 4 will use to compute a rolling
`avg_consult_sec`, which the entire patient ETA is built on.

So the failure mode is: **the busier the clinic, the more inflated the measured consult
duration, and the more wrong every patient's ETA — precisely when the ETA matters most.**

Loop 1 sidesteps this by using a fixed `avg_consult_sec` (default `480`). No rolling average
means no bias to propagate, yet.

## What Loop 4 must not forget

Before turning `completed_at` into a rolling average, decide how to handle observation lag.
Options to weigh then, not now:

- Let the doctor end the consult (the screen deferred here), removing the lag at its source
- Use a trimmed mean or median so outliers from a distracted receptionist do not dominate
- Measure `queued_at → started_at` (a receptionist action on both ends) instead of consult
  duration, and accept it answers a slightly different question

The doctor screen's real justification is this number — not the UI.
