# 255 — Collector Resilience & Diagnostics: Startup Retries, Error Hints, and TUI Logs Overlay (`l`)

**Status**: Open — partial Claude auth recovery; general retries and logs overlay absent
**Priority**: P2 (Medium)
**Severity**: Moderate (transient collector failure during startup splash causes false-positive failure badges and opaque errors)
**Category**: Architecture / UX / Diagnostics
**Related**: [[252-splash-completed-source-badges-agy-claude-mic-under-fetch-status-log]],
[[254-splash-brand-glyphs-agy-claude-codex-mic-with-green-red-status-instead-of-checkmarks]],
[[105-surface-per-collector-fetch-status-in-usage-ui]],
[[113-record-collector-roundtrip-times-usage-meta]],
[[169-splash-status-log-line-per-fetch-stage]],
`internal/usage/watch.go`, `internal/usage/usage.go`, `internal/usage/codex.go`

---

## 1. Problem & Motivation

### Audit — 2026-09-10

- **Conclusion: partially solved.** `CollectClaude` now invokes the Claude CLI
  after an unauthorized response, rereads credentials, and retries the quota
  API (`28c53a8`). `TestCollectClaudeRefreshesThroughClaudeCLIAfterUnauthorized`
  verifies the refreshed credential is used. This is one auth recovery path,
  not the cross-collector transient startup retry policy requested here.
- `collectAll` still calls each collector once and immediately reports its
  final `QuotaFetchError` through `FetchFailed`. There is no shared collector
  event ring buffer or scrollable logs overlay in `watch.go`; key dispatch
  and `spec/actions.yaml` expose controls/debug views, not the proposed logs
  action. Splash badges and fetch-duration estimates are partial diagnostics.
- **Measured:** `go test ./...` passes. Existing splash/status, duration, and
  Claude refresh tests do not cover general transient retries, log scrolling,
  or logs-overlay key handling. Those acceptance criteria remain open.

During `--watch` startup splash, collector probes (such as Codex, Claude, or AGY) can occasionally fail
on the first cold attempt due to transient network latency, token refresh delays, or locked cache files.
Because `CollectAllProgress` currently reports `FetchFailed` immediately on the first attempt without a
retry, the splash screen displays a red failure badge (`֍ codex` in red) even when a second attempt a
moment later would succeed.

Furthermore, when a probe does fail:
1. **Opaque Failure**: The user has no immediate visibility into *why* the collector failed (e.g. HTTP 401,
   token expiry, network timeout, daemon unreachable, or JSON parse error).
2. **No Interactive Logs View**: The `usage --watch` TUI provides view presets for agents, processes, mic,
   and controls (`?`), but lacks a dedicated diagnostic/logs overlay or panel (e.g. keyboard shortcut `l`)
   to view recent collector run logs, timestamps, error messages, and roundtrip latency.

## 2. Technical Design & Architecture

### 2.1 Startup Collector Retry Mechanism
- In `internal/usage/usage.go` (and individual collectors like `CollectCodex`, `CollectClaude`, `CollectAGY`),
  introduce a bounded retry mechanism for startup / initial fetch (e.g. 2 attempts with a short 200–500ms
  backoff on transient network or 429/5xx errors).
- Distinguish between fatal auth errors (missing keys/credentials) and transient I/O or token refresh glitches.
- Only report `FetchFailed` to the progress callback if the retry attempt also fails.

### 2.2 Collector Diagnostics & Ring Buffer Log
- Introduce an in-memory rolling event log (e.g. `CollectorLogEntry` with timestamp, source, stage, duration,
  status code / error string, and cached vs live flag).
- Record entries whenever collectors start, finish, retry, or fail across local and remote fetch cycles.

### 2.3 Interactive TUI Logs Overlay (`l`)
- Add a keyboard command `l` (and document it in the Controls overlay `?` and footer):
  - Toggles a full-screen or sliding diagnostic logs overlay showing the last N collector events,
    error messages, HTTP status codes, and latency figures.
  - Supports scrolling (Up/Down/PageUp/PageDown) or closing with `Esc`, `q`, `Enter`, or `l`.
- Splash Hint: If any collector reports `FetchFailed` during the startup splash, display a subtle hint:
  `"Press 'l' in dashboard to view fetch diagnostics"`.

## 3. Scope of Implementation

1. **`internal/usage/usage.go` / `codex.go` / `claude.go` / `agy.go`**:
   - Add transient retry logic on initial collector fetches.
   - Capture structured error diagnostics on failed fetches.
2. **`internal/usage/watch.go`**:
   - Add in-memory collector diagnostic log ring buffer.
   - Implement `buildLogsOverlayLines` rendering the logs viewer.
   - Wire key `l` in `dispatchWatchKey` to toggle the logs overlay.
   - Update `controlsOverlayLines` (`?`) with the new `l` Logs shortcut.
3. **`internal/usage/watch_test.go`**:
   - Unit test key dispatch for `l`, overlay rendering, scrolling, and retry behavior.

## 4. Acceptance Criteria

- [ ] Transient collector network/token blips on first launch retry once before marking the probe as failed.
- [ ] Detailed error strings from failed fetches are captured in an in-memory diagnostic log.
- [ ] Pressing `l` in `harnez usage --watch` opens a scrollable Logs/Diagnostics overlay showing recent collector events, latencies, and error messages.
- [ ] Pressing `?`, `Esc`, `q`, or `l` closes the overlay.
- [ ] The Controls overlay (`?`) lists `l` under available shortcuts.
- [ ] All tests pass (`make check`).

## 5. Verification

- **Automated**: `go test -v ./internal/usage/...` covering logs overlay dispatch, formatting, and retry logic.
- **Manual Verification**: Run `harnez usage --watch`, press `l` to inspect the collector event log, verify error details and clean exit.
