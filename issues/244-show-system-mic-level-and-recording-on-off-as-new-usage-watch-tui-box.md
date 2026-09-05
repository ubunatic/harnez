# 244 — Show System MIC Level and Recording On/Off as New `harnez usage --watch` TUI Box

**Status**: Open — filed via /issue
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX / Agentic Ergonomics
**Related**: `internal/usage/watch.go` (existing TUI box layout), [[085-watch-tui-show-collector-daemon-status]]
(similar "small status box" precedent), `voxi` (github.com/ubunatic/voxi — the standalone voice-input
project extracted from harnez per issue 029; may already own mic-level probing and be the more natural
home for this rather than duplicating it in harnez)

---

## 1. Problem & Motivation

The user wants a new box in `harnez usage --watch`'s TUI showing:

- The current default system microphone input level (a live/periodic meter, not a one-shot read).
- Whether recording is currently on or off (i.e. some process actively capturing from the mic).

No existing code in this repo touches audio devices, ALSA, PulseAudio, or PipeWire — grepped
`amixer|pactl|pulseaudio|pipewire|alsa` across all `.go` files with zero real hits. This is new
capability, not a small tweak to an existing box.

## 2. Open Questions (exploratory — filed to capture the idea, not yet scoped)

- **Where should this live?** Voice input (`voxi`) was deliberately extracted out of harnez as its
  own standalone project (issue 029). If `voxi` already has mic-level/recording-state plumbing,
  this may belong there (or as a small status file/socket `voxi` writes that harnez's watch TUI
  reads), rather than harnez growing its own audio-device probing code from scratch.
- **How is "mic level" sourced?** Candidate mechanisms on Linux: `pactl` / `pipewire`
  peak-volume monitoring, ALSA `amixer` capture-volume polling, or reading whatever `voxi` already
  exposes. Per this repo's Canary-first practice (`docs/practices/Canary.md`), whichever mechanism
  is chosen should be canary-probed for availability/permissions before building the feature on it.
- **How is "recording on/off" determined?** Likely distinct from level (a mic can be open with
  noise-floor level while nothing is actually consuming it) — needs its own signal, not inferred
  from level alone.
- **Placement/format**: per the [[085-watch-tui-show-collector-daemon-status]] precedent, keep it
  minimal by default (a compact box or status line) rather than a large permanent panel, unless the
  user wants it prominent — not specified here.

## 3. Implementation & Verification Plan

Not yet planned — needs a design/feasibility pass (likely a read-only advisor dispatch per
`docs/practices/AgenticLoop.md`) to resolve §2's open questions, especially whether this belongs in
`voxi` instead of harnez, before any code is written.
