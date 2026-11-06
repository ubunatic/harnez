# 1145 — Jules backlog batch: git, PRs and forges (1 of 2) (merge/close assessment)

**Status**: Closed — Batch applied in the 2026-11-06 Jules backlog cleanup; per-ticket results in this ticket's Jules runs table and docs/studies/2026-11-06-jules-backlog-cleanup.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 298, 378, 402, 836, 997, 1018, 1040

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete, taia candidate, deinvest) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 298 Third recurrence of the commit-checkpoint gap: file granularity, git checkout -- destructiveness
- 378 Fleet-wide multi-repo git history sparks and token attribution matrix for uman
- 402 Move config diff below status and free top-level harnez diff for visual git diff
- 836 Stop asking to push other repos mid-session; check unpushed repos in wrap
- 997 Code box in harnez usage for configured repos PR counts and latest PR age
- 1018 reconcile follow-ups: canary on a real merge, share renumbering with mv, test --check with --pull
- 1040 harnez config: get/set/unset repo config keys like git config, with tab completion

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1145.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.
3. Collect `taia candidate` and `deinvest` verdicts as leads for the owner; do not move or close on them alone.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1145 | 4281001866076029153 | github #108 (closed) | done (>25m, one resumed wait): 5/5 refs hold, all 4 closes rejected. 298 asks for a hook, not the doc rule. 836 asks wrap to list unpushed repos. 1018 is the follow-ups. 1040: get/set/unset/list done, completion missing (live `harnez __complete config git.` returns nothing); noted in 1040. 7 kept |
