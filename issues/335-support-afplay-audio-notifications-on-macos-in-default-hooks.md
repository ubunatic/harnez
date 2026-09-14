# 335 — Support afplay audio notifications on macOS in default hooks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: UX Polish (Platform Compatibility)
**Category**: Config & Hooks / Multi-OS
**Related**: [docs/MacOSPortability.md](../docs/MacOSPortability.md)

---

## 1. Problem & Motivation

The default `hooks` configuration in `config.yaml` includes a `Stop` lifecycle event hook that plays a notification chime when an agent completes its turn:

```yaml
hooks:
  - event: Stop
    command: "ffplay -nodisp -autoexit /usr/share/sounds/freedesktop/stereo/window-attention.oga 2>/dev/null || true"
```

On macOS (Darwin):
1. `ffplay` is not installed by default (requires FFmpeg via Homebrew).
2. The path `/usr/share/sounds/freedesktop/...` does not exist on macOS.
3. macOS ships with a built-in zero-dependency CLI audio player: `afplay`, and standard system sound assets under `/System/Library/Sounds/` (e.g. `Glass.aiff`, `Ping.aiff`, `Tink.aiff`).

## 2. Technical Specification

- Design an OS-aware hook command or runtime detection that invokes:
  - **Linux**: `ffplay -nodisp -autoexit /usr/share/sounds/freedesktop/stereo/window-attention.oga 2>/dev/null || true` (or `paplay` / `pw-play`)
  - **macOS**: `afplay /System/Library/Sounds/Glass.aiff 2>/dev/null || true`
- Investigate whether `harnez apply` should conditionally generate the platform-appropriate hook command based on `runtime.GOOS`, or provide a unified shell fallback (e.g. `command -v afplay ... || command -v ffplay ...`).

## 3. Implementation & Verification Plan

1. Update `config.yaml` / hook generation in `internal/claude/apply.go` to support macOS audio playback gracefully.
2. Test hook command execution on Linux and macOS environments.
3. Verify idempotency and ensure no breaking changes for existing Linux configurations.
