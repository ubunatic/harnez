# 256 — Persist Rolling Watch & Collector Launch Logs to Disk for Post-Mortem Diagnostics

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate (lack of persisted launch logs prevents root-cause analysis of intermittent failures across sessions)
**Category**: Architecture / Diagnostics / Observability
**Related**: [[255-collector-resilience-diagnostics-startup-retries-error-hints-and-tui-logs-overlay-l]],
[[113-record-collector-roundtrip-times-usage-meta]],
[[082-agent-usage-collector-daemon]],
`internal/usage/watch.go`, `internal/usage/collector.go`, `internal/usage/statecache.go`

---

## 1. Problem & Motivation

When `harnez usage --watch` launches or runs background collector probes, any errors, retry attempts,
or latency spikes are currently held only in transient process memory (or swallowed). Once the user
exits `--watch` or encounters an intermittent issue (such as the recent transient Codex probe failure
during splash), all diagnostic context is lost.

When investigating TUI launches or collector behavior from the last few hours, there are no persisted
log files on disk to inspect. Agents and developers must reproduce the issue live rather than inspecting
an authoritative event log of recent launches.

## 2. Technical Design

### 2.1 Persistent Log Location & Format
- **Directory**: `$XDG_STATE_HOME/harnez/usage/` (fallback `~/.local/state/harnez/usage/`), matching
  the existing `StateDir` used for quota snapshots and fetch durations.
- **Log File**: `~/.local/state/harnez/usage/watch.log` (or `collector.log`).
- **Entry Format**: Structured log lines (timestamp, session ID, source, stage/event, latency, error detail, cached flag):
  ```json
  {"ts":"2026-09-06T23:20:15Z","session":"a7b8c9","event":"fetch_failed","source":"codex","latency_ms":1240,"err":"http 401 unauthorized: token refresh pending"}
  ```

### 2.2 Bounded Rolling Retention Policy
- Prevent unbounded disk growth via a size- or line-capped rolling policy (e.g. max 1 MB or ~5,000 lines,
  rotating to `watch.log.1` or auto-pruning entries older than 48 hours).
- Appends must be non-blocking and best-effort: log write failures must never crash or block the TUI dashboard.

### 2.3 Inspection Surface
- Complement in-TUI diagnostics (ticket 255's `l` overlay) by allowing the overlay to load recent history
  from `watch.log`.
- Provide CLI access via `harnez usage logs` (or `harnez usage --logs`) to quickly dump or tail recent
  launch and probe events from the command line without opening the full TUI.

## 3. Scope of Implementation

1. **`internal/usage/logger.go` (or `diagnostics.go`)**:
   - Implement `LogWatchEvent(homeDir string, entry WatchLogEntry)` with atomic append and bounded rotation.
   - Implement `ReadRecentWatchLogs(homeDir string, limit int, since time.Duration) ([]WatchLogEntry, error)`.
2. **`internal/usage/watch.go` & `usage.go`**:
   - Hook launch lifecycle, startup probe results, retries, and errors into `LogWatchEvent`.
3. **`cmd/harnez/`**:
   - Expose `harnez usage logs` (supporting `-n <limit>` and `--since <duration>`).
4. **Unit Tests**:
   - Test log entry serialization, atomic appending, and bounded rotation / size caps.

## 4. Acceptance Criteria

- [ ] Every `--watch` launch, probe completion, retry, and failure appends a structured entry to `~/.local/state/harnez/usage/watch.log`.
- [ ] Disk usage is strictly capped (e.g. 1 MB / rolling backup) to prevent unbounded growth.
- [ ] `harnez usage logs` displays recent launch events from the last few hours with timestamps, durations, and errors.
- [ ] Log appends are resilient and non-blocking.
- [ ] All tests pass (`make check`).

## 5. Verification

- **Automated**: Unit tests covering log writing, reading, and rotation in `internal/usage`.
- **Manual Verification**: Run `harnez usage --watch`, exit, and run `harnez usage logs` to verify that startup events from the session are accurately recorded and readable.

