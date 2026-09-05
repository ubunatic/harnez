# 244 — Show System MIC Level and Recording On/Off as New `harnez usage --watch` TUI Box

**Status**: Open — filed via /issue
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX / Agentic Ergonomics
**Related**: `internal/usage/watch.go` (existing TUI box layout), [[085-watch-tui-show-collector-daemon-status]]
(similar "small status box" precedent), `docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md`
(harnez's existing policy of sourcing device telemetry from generic OS-level interfaces rather than a
specific vendor/tool CLI — the same principle applies here: read whatever generic audio-server interface
the running system exposes, don't assume any particular voice-input tool is installed)

---

## 1. Problem & Motivation

The user wants a new box in `harnez usage --watch`'s TUI showing:

- The current default system microphone input level (a live/periodic meter, not a one-shot read).
- Whether recording is currently on or off (i.e. some process actively capturing from the mic).

This must work on **any** setup — it should read the system's audio server directly (whatever mic is
the current default input device), not depend on or assume any specific voice-input application being
installed. No existing code in this repo touches audio devices, ALSA, PulseAudio, or PipeWire — grepped
`amixer|pactl|pulseaudio|pipewire|alsa` across all `.go` files with zero real hits. This is new
capability, not a small tweak to an existing box.

## 2. Open Questions (exploratory — filed to capture the idea, not yet scoped)

- **How is "mic level" sourced, generically?** Candidate mechanisms on Linux: `pactl`/`wpctl`
  (PipeWire/PulseAudio) peak-volume monitoring of the default source, or ALSA `amixer` capture-volume
  polling as a fallback where no sound server is present. Should detect at runtime which audio stack is
  actually available rather than hardcoding one. Per this repo's Canary-first practice
  (`docs/practices/Canary.md`), probe availability/permissions before building the feature on it, and
  per the kernel-standard-metrics policy, prefer the most standard/portable interface available over a
  specific tool's CLI.
- **How is "recording on/off" determined?** Likely distinct from level (a mic can be open with
  noise-floor level while nothing is actually consuming it) — on PipeWire/PulseAudio this typically
  means checking whether any client currently holds the default source's monitor/capture stream open;
  needs its own signal, not inferred from level alone.
- **Placement/format**: per the [[085-watch-tui-show-collector-daemon-status]] precedent, keep it
  minimal by default (a compact box or status line) rather than a large permanent panel, unless the
  user wants it prominent — not specified here.
- **Cross-platform scope**: not specified whether non-Linux support (macOS `coreaudio`, etc.) is in
  scope — default assumption is Linux-first (matching this repo's other OS-level telemetry), to be
  confirmed before implementation.

## 3. Implementation & Verification Plan

Not yet planned — needs a design/feasibility pass (likely a read-only advisor dispatch per
`docs/practices/AgenticLoop.md`) to resolve §2's open questions before any code is written.
