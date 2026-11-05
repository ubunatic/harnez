# 1146 — Jules backlog batch: git, PRs and forges (2 of 2) (merge/close assessment)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 1042, 1054, 1055, 1058, 1060, 1077, 1080

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 1042 Use merge=union in .gitattributes for generated issue indexes to avoid forge merge conflicts
- 1054 Configure repository roles and push rules per repo
- 1055 Automate issue indexes after merges with a Forgejo workflow
- 1058 Squash sprint commits by milestone without rewriting main
- 1060 Add a branch cleanup skill with optional autonomous decisions
- 1077 init: Go summary drops Cobra; Quota-1 merge duplicates Enforced Test Target
- 1080 gokay check skips untracked files, so a repo cannot be checked before its first commit

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1146.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1146 | 17157263050103068159 | github #101 (closed) | done 10m: 7 keep, 0 close. 3/3 refs hold, but 4 tickets `not verified` without searches; correction on 1042 wrong (2a7c9bee exists), on 1058 right (7248728f missing). No ticket changes |
