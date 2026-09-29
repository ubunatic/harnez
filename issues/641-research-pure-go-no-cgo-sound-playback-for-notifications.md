# 641 — Research: pure-Go (no CGo) sound playback for notifications

**Status**: Closed — Decision: keep the 640 player chain; see docs/studies/PureGoSound.md
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

## Findings

Full study and comparison matrix: See `@docs/studies/PureGoSound.md`.

- **Linux Pure-Go PulseAudio**: `github.com/jfreymuth/pulse` is actively maintained and implements native PA protocol over UNIX domain sockets in pure Go. A bespoke in-tree client would require ~800–1,200 LOC.
- **`oto` CGo Status**: `ebitengine/oto` v3 is CGo-free across Linux and macOS (uses `jfreymuth/pulse` and `purego` for ALSA/AudioToolbox dynamic calls), but introduces heavy game-engine architecture (~1–1.4 MB binary overhead).
- **Decoding Economics**: Embedding uncompressed 22.05 kHz 16-bit mono WAV (~44 KB) is net smaller than Ogg Vorbis + pure-Go Vorbis decoder (~170–230 KB).
- **Recommendation / Decision**: **Keep the Issue 640 external player fallback chain**. It provides zero binary bloat (0 KB), zero dependencies, clean headless/CI bypass via `LookPath`, and guaranteed crash/hang isolation via process-group `SIGKILL` on timeout.

Host note: the source research (Gemini Deep Research) cites oto release notes and pkg.go.dev
for `oto` v3 needing no CGo on Linux (purego + `jfreymuth/pulse`); treat that as sourced. Its
hybrid recommendation (pure-Go pulse client before the player chain on Linux) was considered and
declined by the user on 2026-09-30: the gap it closes (daemon present, no player CLI) is rare and
does not justify ~400 KB and a new dependency.
