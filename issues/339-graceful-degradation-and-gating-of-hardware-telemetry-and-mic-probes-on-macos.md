# 339 — Graceful degradation and gating of hardware telemetry and mic probes on macOS

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: UX & Reliability (Multi-OS Parity)
**Category**: Usage & Watch / Multi-OS
**Related**: [docs/MacOSPortability.md](../docs/MacOSPortability.md), [issues/286](286-promote-golang-org-x-term-for-terminal-operations-in-go-conventions.md)

---

## 1. Context & Architecture Decision

Per the OS-Agnostic architecture established in `docs/MacOSPortability.md`:
1. **Hardware Telemetry (Low Priority for Darwin)**: Detailed hardware monitoring (per-core CPU % from `/proc/stat`, RAM breakdown from `/proc/meminfo`, thermals from `/sys/class/thermal/`, power/battery from `/sys/class/power_supply/`) can remain Linux-first. We do not need complex native macOS hardware collectors immediately.
2. **Audio/Mic Activity (Nice-to-Have)**: Microphone volume and live audio metering via Linux daemons (`pactl`, `pw-record`, `amixer`) is nice-to-have. On macOS or non-Linux systems where these daemons are absent, the watch dashboard should not log errors, crash, or attempt broken subprocesses.
3. **Graceful Degradation Requirement**:
   - On macOS / non-Linux OSes, unsupported telemetry graphs (thermals, battery, per-core CPU breakdown) must be cleanly hidden or rendered in a compact/fallback layout without breaking grid alignment.
   - `internal/usage/load.go` must use Go build tags (`load_linux.go`, `load_darwin.go`, `load_fallback.go`) so that reading non-existent `/proc` and `/sys` paths is never attempted on Darwin.
   - On macOS, `CPULoad` should return basic system load averages (via `getloadavg` / `sysctl`) and mark unsupported fields as `Ok: false`, so the TUI renders only what is valid.
   - `internal/usage/mic.go` should return `MicUnavailable` cleanly when no supported audio backend is found, hiding the mic meter without error logs.

## 2. Technical Specification

1. **Build Tag Split for `load.go`**:
   - Extract Linux-specific `/proc` and `/sys` reading functions from `internal/usage/load.go` into `internal/usage/load_linux.go`.
   - Create `internal/usage/load_darwin.go` and `internal/usage/load_fallback.go` providing:
     - `readLoadAvg()`: using standard `getloadavg()` or `sysctl vm.loadavg`.
     - `readMemory()`: basic system memory fallback.
     - `readCPUPackageTemp()`: returns `TempOk = false` (cleanly hidden).
     - `readBattery()`: returns `BatteryOk = false` (cleanly hidden).
2. **TUI Render Invariants**:
   - Verify `internal/usage/watch.go` renders cleanly when `TempOk == false`, `BatteryOk == false`, and `CPUPercentOk == false`.
   - Ensure the compact view and full grid adjust gracefully to the absence of Linux-specific metrics.

## 3. Implementation & Verification Plan

1. Split `internal/usage/load.go` into build-tagged platform files.
2. Ensure `GOOS=darwin go test ./...` and `GOOS=linux go test ./...` compile and pass all test assertions.
3. Add unit tests for `load_darwin.go` and `load_fallback.go` mock states.
