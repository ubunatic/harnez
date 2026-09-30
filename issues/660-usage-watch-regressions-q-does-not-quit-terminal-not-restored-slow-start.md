# 660 — usage --watch regressions: q does not quit, terminal not restored, slow start

**Status**: Closed — Fast interactive startup, prompt quit and terminal restoration, and visible logged compact fallback verified
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 650/657 compact work; 659 statusline latency; [watch loop](../internal/usage/watch.go), [PTY lifecycle tests](../internal/usage/watch_lifecycle_test.go)

---

## 1. Problem & Motivation

Reported on 2026-09-30: `harnez usage --watch` ignored `q`, left the terminal
unrestored when interrupted during slow work, and took much longer to load.
Acceptance: first frame under one second, immediate q/Ctrl-C handling,
restoration on every controlled exit, visible/logged compact projection errors,
regression tests, `make test-q1`, installation and installed-binary verification.

## 2. Technical Specification / Findings

- Inspected live code and `git log -8 -- internal/usage/` before editing.
  Built HEAD and `5e1e6b2a^` into temporary binaries using an archived checkout;
  the historical binary was never installed and no worktree was created.
- PTY baseline: HEAD splash 0.054–0.088s, actual dashboard 14.26–18.63s;
  `5e1e6b2a^` splash 0.071s, actual dashboard 5.21s. q during the splash was
  ignored in both versions. q after the HEAD dashboard sometimes exceeded the
  two-second probe deadline, with no restore sequence emitted before forced exit.
- The splash dispatcher deliberately ignored q/Q. More importantly, a quit-time
  stack dump identified `buildWatchFrameAt -> HistoryStats -> ReadHistory ->
  ImportUsageCompatibility -> importUsageSummaryHistory -> WriteUsageSummary`
  on the UI thread, executing SQLite writes/fsyncs while exit cleanup waited.
  Legacy history import had no consumed-content marker, so unchanged records
  were replayed on every cache/history read. The live compact store seam itself
  was only used by the one-shot command and was not the direct watch blocker.
  These direct findings made further bisecting unnecessary.
- CPU/GPU cold history seeding added another approximately two seconds of
  deliberate sleeps. Watch now grows these timelines on normal ticks.

## 3. Implementation & Verification Outcome

- Paint an interactive dashboard immediately with the spec-defined
  `Collecting usage…` label, then collect asynchronously. Startup and subsequent
  refreshes use the same single-worker path; frame state and output stay owned
  by the UI thread. History statistics are computed during collection and reused
  during redraws, including the History panel's measurement/render passes.
- Mark successfully imported history by path/content hash; skip unchanged files
  and still consume changed files. Propagate cancellation through cache/history
  reads and Codex rollout scanning.
- Replace external stty setup with raw termios handling on a nonblocking tty.
  Restore termios, SGR, cursor and alternate screen before draining canceled
  collection; close/drain the key reader, SIGWINCH listener and remote manager.
  Output failures return an error through the same cleanup path.
- `make test-q1` passed after the final source edit: formatting, `GOWORK=off go
  vet ./...`, and `GOWORK=off go test ./...` (usage package 6.564s). PTY tests
  cover q/Q/Ctrl-C/Esc during blocked startup, q/Ctrl-C during blocked refresh,
  context cancellation, SIGTERM, SIGINT and output failure. They compare complete
  before/after termios and assert cursor/alternate-screen restoration. Store tests
  assert unchanged history performs no replay writes, changed files import, and
  projection errors retain providers, preserve quota values and log with DEBUG off.
- `make install` passed. PTY verification used `/home/uwe/go/bin/harnez`:
  initial loading dashboard 0.041–0.047s; populated Codex quota frame 1.94–3.98s
  depending on collection. Thus initial interaction meets <1s without pretending
  live quota collection itself is instantaneous. Completed-collection q/Ctrl-C
  process exits took 0.022–0.024s. During active collection, exits took
  0.138–0.632s to drain; terminal restoration occurred in 0.00014–0.00195s.
  All installed runs restored complete termios, cursor and alternate screen,
  including `--watch --compact`.
- The first quota gate invocation did not run tests because an inherited quota
  record had no result. `harnez clean procs q1` confirmed its process group was
  gone; verified cleanup released it. The subsequent gate passed without bypass.
  An unrelated pre-existing untracked config file was preserved during the gate
  and restored afterwards. No subagents or background agent sessions were started.

## Related: Codex sometimes missing from compact

The one-shot caller previously discarded a failed projection's result and kept
only the live summary, potentially omitting a provider whose data only existed
in the store. Projection failure now preserves collected values and includes a
visible error for every supported provider whose store presence cannot be
established. It always appends the diagnostic to `~/.harnez/debug.log`, regardless
of DEBUG. Compact watch also uses this projection/fallback path in its worker.

Installed fault injection used `usage --offline --compact` with an invalid
`XDG_DATA_HOME` database root, leaving the real database untouched. Exit was zero;
Claude and Codex error rows, AGY's Claude/GPT and Gemini error pool rows, and the
new debug.log diagnostic were all verified. The initial canary expected the AGY
provider name rather than its established compact pool labels; correcting that
assertion passed without changing code. The parked archived patch was not needed.

No remaining blocker. Full live collection remains asynchronous and can take
several seconds; first interaction and terminal cleanup no longer wait for it.
