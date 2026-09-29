# 640 — Stop sound hook: player fallback chain or Go-native playback, never blocking

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Config & Hooks
**Related**: [[641-research-pure-go-no-cgo-sound-playback-for-notifications]], [[335-support-afplay-audio-notifications-on-macos-in-default-hooks]], config.yaml `hooks`, commit d2467af0

---

/goal Replace the hardcoded `ffplay` Stop hook with a sound command that tries a chain of
installed players and never blocks the agent; verify with a sleeping or missing audio sink; or
stop and report when blocked on a user decision.

## 1. Problem
The Stop hook in `config.yaml` runs `ffplay`. On 2026-09-29 it hung for minutes in two Claude
sessions while the Bluetooth sink was asleep; commit d2467af0 capped it with `timeout 3`. It
still depends on one player being installed, and the file path is Linux-only.

## 2. Direction
- A harnez subcommand (e.g. `harnez notify sound`) that the hook calls, so logic lives in Go.
- It detaches (returns at once) so the Stop hook never waits, with a hard timeout on the player.
- Fallback chain (decided 2026-09-29): first available of `pw-play`, `paplay`,
  `canberra-gtk-play`, `aplay`, `ffplay` (Linux), `afplay` (macOS); the four Linux ones are
  present on this machine. Pure-Go playback is researched separately in issue 641.
- Player list, sound file and timeout belong in `spec/` (AGENTS.md rule), not in Go.
- macOS (merged from issue 335): `ffplay` and the freedesktop sound are absent there; use
  `afplay` with a system sound such as `/System/Library/Sounds/Glass.aiff`. The sound file is
  chosen per OS. Linux configs must keep working unchanged after `apply`.
