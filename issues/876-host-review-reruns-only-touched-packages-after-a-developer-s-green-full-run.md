# 876 — host review reruns only touched packages after a developer's green full run

**Status**: Closed — Lean-sprint and AgenticLoop host guidance limits reruns after green full runs to touched packages; full suite passes.
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Practice
**Related**: 704 (host rerun runs make test, not test-q1), 545 (lean-sprint guardrails), loom 377

---

/goal The lean-sprint and AgenticLoop docs (and the skills that copy them) tell the reviewing host to
rerun only the packages a ticket touched after the developer reported a green full run, and to run a
full suite only with the user's go or at release; or stop and report when blocked on a user decision.

## Finding (loom, 2026-10-23)

The loom host reran `make test-q1` (unbounded `go test ./...`, PTY tests) after every ticket, right after
the developer's own full run. One rerun maxed the user's CPU and filled swap (7.8 of 8 GiB). The
developer's "fixed but not rerun counts as untested" rule (loom CLAUDE.md) pushed the host to rerun
everything, though the developer had already run it.

## Proposal

- Host verification = touched packages (`go test -p 2 <pkgs>` or the repo's equivalent) + the live check.
- A full run is needed only when the developer did not run one, at release, or with the user's go.
- Long chained host commands are opaque in the UI: one step per command, or a shown script.
- Check that this does not contradict 704 and the loom/harnez "host reruns q1" rules; update them.
