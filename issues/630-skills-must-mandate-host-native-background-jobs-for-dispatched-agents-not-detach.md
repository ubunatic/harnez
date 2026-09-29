# 630 — Skills must mandate host-native background jobs for dispatched agents, not --detach

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: 611 (host polling loop), 581, 125, 587, docs/commands/lean-sprint.md, docs/practices/AgenticLoop.md (anti-patterns "Shell `&` Detaching", "Chat-Visible Empty Polling")

---

## 1. Problem & Motivation

Observed in project `voxi` (Claude Code host, model Sonnet 5.5), running `/lean-sprint add "voxi chunks delete"`
(voxi issue 165). The host dispatched the developer with
`harnez agent start --role developer --model luna --detach ...` straight from the Bash tool, without `run_in_background`.

Consequences:

- The job is invisible in the Claude Code terminal task list and the user cannot see or stop it there.
- No completion notification reaches the host, so nothing wakes it when the worker finishes.
- The host then wrongly promised "when it reports back, I'll review". Nothing would have triggered that.

The user considers this a compliance failure: the harness expects a host-tracked background job. Honest
finding from the host's post-mortem: it could not find an explicit rule in the instructions it had loaded, so
it read "Stay Responsive ... return control" in `lean-sprint.md` §1 as satisfied by `--detach`. The
general rule exists only in `AgenticLoop.md` anti-patterns (use the harness background facility, e.g. Claude
`run_in_background`), which the skill does not reference at the point of dispatch.

Root causes in the instructions:

1. No step in `lean-sprint` / `reverse-sprint` / `reverse-sprinter` says how to launch the worker so the host is tracked and notified. "Return control" is satisfied by harnez's own `--detach`.
2. `--detach` / `--async` are advertised in `harnez agent start --help` and the skill as the way to avoid blocking, with no warning that they bypass host job tracking.
3. Nothing requires the host to have a completion signal before promising a follow-up review.
4. Skills give no per-host mapping of the background facility.

/goal Make the dispatch instructions (skills and `harnez agent start` help) unambiguous that milestone workers run as host-native background jobs, verified by a fresh host following the skill without using `--detach`; or stop and report when blocked on a user decision or denied permission.

## 2. Proposal

1. **Name the mechanism in every dispatching skill** (`lean-sprint`, `reverse-sprint`, `reverse-sprinter`, and the `docs/commands/*` sources): run `harnez agent start` (foreground form, no `--detach`) as a host background job, so it is visible to the user and its exit re-invokes the host. Per host:
   - Claude Code: Bash tool with `run_in_background: true`; watch with Monitor if progress events are needed.
   - Codex: its background terminal / long-running exec session (exact tool name to be verified against the current Codex tool list).
   - AGY: its background task facility (`manage_task` with completion notification; verify).
   - Any host without one: say so and fall back to the documented poll-free pattern (log file plus stated job id), never silent `--detach`.
2. **Restrict `--detach`/`--async`**: help text and skills say it is for hosts with no background facility; a host that has one must not use it for milestone workers. Consider a stderr warning when `--detach` is used under a detected Claude/Codex/AGY host.
3. **Completion-signal rule**: before telling the user a review will follow, the host must have a completion signal (background job exit, notification, or scheduled wakeup). Otherwise it says plainly that it will not be notified and how the user can prompt it.
4. **Say "return control" precisely**: the skill's "Stay Responsive" bullet should reference the mechanism above rather than only the outcome.
5. **Visibility check step**: after dispatch, the host confirms the job appears in the host task list and reports the job id and `harnez agent list` name to the user.
6. **Compliance test**: add a policy or doc test that the dispatching skill texts mention the per-host mechanism (mirrors the existing `internal/agentpolicy` tests).

## 3. Verification Plan

Re-verify against live docs and recent commits before starting (611 and 581 may already cover part of this;
merge rather than duplicate). Run a fresh Claude host through `/lean-sprint` on a trivial ticket and confirm the worker shows up in
the terminal task list and its exit wakes the host. Repeat check for Codex and AGY if available.
