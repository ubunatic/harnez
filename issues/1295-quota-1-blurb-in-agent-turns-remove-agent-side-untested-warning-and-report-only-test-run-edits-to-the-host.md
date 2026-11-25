# 1295 — quota-1 blurb in agent turns: remove agent-side untested warning and report only test-run edits to the host

**Status**: Closed — agent turns no longer print the quota-1 line; a run reports files edited while it ran (11439ed3)
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1227 (false "untested" line after a branch switch), 1275, docs/SessionIsolation.md

---

## 1. Problem & Motivation
Every `harnez agent` turn ends with "harnez: N file(s) changed after the last quota-1 run at ...
— code is untested, run make test-q1" whenever the repo has edits since the last test run. The line
goes to the agent's and the host's view, tells the agent to run the suite (which the host owns), and
repeats on every turn until someone runs `make test-q1`. It is environment noise, not agent error.

The line is not always wrong. On 2026-11-25 the last recorded run started at 12:11:31Z and
`internal/repocfg/get_test.go` was written at 12:12:00Z, inside the run window. The snapshot was
taken at start, so the run tested the old content, and the commit that followed carried the new
content untested. The warning was right, but nothing told the host at the time the run ended.

## 2. Technical Specification / Findings
- Agent-side emitter: `warnQuota1Changes` (`cmd/harnez/agent_run.go`), called after every turn in
  start, resume and stream paths, gated by `quota1Advised` (Makefile target plus `rules.quota`).
- The snapshot is taken at run start (`internal/quota1/snapshot.go`, `snapshotRun`). Files edited
  while the run is in progress are not in it and are not detected at run end.
- `internal/exec/exec.go` calls `quota1.FinishRun` after the run without comparing the tree again.

## 3. Implementation & Verification Plan
1. Remove the agent-turn warning: drop the `warnQuota1Changes` calls and `quota1Advised` from
   `cmd/harnez/agent_run.go`. Agents report untested edits through the existing rule in
   `.harnez/rules/Quota.md` ("Report Untested Edits").
2. Host side: when a quota-1 run ends, `quota1.EditedDuringRun` compares the tree with the start
   snapshot. If eligible files changed during the run, `harnez exec` prints one line on stderr naming
   them (up to five) and says that the run does not cover them. No new state is needed: the start
   snapshot still differs from those files, so the next `CheckAndRecord` allows a rerun.
3. Remove the mtime-based `ChangesSinceLastRun`/`ChangesSinceLastRunAfter`; they caused the noise and
   had no caller after step 1. Content hashes from the start snapshot decide.
4. Tests: agent turns print nothing about quota-1 (replace the stderr assertions in
   `cmd/harnez/agent_quota_test.go`); a run whose file changes during the command reports them
   (`TestRunExecWrapper_Quota1ReportsEditsDuringRun`); a quiet run reports nothing; `TestEditedDuringRun`
   covers ignored, markdown and issue files.
5. Verify: `HTO=0 make test-q1` green; then a manual check that an `harnez agent` turn over a dirty
   tree prints no quota-1 line.

## Open choices (decided here)
- The host line prints only for edits made during a test run. Edits between runs are not announced
  by any command; `make test-q1` itself is the place to learn about them.
- Quota-1 stays advisory and no rule text changes in this ticket.
