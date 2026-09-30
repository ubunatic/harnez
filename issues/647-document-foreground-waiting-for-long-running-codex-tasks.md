# 647 — Document foreground waiting for long-running Codex tasks

**Status**: Open
**Priority**: P1 (High)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [Agentic Loop Practices](../docs/practices/AgenticLoop.md)

---

## 1. Problem & Motivation
Codex agents sometimes launch a long-running shell command in a detached/background agent session and then end the turn without a reliable way to collect its result. This can leave work orphaned or appear as a zombie, even though the shell tool already returned a live session ID that the host can continue waiting on.

Add explicit guidance for Codex: when a shell command returns a live session ID, keep that command tracked and wait for more output or completion with `functions.write_stdin` using that ID. Do not detach the task and then report completion without collecting its terminal result. Preserve existing guidance for other harnesses and for explicitly requested handoffs.

## 2. Technical Specification / Findings
The intended workflow is the normal Codex command lifecycle: `functions.exec_command` returns a session ID for a still-running command, and `functions.write_stdin` polls or sends input to that same session. This gives the host a concrete completion result and allows the task to be stopped if needed. Harnez-managed or other harness background facilities remain appropriate when their lifecycle is actually tracked and the caller is expected to remain responsive.

## 3. Implementation & Verification Plan
**Goal**: Update the canonical agent-loop guidance and its generated copy to explain how Codex agents wait on a running shell session, or stop and report if the Codex tool environment does not expose that mechanism.

Acceptance criteria:
- The guidance names `functions.write_stdin` and explains that it waits on the session ID returned by `functions.exec_command`.
- It requires collecting a terminal result or explicitly cleaning up the task before ending the session.
- The addition does not conflict with harness-specific background-task workflows or requested delegation.
- Regenerate/sync the managed documentation copy and verify the docs index/status.

## 4. Update 2026-09-30 — recurring failure, consider removing auto-detach

**Observed**: two Codex (Luna) sessions both hit the harnez agent auto-detach, left the detached
agents running and ended without reattaching. There was no synchronous wait to go back to.
Claude hosts handle the same flow correctly; only Codex fails. About 10 earlier guidance fixes
have not changed this.

**What Codex is currently shown** (likely source of confusion):
- `cmd/harnez/agent_async.go` `writeDetachGuidance`: "Agent turn exceeded 60s and has been cleanly
  detached ... Do NOT poll ... Launch `harnez agent wait <name>` as a host background job; the
  environment will automatically notify this session when it finishes." Codex has no such
  notification. Its only wait mechanism is `functions.write_stdin` on a live exec session. So
  the message tells Codex to use a facility it lacks, and "Do NOT poll" also rules out the one
  that works.
- The same wording appears in `cmd/harnez/agent_stream.go:138` and `cmd/harnez/agent.go:754`.
- `.harnez/rules/Tools.md:50` says to start agents "in a background shell, e.g. Claude
  `run_in_background`", which is Claude-only advice shown to every host.
- The auto-detach triggers whenever the outer exec timeout is set (`foregroundDetachTimeout`, 60s cap).

**Proposed direction** (to decide before implementing):
1. Preferred: remove the foreground auto-detach entirely. `harnez agent start` stays synchronous
   until the turn is done. Keep explicit `--detach/--async` only for callers that ask for it.
2. Fallback: detect a Codex host and never auto-detach there, or emit Codex-specific guidance.
   This is the approach that has failed repeatedly, so it's second choice.

**Revised goal**: /goal Stop Codex hosts from orphaning harnez agents. Remove the foreground
auto-detach (or disable it for Codex) and rewrite all detach/wait guidance so that each host is
told only about mechanisms it actually has. Verify this with a Codex canary run over 60s that
ends with the collected result. Stop and report if the user chooses to keep auto-detach.

## 5. Decision 2026-09-30 — keep auto-detach, host-specific reattach wording

The user chose to keep the 60s auto-detach (it works for Claude and agy) and never promote
`--detach`. Instead, the detach message and pre-turn wait hint now use each host's own terms
(`reattachSteps` in `cmd/harnez/agent_async.go`, host from `detachHost`):
- Codex (`CODEX_THREAD_ID`/`CODEX_CLI`): run `harnez agent wait` with `exec_command`, then
  `write_stdin` on the returned `session_id` until an exit code arrives. Do not end the turn before that.
  No "Do NOT poll".
- Claude: Bash `run_in_background: true`, which notifies on exit.
- agy: run it as a background *task*, which notifies when complete.
- Unknown hosts get the previous generic wording.
The message also states that the agent is still running and its result has not been collected yet.

Background (Codex self-reports): Codex has no shell job that notifies it on completion. Native async
exists only for `collaboration.spawn_agent` subagents (results arrive in the mailbox automatically).

**Next**: observe Codex/agy sessions for a few days. If Codex still orphans agents, revisit
removing auto-detach (§4 option 1). The `.harnez/rules/Tools.md` start hint (source `config.yaml`) now names each host's
wait tool; Codex wait tools are documented in `docs/CodexSettings.md`.
