# Ticket Hygiene: Decision Matrix (issue 842)

**Date**: 2026-10-21
**Ticket**: [842](../../issues/842-research-ticket-hygiene-at-900-tickets-archive-backlog-tiers-short-open-lists-duplicate-detection.md)
**Inputs**: three independent reports, 9 proposals in total:
[sol](2026-10-21-ticket-hygiene-sol.md) (sol:med), [opus](2026-10-21-ticket-hygiene-opus.md)
(opus:med), [argon](2026-10-21-ticket-hygiene-argon.md) (argon:low).

## Baseline (agreed by all three)

- ~249 unresolved tickets (238 Open, 7 Blocked, 4 In Progress) and 17–18 Drafts; 562 Closed, of
  which 543 still sit in `issues/` (the archive convention in IssueTracking.md is barely used).
- Priorities are flat: ~156 of the open tickets are P2. Roadmap "Now" holds only ~12 items.
- **Context cost today**: `issues/README.md` is ~246 KB (~60k tokens, every ticket ever);
  `harnez find issues -a status:open` is ~41 KB (~10k tokens). The managed rules
  (`.harnez/rules/Tools.md`, `Issues.md`) teach `-a status:open`, so the cost is self-inflicted.
- Duplicate search already scans all tickets including `archive/`, but it is AND-only (long
  title queries miss), nothing forces it before `issues new`, and there is no promote/reopen flow.
  Example duplicate pair found: 697 / 698.

## The 9 proposals collapse into 4 distinct solutions

| # | Solution | Proposed by | Mechanism |
|---|---|---|---|
| S1 | **Backlog status** | sol 1, opus A, argon A | New lifecycle value `Backlog — reason`; files stay in `issues/`; default open lists exclude it; verbs `backlog` / `promote`. |
| S2 | **Tier metadata field** | sol 2 (`Queue`), argon C (`Focus`) | Extra header `**Queue**: Active\|Backlog\|Archive`; status stays the lifecycle (Blocked stays Blocked when parked). |
| S3 | **Directory tiers** | sol 3, opus B, argon B | `issues/` = active only, `issues/backlog/`, `issues/archive/`; status verbs `git mv` the file. |
| S4 | **Ranked NOW list** | opus C | `issues/NOW.md` lists ≤20 ranked ticket numbers; every other Open ticket is backlog implicitly. |

## Decision matrix

| | S1 Backlog status | S2 Tier field | S3 Directories | S4 NOW list |
|---|---|---|---|---|
| Context cost of default list | low | low | lowest; also for `ls`/Grep without harnez | lowest (~20 rows) |
| Reaches ~20 active | needs grooming | needs grooming | needs grooming | by construction |
| Duplicate reach | all tickets | all tickets | all dirs (scanner change) | all tickets |
| Migration effort | low (batch verbs exist) | low–medium (new header) | high (moves + link rewrite) | lowest (pick 20) |
| Tooling change | small | medium | large | medium |
| Links / git history | stable | stable | paths change on every park/promote | stable |
| Old harnez binaries / other projects | see `Backlog` as unknown status | ignore field → print everything | skip `backlog/` → may reuse numbers | ignore NOW → old behaviour |
| Main risk | backlog regrows; Blocked info lost when parked | two fields can contradict (Active + Closed) | broken links, merge conflicts | second source of truth goes stale |

## Shared building blocks (all three researchers, independent of the choice)

1. **Duplicate gate in `issues new`**: before reserving a number, run a new OR-ranked
   similarity search (`find issues similar`) over every ticket in every tier; show the top ~5
   with tier/status. On a real match, reopen or promote the old ticket under its old number and
   append the new context; `--force` (or `--not-dup-of N`) files anyway. Never auto-reopen on
   fuzzy similarity alone.
2. **README split**: `issues/README.md` lists only active tickets and links generated backlog
   and closed indexes. This alone removes the biggest context cost (~60k → ~2–10k tokens).
3. **Fix the rules**: replace `-a status:open` in the managed rules and skills with a bounded
   active listing.
4. **Active-count cap**: lint/status warning above a budget configured in `spec/` (e.g. 30 or 100).
5. **Grooming pass**: normalize status-line variants first, close shipped/superseded tickets
   (Roadmap §9 lists ~10), then park by rule (P3, stale P2, Drafts > 14 days). Rules alone give
   ~70–100 active; reaching ~20 needs one human ranking pass.

## Pros and cons in one line each

- **S1**: + smallest change, stable links, trivially reversible. − Blocked/Backlog share one
  field; `ls issues/` still shows everything.
- **S2**: + keeps lifecycle and scheduling separate, best reporting. − extra header, invalid
  combinations need lint, old binaries become noisy.
- **S3**: + only active files in `issues/`, clear for any tool. − biggest migration, link churn on
  every status flip, conflict-prone.
- **S4**: + explicit ranking (fixes "everything is P2"), zero migration. − another file to keep
  in sync; "Open" no longer means "active".

## Recommendation

**S1 (Backlog status) + building blocks 1–5**, with S4's ranked NOW list as an optional second
step if ordering among the ~20 matters. Two of three researchers recommend exactly this core
(opus: S1 + S4; argon leans S1; sol prefers S2 and names S1 as the smaller alternative). Pick S2
over S1 only if keeping `Blocked` visible while parked matters to you. S3 is not recommended:
all three rate it highest-risk, and the README split already delivers most of its context gain.

Cheapest first step regardless of choice: building blocks 2 and 3 (README split, rule fix).

## Decisions needed from the user

1. Solution: S1, S2, S3, or S1 + S4.
2. Duplicate gate: hard stop (exit non-zero unless `--force`) or warning only.
3. Active target: ~20 strict, or ≤100 with a warning.
4. Should `close` move tickets to `issues/archive/` automatically (543 files to sweep once)?

## Uncertainties

- Which listing commands agents actually run was not measured; telemetry could confirm the costs.
- Git creation dates look compressed, so age-based grooming rules need a check first.
- Recall of a token-overlap similarity search is unknown; replay known duplicate pairs first.
