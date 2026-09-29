# 640 — Stop sound hook: player fallback chain or Go-native playback, never blocking

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Config & Hooks
**Related**: [[335-support-afplay-audio-notifications-on-macos-in-default-hooks]], config.yaml `hooks`, commit d2467af0

---

/goal Replace the hardcoded `ffplay` Stop hook with a sound command that tries several players
(or plays natively in pure Go, no CGo) and never blocks the agent; verify with a sleeping or
missing audio sink; or stop and report when blocked on a user decision.

## 1. Problem
The Stop hook in `config.yaml` runs `ffplay`. On 2026-09-29 it hung for minutes in two Claude
sessions while the Bluetooth sink was asleep; commit d2467af0 capped it with `timeout 3`. It
still depends on one player being installed, and the file path is Linux-only (issue 335).

## 2. Direction
- A harnez subcommand (e.g. `harnez notify sound`) that the hook calls, so logic lives in Go.
- It detaches (returns at once) so the Stop hook never waits, with a hard timeout on the player.
- Option A, fallback chain: first available of `pw-play`, `paplay`, `canberra-gtk-play`,
  `aplay`, `ffplay` (Linux), `afplay` (macOS); all four Linux ones are present on this machine.
- Option B, Go-native without CGo: decode a small embedded sound and write it to the audio server
  directly (e.g. PulseAudio native protocol over its socket, which PipeWire serves). To verify:
  common libraries such as `oto` need CGo for ALSA on Linux, so a pure-Go path may mean a
  PulseAudio client library or a hand-written minimal one.
- Player list, sound file and timeout belong in `spec/` (AGENTS.md rule), not in Go.

## 3. Open points
- A or B, or A now and B later.
- Whether this absorbs issue 335 (macOS `afplay`).
