# 1259 — cmd/harnez and selfinstall tests fail inside taia: long socket paths and staged root permission

**Status**: Closed — Resolved: the named tests were removed with runner mode (859d787b), and the socket-path failures are fixed by the socktest helper (82e25c0a) that keeps every unix socket under 107 bytes; no harnez-side workaround was added
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Infrastructure
**Related**: 1181, 1245

---

## 1. Problem & Motivation

Inside a taia session with host mode forced (`env -u TAIA_SESSION_DIR`), four tests fail for
environment reasons only, so every run needs a manual baseline comparison to spot real failures.
That baseline masked two real failures in 1252.

## 2. Technical Specification / Findings

Observed on 2026-11-21, failing the same way on main:
- `TestTelemetryCollectorServiceRegistryOwnSessionAndLegacyRejection` and
  `TestTelemetryCollectorServiceUsageQueryViaRunnerRelay` (`cmd/harnez`): "Unix socket path exceeds
  107 bytes", because `t.TempDir()` under `TMPDIR=/tmp/taia-<id>/tmp` plus the long test name is too long.
- `TestInstallStagedCommandAndRunnerManGuards` (`cmd/harnez`) and
  `TestPromoteStagedAtomicPinnedSuccessAndSecurityRejections` (`internal/selfinstall`): "validate
  staged root: open root directory: permission denied" under Landlock.

## 3. Implementation & Verification Plan

- Socket tests: create the socket dir with a short `os.MkdirTemp("", "hz")` instead of nesting
  under `t.TempDir()`, or skip with a clear reason when the path cannot fit.
- Promoter tests: detect the Landlock denial and skip with the reason, keeping them active on hosts.
- Verify: the four tests pass on the host and skip (not fail) in a taia session.
