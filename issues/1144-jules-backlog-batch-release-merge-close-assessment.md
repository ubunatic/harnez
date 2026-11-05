# 1144 — Jules backlog batch: release (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 230, 246, 314, 321, 365, 568, 950

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 230 Android direct-release scaffold (Makefile template, signing, checksums)
- 246 Add /commit and /publish Skills for Staged Commit Ownership and Multi-Project Release/Publish Workflows
- 314 harnez release: auto-create minisign key if missing and no repo key defined
- 321 harnez release --login flag to authenticate or refresh forge credentials via fj auth login
- 365 Add `harnez release --init=<lang|mode>` to bootstrap release scaffolding
- 568 Split IssueTracking, AgenticLoop and GoRelease into generic docs and harnez rules
- 950 Go release docs: vet generated version.go before tagging, lowest go directive for libraries

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1144.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1144 | 9470734212392980745 | pending | running (started 2026-11-06 00:45, audited 9f7dcf78) |
