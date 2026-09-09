# 295 — Splash: Actionable In-Place Operation Status With Duration and Failure Reasons

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: [[169-splash-status-log-line-per-fetch-stage]] (introduced the rolling splash status
line and progress callback), [[252-splash-completed-source-badges-agy-claude-mic-under-fetch-status-log]],
[[254-splash-brand-glyphs-agy-claude-codex-mic-with-green-red-status-instead-of-checkmarks]],
[[255-collector-resilience-diagnostics-startup-retries-error-hints-and-tui-logs-overlay-l]] (broader
retry and persistent diagnostics work), `internal/usage/usage.go`, `internal/usage/remote.go`,
`internal/usage/watch.go`, `internal/usage/{usage,remote,watch}_test.go`

---

## 1. Problem & Motivation

The `harnez usage --watch` startup splash reports collector lifecycle events, but its terminal
messages are too vague to help a user understand the result. A source currently moves from text
such as `fetching agy...` to `agy done` or `agy failed`; success gives no timing information and
failure gives no reason. The start and terminal transitions are also independent queued events,
rather than one operation status whose visible line resolves in place.

Make each splash operation read as an actionable lifecycle: for example, show
`loading agy stats...` while it runs, then replace that same status row with
`loading agy stats ✔ (3s)` on success or `loading agy stats ✘ (<error reason>)` on failure.
The exact verb and duration precision may follow existing UI style, but the source, activity,
terminal outcome, and useful context must remain recognizable.

## 2. Current Implementation & Scope

- `FetchProgressFunc` in `internal/usage/usage.go` currently carries only `(source, FetchStage)`.
  It cannot transport an operation start time, elapsed duration, or failure detail.
- `CollectAllProgress` can obtain local failure detail from `AgentUsage.QuotaFetchError`, but
  `reportDone` reduces that to `FetchFailed`. `CollectRemoteProgress` similarly receives the SSH
  or parsing error from `doCollectRemote` and discards it when reporting the terminal stage.
- `splashStatusEvent` and `splashStatusState` in `internal/usage/watch.go` retain only source/stage.
  `splashStatusLine` therefore renders only `fetching <source>...`, `<source> done`, or
  `<source> failed`. The 300 ms queue pacing prevents fast events from disappearing, but does not
  correlate a terminal event with its start or make the outcome actionable.
- Local Claude, AGY, and Codex collectors run concurrently. Mic reporting and the opaque remote
  SSH operation also use the same callback, so lifecycle correlation must not depend on global
  FIFO adjacency.
- The splash exists only for `--watch`. Bare `harnez usage`, `--raw`, and `--json` are supported
  one-shot/non-interactive surfaces and must not gain progress text or ANSI redraw noise. A
  scripted watch capture must use a real PTY; `scripts/canary-watch-pty.sh` documents that contract.
- Keep this ticket focused on the transient startup splash. Retries, a dashboard logs overlay,
  and persistent diagnostics remain issue 255/256 scope.

## 3. Implementation Guidance

- Introduce a structured progress event (or equivalent additive callback contract) that can
  correlate each source operation's start and terminal result and carry concise failure detail.
  Preserve nil-callback behavior for non-watch callers.
- Measure elapsed time from the matching operation's reported start to its terminal transition,
  using a testable clock or timestamps captured at the lifecycle boundary. Do not use the whole
  splash duration as a per-operation duration.
- Model the visible entry as one operation that changes state. Preserve the splash's fixed,
  non-scrolling layout and existing spec-driven colors while avoiding confusing interleaving when
  collectors finish in a different order than they started.
- Derive failure text from the collector/remote error already available at the reporting site.
  Keep it concise and safe for a single terminal row: strip control characters/newlines and fit or
  truncate it within the current terminal-width budget without hiding the source and failure mark.
- Decide explicitly how cached/no-live-work paths and the synthetic mic operation behave. Do not
  invent a duration or success result for an operation that never emitted a start event.

## 4. Acceptance Criteria

- [ ] Every live operation reported during the `--watch` startup splash first shows a clear
      in-progress message naming the source and activity (for example, `loading agy stats...`).
- [ ] A successful operation updates the same splash status row to a success mark and a
      human-readable elapsed duration measured from that operation's own start (for example,
      `loading agy stats ✔ (3s)`).
- [ ] A failed operation updates that row to a failure mark plus a concise, actionable reason
      obtained from the real collector, SSH, or parsing error (for example,
      `loading agy stats ✘ (credentials expired)`), rather than only `agy failed`.
- [ ] Concurrent local collectors and the remote-host operation correlate each terminal result
      with the correct start, without duplicate terminal entries, cross-source durations, or a
      terminal status appearing as an unrelated vague event.
- [ ] Long or multiline failure details are sanitized and width-bounded; the splash remains an
      in-place, non-scrolling TTY frame and retains the source, outcome mark, and useful reason on
      narrow supported terminal geometries.
- [ ] `--watch` behaves equivalently in an interactive terminal and a real-PTY scripted capture.
      Supported non-interactive one-shot modes (default compact output, `--raw`, and `--json`)
      remain free of splash lifecycle messages and redraw/control-sequence contamination.
- [ ] Cache-hit/no-live-operation paths and post-Esc frozen-splash behavior remain deterministic
      and do not fabricate or continue updating operation statuses.

## 5. Verification Guidance

- Extend `internal/usage/watch_test.go` around `splashStatusRecord`, `splashStatusAdvance`,
  `splashStatusLine`, and `buildSplashFrame` to assert in-progress-to-terminal replacement,
  per-source elapsed timing, error sanitization/truncation, concurrency ordering, narrow geometry,
  and frozen/post-Esc behavior.
- Extend `internal/usage/usage_test.go`'s
  `TestCollectAllProgressReportsStartedAndTerminalStageForEverySource` to cover structured success
  and real `QuotaFetchError` propagation while preserving `TestCollectAllProgressNilCallbackIsNoop`.
  Add corresponding remote callback assertions in `internal/usage/remote_test.go` for SSH/parse
  error detail.
- Run `go test ./internal/usage/...` and `make check`.
- Manually run `harnez usage --watch` for the live TTY path. Use
  `scripts/canary-watch-pty.sh <seconds>` and inspect its retained raw capture to verify that each
  status row visibly resolves in place. Exercise one controlled failure (such as an invalid remote
  target) and confirm the reason is useful, sanitized, and does not disturb the final dashboard.
