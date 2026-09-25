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

## Research findings (2026-09-25)

- Verified on HEAD `6622488`; working tree was clean. The ticket premise is confirmed: `issues/README.md` takes each title from the ticket H1 and copies its raw Status field verbatim. See [internal/index/index.go](/home/uwe/projects/harnez/internal/index/index.go:67).
- Ticket 115 is a concrete stale title: its H1 still says “evaluate DuckDB,” while its body records “NO-GO on DuckDB” and selecting SQLite. Ticket 116 still says “resolved in 6cd33b0”; see [115](/home/uwe/projects/harnez/issues/115-canary-duckdb-go-embedding-for-tool-telemetry.md:1) and [116](/home/uwe/projects/harnez/issues/116-tool-telemetry-schema-and-storage-layer.md:1).
- Close reasons are not hidden by the generator: it outputs the Status field as written. But a bare SHA-only reason is permitted and gives little outcome context. `harnez issues close` also permits no reason, explicitly avoiding fabricated text; see [cmd/harnez/issues.go](/home/uwe/projects/harnez/cmd/harnez/issues.go:89) and [composeNewStatus](/home/uwe/projects/harnez/cmd/harnez/issues.go:417).
- `harnez issues lint` currently checks structural/index consistency, not whether a closed status communicates an outcome; see [internal/issues/issues.go](/home/uwe/projects/harnez/internal/issues/issues.go:444). Existing counts show 98 closed statuses with a “resolved in” SHA prefix, though some may include additional useful wording.
- Minimal fix: keep filenames and table layout stable; retitle misleading tickets by editing their H1 and update the corresponding Status to start with a short outcome phrase, e.g. `Closed — superseded: SQLite chosen over DuckDB (857ff99)`. Add a lint diagnostic for closed statuses that contain only a commit reference (or are bare `Closed`), with a fixture covering both a SHA-only status and a descriptive status.
- Acceptance tests: `IssuesTable` emits the updated H1 title and full Status text; lint flags bare/SHA-only closed statuses but accepts descriptive outcomes that include a SHA; `harnez index --check` reports no drift after retitling and regenerating.

## Host decision (2026-09-25)

M1 scope: (a) lint warning for closed statuses that are bare or only a commit sha; descriptive text plus sha passes. (b) Backfill 115 and 116: retitle H1 to the outcome, status starts with an outcome phrase; file names stay. Out of scope: forcing a reason in `harnez issues close` (closing without reason stays allowed so agents never invent text; the lint catches it).
