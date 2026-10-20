# 831 — Install the usage data collector as a systemd user service

**Status**: Open
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
- Prior art: 082 shipped a `harnez-agent-collector` systemd unit, and `internal/claude/systemd*.go` writes
  a unit to `~/.config/systemd/user/` with the binary path filled in. Check whether that unit is the same
  collector as 829's; reuse it, or replace it, rather than adding a second unit.
- The service and the auto-start from 829 share the fixed per-host port, so they never run twice.
- Uncertain: command name and place (`harnez usage collector install`? see 152). Whether `make install`
  or upgrades restart the service is 535's lifecycle question; leave it there unless trivial.
- Unit name, port and other values go in `spec/`.

## 3. Implementation & Verification Plan
- Commands to install, uninstall and show status; install enables and starts the unit.
- Tests with a temp home: the unit file content and path; the non-systemd branch prints the
  `harnez usage -w` hint and writes nothing.
- Live check on this host: install, `systemctl --user is-active`, `harnez usage` uses the service
  instead of starting a second collector, uninstall.
