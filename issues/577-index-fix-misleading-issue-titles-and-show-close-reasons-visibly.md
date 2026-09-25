# 577 — Index: fix misleading issue titles and show close reasons visibly

**Status**: Open
**Priority**: P2
**Severity**: Enhancement
**Category**: Issue tracking / `harnez index`

## Problem

Agents scan `issues/README.md` to learn project state without reading ticket bodies. Two things in
the index mislead them:

1. **Titles describe the original plan, not the outcome.** Example: 115 "Canary: evaluate DuckDB Go
   embedding for tool-call telemetry storage" reads as "harnez uses DuckDB". In fact DuckDB was
   dropped and `modernc.org/sqlite` is used (`internal/telemetry`). An agent in `../search` took
   DuckDB as a live storage option from the title alone (2026-09-25).
2. **Close reasons are missing or hidden.** Many rows say only "Closed — resolved in <sha>", which
   does not say *what* was decided. Some reasons are long and get cut off in narrow views.

## Proposal

- On close (`harnez issues <verb> ... [reason]`), require or prompt for a short outcome phrase,
  e.g. "superseded: SQLite chosen over DuckDB", and put it first in the Status column.
- Allow an outcome suffix or retitle on close for canaries/research tickets
  (e.g. "... — rejected, see SQLite") while keeping the file name stable.
- `harnez status` lint: flag closed tickets whose status has no reason besides a commit sha.
- Backfill: fix the worst misleading rows, starting with 115/116.
