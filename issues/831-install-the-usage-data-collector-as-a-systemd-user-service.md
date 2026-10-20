# 831 — Install the usage data collector as a systemd user service

**Status**: Closed — harnez server install/uninstall/status manage the systemd user unit (tested with a fake systemctl; status reports the socket owner vs MainPID live). Not enabled on this host: the user decides; run 'harnez server install' to enable.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 829, 830, 535, 082, 152

---

/goal A user can install the usage data collector (the single per-host writer from 829) as a systemd user service with one harnez command, and remove it again; on hosts without systemd the command says to run `harnez usage -w` instead and installs nothing. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
After 829 the collector is the only writer of the usage store, and `harnez usage` starts one when none
runs. A collector that only lives while `usage` runs leaves gaps. A user service keeps it running
across sessions and reboots.

## 2. Technical Specification / Findings
- Decided (user, 2026-10-20): systemd user service is the MVP. On other platforms the user runs
  `harnez usage -w`, which keeps a collector alive for as long as it runs (829).
- Reuse `harnez-agent-collector.service`: its `ExecStart` runs the current `harnez server`, now the
  single writer from 830. `harnez apply --systemd` remains a compatibility path that writes the unit.
- Lifecycle commands are `harnez server install`, `harnez server status`, and
  `harnez server uninstall`; service-specific settings live in `spec/server.yaml`.
- The service keeps `Restart=on-failure`. A successful version takeover exits 0, so systemd does
  not restart over the newer detached server. Status compares the socket PID with systemd's
  `MainPID` and prints the restart command when another process owns the socket.
- Other platforms or hosts without a running user manager print the `harnez usage -w` fallback
  and install no files.

## 3. Implementation & Verification Plan
- Commands to install, uninstall and show status; install enables and starts the unit.
- Tests with a temp home: the unit file content and path; the non-systemd branch prints the
  `harnez usage -w` hint and writes nothing.
- Tests use a temporary unit directory and a fake `systemctl`; no real user service is activated
  during development. The host will enable the service after user approval.
