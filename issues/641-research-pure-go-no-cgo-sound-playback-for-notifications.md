# 641 — Research: pure-Go (no CGo) sound playback for notifications

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: [[640-stop-sound-hook-player-fallback-chain-or-go-native-playback-never-blocking]]

---

/goal Find out whether harnez can play a short notification sound in pure Go (no CGo) on Linux
and macOS, and report options with effort and trade-offs; do not build it. Stop and report if
no viable path exists.

## Questions
- Linux: is there a maintained pure-Go PulseAudio client (native protocol over the socket, which
  PipeWire also serves)? How much code would a minimal "play one buffer" client be?
- `oto`: confirm it needs CGo for ALSA on Linux and uses purego (no CGo) on macOS.
- Decoding: embed a WAV (no decoder needed) vs `.oga` (needs a pure-Go Vorbis decoder).
- Binary size and cross-compile impact.

Outcome: a short findings section here, and a decision whether to replace 640's player chain.
