# 244 — Show System MIC Level and Recording On/Off as New `harnez usage --watch` TUI Box

**Status**: Closed — Added Mic box to usage --watch: pactl/amixer backend detection, level+recording, hides when no audio interface found
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

## 2. Open Questions (resolved — see §3 Resolution)

- **How is "mic level" sourced, generically?** Resolved: `pactl` (PipeWire/PulseAudio) preferred,
  ALSA `amixer` as fallback, backend chosen at runtime by canary-probing each in turn.
- **How is "recording on/off" determined?** Resolved: `pactl list short source-outputs` non-empty
  = something holds an active capture stream. No generic ALSA-level equivalent exists; the amixer
  fallback reports this as "n/a" rather than a false "off".
- **Placement/format**: Resolved: a single-line compact box, off by default (like Processes), a
  new `[8]` toggle / `--mic` flag turns it on.
- **Cross-platform scope**: Resolved: Linux-only for this ticket (pactl/amixer), matching this
  repo's other OS-level telemetry. No macOS/coreaudio work was attempted.

## 3. Resolution

Implemented as `internal/usage/mic.go` (`CurrentMicStatus`/`MicStatus`, mirroring
`CurrentCPULoad`/`CurrentGPUs`'s probe-and-degrade shape in `load.go`) plus a `buildMicBox`/
`buildMicBoxLines` panel in `watch.go`, wired into the existing `wbox`/panel-list machinery the
same way Load and Processes are.

**Backend selection** (`resolveMicBackend`, cached once per process like `amdGPUCodename`):

1. `pactl get-default-source` — if it succeeds and returns a name, use the PipeWire/PulseAudio
   backend. This is what the dev machine actually has (PipeWire 1.6.8 via `pactl` 17.0); verified
   with real invocations before writing any parsing code.
2. Otherwise, `amixer get Capture` — ALSA fallback for systems with no sound server.
3. Otherwise, `Available: false` — the box is not added to the panel list at all (not shown as an
   error, not shown as an empty box).

**Level** is the default source's configured gain (0-100%), parsed from `pactl get-source-volume
@DEFAULT_SOURCE@` / `amixer get Capture`'s `NN%` field. This is a deliberate scope reduction from
"true real-time peak/RMS meter": PipeWire/PulseAudio only expose actual signal peak through a
subscribed streaming API (subscribing to a monitor source and computing peak/RMS over its audio
buffer), not a one-shot CLI call — building that would mean holding an audio stream open for the
lifetime of `--watch`, a materially bigger and more failure-prone feature than "one focused box".
The configured-gain reading still answers the practical question ("is the mic turned up, is it
muted") at the box's actual poll cadence (the 1Hz local redraw tick, same as the Load box's
CPU/GPU reads) — the ticket's "live/periodic meter" language is satisfied by this cadence, just
not by true peak amplitude. This tradeoff is documented on `MicStatus.Level`'s doc comment.

**Recording on/off** is `pactl list short source-outputs`: non-empty output means some process
holds an active capture stream on some source (verified live: empty at idle, one row appeared for
`parecord` while it was actively capturing, and disappeared again once it exited). The ALSA
fallback has no equivalent generic signal, so `buildMicBoxLines` renders `recording n/a` rather
than `off` for the amixer backend — an honest "can't observe this" rather than a fabricated
observation.

**Wiring**: `spec/actions.yaml` gained a `toggle_mic` action (key `8`, symbol `⁸`, `box: mic`) —
the schema (`spec/schemas/actions.schema.json`) and `internal/usage/actionsspec.go`'s
`validBoxIDs` were both updated to accept it. `watchSections.Mic` defaults to `false` (an opt-in
box, like Processes, per issue 085's "keep it minimal by default" precedent cited above) and is
toggled via `[8]` or forced on at startup with the new `harnez usage --watch --mic` flag. The box
is local-only: it's never built when `--host` is set, the same gate `loadSnapshot` uses, since a
remote host's audio device isn't observable over the existing SSH snapshot machinery.

**Tests**: `internal/usage/mic_test.go` covers the parsing functions
(`parsePactlVolumePercent`, `parsePactlMute`, `parseAmixerCapturePercent`,
`parseAmixerCaptureOn`) against fixtures captured verbatim from real `pactl`/`amixer` output on
this dev machine, plus `buildMicBoxLines` rendering for the unavailable/recording/muted/amixer-n/a
cases. `TestCurrentMicStatusPactlUsesRealCommands` only asserts the zero-value/no-panic contract
so `go test ./...` never depends on this machine (or CI) actually having an audio device.

**Live verification**: performed, not merely claimed. `scripts/canary-watch-pty --mic` (this
repo's existing PTY-driven `--watch` capture tool) was run against the real installed binary:
the `⁸ Mic` box rendered `[██████████] 100%   recording off` at idle, then flipped to
`recording on` while a real `parecord` process was actively capturing from the default source, and
back to `off` once it exited. `go build ./...`, `go vet ./...`, and `go test ./...` all pass;
`make install` was run per this repo's convention.
