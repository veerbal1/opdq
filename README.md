# OPD Queue

A backend system for managing a clinic's out-patient department (OPD) queue.

## Problem

At a walk-in clinic, patients don't book a fixed time slot — they arrive, take a token,
and wait. The front desk needs to know: who's in line, in what order, and what's
happening with each patient right now (waiting, with the doctor, done, or a no-show).

Paper registers and verbal token numbers break down once a clinic has more than a
handful of patients — tokens get mixed up, priority patients get skipped, and nobody
can tell a patient how long the wait actually is.

## What this is

A Go + PostgreSQL service that models a clinic's queue as data:

- Doctors run **sessions** (a sitting, with a start time and a delay if the doctor
  runs late).
- Patients get an **appointment** in a session — either booked ahead or a walk-in
  token issued at the desk.
- Each appointment moves through a small set of states: waiting, in consultation,
  done — or absent, if the patient stepped out and needs to check back in.

The system is being built up piece by piece, learning the reasoning behind each
decision along the way — not copied from a template.
