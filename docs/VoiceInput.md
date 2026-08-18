# Voice Input (`voxi`)

> [!NOTE]
> Following architectural extraction (issue 029), the standalone voice input engine,
> continuous eager streaming orchestrator, modifier daemon, and GNOME companion extension
> have moved to the dedicated repository: **[`ubunatic/voxi`](https://github.com/ubunatic/voxi)** (`voxi`).
> `harnez tools voice-input` remains available as a lightweight wrapper delegating to `voxi`.

`harnez tools` is an explicit boundary for optional operating-system capabilities. It is
separate from global harness `apply` and project `init`; neither installs packages or
services. For the multi-tier desktop companion, Mutter focus coordination, and Wayland
input architecture, see [VoiceInputArchitecture.md](VoiceInputArchitecture.md).

## Quick Start with `voxi`

Install the standalone `voxi` CLI and services:

```bash
# Install voxi binaries to ~/go/bin
go install ubunatic.com/voxi/cmd/voxi@latest
go install ubunatic.com/voxi/cmd/voxi-modifierd@latest

# Or build from source in the workspace
cd ~/projects/voxi && make install
```

## Commands

`harnez tools voice-input` delegates directly to `voxi`:

```text
voxi mode                     # show active mode (batch/streaming/eager/neither)
voxi mode eager               # switch to continuous eager sentence streaming
voxi mode streaming           # switch to opt-in local streaming
voxi mode batch               # switch back to default batch flow
voxi record status            # show active dictation status (idle or recording)
voxi record toggle            # universal toggle across active mode
voxi eager                    # run continuous eager sentence streaming dictation
voxi monitor --watch          # live btop-style resource & latency monitor
voxi history list             # list recent dictations (most recent first, sensitive)
voxi history copy <ID>        # copy transcript to clipboard via wl-copy
voxi history retype <ID>      # re-type transcript at cursor via dotool
voxi history clear            # wipe local history file
voxi config get type-delay-ms # read type_delay_ms from config
voxi config set type-delay-ms # edit type_delay_ms preserving comments
```

## Architecture & Separation

`voxi` manages:
- **Audio DSP & VAD**: 16kHz mono PipeWire capture with rolling circular pre-roll buffer.
- **ASR Inference**: Rolling Whisper model inference and local Parakeet ONNX streaming.
- **Modifier Gating**: Physical modifier daemon (`voxi-modifierd`) monitoring evdev keystrokes with zero non-modifier keylogging.
- **Desktop Companion**: GNOME Shell Extension (`voxi@ubunatic.com`) for desktop focus restoration, speed slider, and status indicator.
- **TUI Monitor**: Btop-style terminal resource dashboard (`voxi monitor`).

`harnez` retains its core domain focus:
- Declarative Claude Code and Prime Agent harness configuration (`harnez apply`, `harnez init`).
- Multi-agent token and session quota tracking (`harnez usage`).
