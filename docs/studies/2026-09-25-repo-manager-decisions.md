# Repo Manager Decisions (2026-09-25)

Decision log of the harnez repo-manager session. Flow per ticket: quick assessment, read-only
research by `codex:luna:med` advisor, findings written into the ticket, `codex:luna:med` developer,
host diff review and commit (lean-sprint rules).

## Triage of 576–579

| Ticket | Assessment | Decision |
|---|---|---|
| 576 empty telemetry DB files | P3, small, likely no code source | Research, then fix or close with finding |
| 577 index titles / close reasons | P2 feature, design choice needed | Research; scope M1 to lint + backfill 115/116 before CLI changes |
| 578 harnez agent discoverability | P2, vague goal, overlaps 575 | Research gaps first; no dev until a concrete list exists |
| 579 lite quota-1 init guidance | P2, clear doc/template bugs | Research, then fix sources |

Order of developer work (one writer at a time): 579, 576, 577, 578.

## Pitfall seen by the host

`harnez agent wait --name res-576` fails with "accepts 1 arg(s), received 0": `wait` takes the
session as a positional argument, although `--name` is shown as a global flag. The host's
background wait loop hid the error and "finished" at once. Correct form:
`harnez agent wait res-576 --timeout 30m`. Input for ticket 578 (discoverability).
