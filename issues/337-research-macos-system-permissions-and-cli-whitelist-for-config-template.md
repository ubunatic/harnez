# 337 — Research macOS system permissions and CLI whitelist for config template

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Security & Usability (Platform Parity)
**Category**: Permissions / Multi-OS
**Related**: [docs/MacOSPortability.md](../docs/MacOSPortability.md)

---

## 1. Problem & Motivation

`config.yaml` provides a pre-approved baseline permissions whitelist for Claude Code and Agent harnesses (`permissions.allow`). Currently, this whitelist is heavily Linux-specific:

- **Linux-only System Commands**:
  - `systemctl`, `journalctl`, `smartctl`, `lsblk`, `dmesg`, `vcgencmd`, `ip link`, `tune2fs`, `getent`, `dpkg`, `apt-cache`, `findmnt`
- **Linux-only System File Paths**:
  - `Read(//proc/**)`, `Read(//sys/**)`, `Read(//run/**)`

On macOS:
- These Linux commands either don't exist or fail to run.
- macOS has distinct system diagnostics, service management, package management, and system directory structures (e.g. `launchctl`, `log show`, `diskutil`, `brew`, `/Library/**`, `/System/Library/**`, `/private/**`).

## 2. Research Scope

1. **Service Management & Diagnostics**:
   - Equivalent safe read-only CLI commands on macOS (`launchctl list`, `log show`, `diskutil list`, `scutil`, `networksetup`, `system_profiler`, `brew list/info`).
2. **Safe System Read Paths**:
   - Safe vs sensitive paths on macOS (e.g. `/Library/**`, `/Applications/**`, `/usr/local/**`, `/opt/homebrew/**` vs protecting `~/Library/Keychains`, `~/Library/Messages`, `~/Library/Mail`).
3. **Template Management**:
   - How `harnez apply` should deliver OS-specific permission templates (e.g. merging a base common template with OS-specific overlays for Linux and Darwin, or platform section tags in `config.yaml`).

## 3. Deliverables

- Write a research report in `docs/studies/` detailing safe macOS command whitelists and path access boundaries.
- File follow-up implementation ticket for OS-aware permission generation in `harnez apply`.
