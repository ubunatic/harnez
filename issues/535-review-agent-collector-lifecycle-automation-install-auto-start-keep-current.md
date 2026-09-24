# 535 — Review agent-collector lifecycle automation: install, auto-start, keep current

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Discovery / Ops
**Related**: [[082-agent-usage-collector-daemon]], [[085-watch-tui-show-collector-daemon-status]], [[152-move-agent-collector-under-usage-command]], [[534-capture-agent-session-tokens-continuously-not-only-at-session-end]]

---

## Problem

On 2026-09-24, `systemctl --user is-active harnez-agent-collector` returned
`inactive` (exit 4 = unit not loaded). The daemon was not installed on the
dev machine, so live telemetry (e.g. session tokens, 534) silently falls back
or stays empty. Nothing installs it, starts it, or restarts it after an upgrade.

## /goal

Decide on and implement a lifecycle policy so that the collector is always
installed, running, and on the current binary version, with no manual
systemctl steps.

## Ideas to discuss (not decisions)

- `make install` installs the unit, and maybe enables and starts it.
- `harnez usage -w` checks the daemon and (re)starts it when it is missing or
  older than the running binary (compare the version or binary mtime).
- Keep it running with a systemd `Restart=` policy, and maybe a check hook at session start (see 274).
- Show daemon state in the TUI (085).
- Open questions: is auto-enabling a user service acceptable? What about
  machines without systemd? What about opting out?
