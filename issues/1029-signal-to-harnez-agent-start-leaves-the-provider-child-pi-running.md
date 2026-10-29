# 1029 — Signal to harnez agent start leaves the provider child (pi) running

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 661, 592, 1024

---

## Symptom
Twice on 2026-10-29 (1024 reruns on `lmcoder:coder`), stopping a synchronous `harnez agent start` left its `pi`
child running:
- Session `r1024`: `kill <harnez pid>` (SIGTERM). The `pi` child was reparented (PPID 5008) and kept talking to the
  model until it was killed by hand. `harnez agent list` still showed the session as `running`, and
  `harnez agent delete` needed `--force`.
- Session `r1024b`: `timeout 600 harnez agent start ...` (SIGTERM at 10 min, exit 124). `pi` was still alive
  afterwards and exited on its own shortly after.

A leftover child keeps using the shared local model server and can still edit files, after the caller thinks
the run is over. 661 fixed this for `harnez agent stop` (kills the process tree); the signal path of the
synchronous start itself was not covered.

## Acceptance
- SIGTERM, SIGINT and SIGHUP to `harnez agent start` (and `resume`) stop the provider child and its process group,
  then record the session as stopped, not `running`.
- A test starts a fake provider that sleeps, signals harnez, and asserts that no child process is left.
- Check the other providers (claude, codex, agy) on the same path.

## Searcher audit (2026-10-29, `haiku:low`, 35 s)
- Provider launch: `runProviderCLI` (`internal/subagent/process.go:100-110`) -> `exec.CommandContext`; `IsolateProcess`
  sets `Setpgid` (`process.go:44`) and `cmd.Cancel` SIGKILLs the group on context cancel (`process.go:56-58`). Claude
  same path (`claude.go:22`); codex `startProcess` (`codex.go:593-598`) also `Setpgid`, cancels with SIGINT.
- No `signal.Notify`/`NotifyContext` on the start path; `root.Execute()` (`cmd/harnez/clilog.go:183`) uses a
  background context. Go's default SIGTERM action exits without running `Cancel`, and no `Pdeathsig` is set, so
  the child lives on. Signal handling exists only in `usage_cmd.go:365,406`, `server_cmd.go:83`, `channel.go:38`.
- Status: sync start writes `failed` on error (`cmd/harnez/agent_run.go:581`) and `stopped` for the watchdog
  (`agent_run.go:554`); `reportSessionStop` writes `stopped` and clears PIDs (`agent_stop.go:93-97`).
- Test templates: `internal/subagent/claude_stream_test.go:160` `TestClaudeStreamCancelKillsProcessGroup`
  (fake provider runs `sleep 30 &`, writes the PID, asserts it is dead); `cmd/harnez/agent_stop_test.go:21,46`
  (`TestStopFakeProviderWrapper`, `TestStopProviderTreeAndWrapper`, helper-process fixtures).

## Decision
- Host, 2026-10-29: Claude models only (OpenAI and Gemini quota exhausted): developer `opus:low`.
- Host: fix in harnez, not in each driver: a `signal.NotifyContext` (SIGINT, SIGTERM, SIGHUP) for `agent start`
  and `agent resume` whose cancel reaches the existing `cmd.Cancel` group kill, then the session is saved as
  `stopped` (as the watchdog does) and harnez exits non-zero. No `Pdeathsig`: it is Linux-only and would not
  update the session record. SIGKILL to harnez stays out of scope (cannot be caught).

## Milestones
- M1 (signal stops the child): the fix above for start and resume, all providers through the shared path.
  Reproduction test first: start `harnez agent start` (or the start function) with a fake provider that runs
  `sleep 30` and records its PID, send SIGTERM to harnez, assert the child is gone within a bounded wait and the
  session status is `stopped`. Also SIGINT. Negative case: a normal completed turn still ends `done`, not `stopped`.
- M1 plan accepted (developer `d1029`, 58 s): `agentSignalContext` with `context.WithCancelCause` in new
  `cmd/harnez/agent_signal.go`, wired in `withAgentTimeout` (`agent_run.go:154`, covers all sync start/resume
  call sites) and the detached runner (`agent_async.go:362,413`); signal cause saved as `stopped` in the start
  error branch and before `recordResumeFailure` (not counted as a resume failure); tests in new
  `cmd/harnez/agent_signal_test.go` reusing the `agent_stop_test.go:21` fixture; doc paragraph in
  `docs/HarnezAgentArchitecture.md`.
  Host pre-work for the write turn:
  - After the first signal, restore default handling (call the NotifyContext stop) so a second Ctrl-C or SIGTERM
    ends harnez at once if the cleanup hangs.
  - Exit code: plain non-zero is enough; add 128+signal only if `clilog.go` already carries exit codes.
  - The codex case must not make the test wait for the 15 s `WaitDelay`: if the fake codex ignores SIGINT, assert
    the bounded outcome you actually get and keep the test under a few seconds.
  - A `--timeout` deadline must still be reported as a timeout, not as a signal (test it).
- M2 (live check, host): `timeout 20 harnez agent start --model lmcoder:coder ...` with a long prompt; afterwards
  no `pi` process is left and `harnez agent list` shows the session stopped.

## Delivery
- M1 delivered (`e0a35fed`, signal stops the child): `agentSignalContext` in `cmd/harnez/agent_signal.go`, wired in
  `withAgentTimeout` and both detached runners; start and resume save `stopped` with `interrupted by <SIG>`, resume
  does not count it as a failure; a second signal gets the default action. Tests in `agent_signal_test.go`: claude
  start SIGTERM/SIGINT, claude resume SIGTERM, codex start SIGTERM failed before the fix ("provider child still
  running") and pass after (harnez exits 3-23 ms after the signal); a normal turn stays `completed`; `--timeout`
  stays a deadline, not `stopped`. Developer `make test-q1`: exit 0, no `--- FAIL`, 129 packages ok. Host rerun
  `go test ./cmd/harnez -run 'Signal|Stop'`: ok. Not tested: SIGHUP and the second signal (same code path).
- M2 delivered (live, host, 2026-10-29 23:25): `timeout 25 harnez agent start --role reviewer --model lmcoder:coder`
  ended with `interrupted by SIGTERM; provider stopped and session "m2-1029" can be resumed`, exit 124; no `pi`
  process left; `harnez agent status --name m2-1029` shows `Status: stopped`. A stopped session is not shown by
  `harnez agent list` (active sessions only), which is expected.

/goal Make a signal to a synchronous `harnez agent start`/`resume` end the provider child and mark the session
stopped, or stop and report when a provider cannot be stopped this way. Circuit breaker: on a loop or no progress,
clear the goal and write a failure report. Re-check live code first; this ticket was written on 2026-10-29.
