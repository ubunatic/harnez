# Extract Standalone Voice Input Engine (`ubunatic/voxi`)

- **Status:** Completed
- **Related Issues:** 020 (Voice Input), 021 (Fluent Streaming), 022 (Transcriber UI), 026 (Continuous Eager Streaming), 027 (Turn-Taking), 028 (LLM Cleanup)

## 1. Context & Motivation

Through issues 020–028, Harnez developed a complete, high-performance voice-input stack for Linux/Wayland:
- Continuous eager sentence streaming with rolling Whisper inference and zero pause drops.
- A physical modifier monitoring daemon (`harnez-modifierd` → `voxi-modifierd`) with sub-10ns release gating.
- Direct synthetic keystroke injection via `dotoolc` and atomic clipboard fallback.
- An interactive btop-styled TUI resource and latency monitor (`voxi monitor`).
- A GNOME Shell companion extension (`voxi@ubunatic.com`) and desktop focus coordinator.

### Architectural Resolution
The entire voice input and desktop typing stack was extracted into the dedicated standalone repository: **`ubunatic/voxi`** (`voxi`). Harnez returns to its lean harness-management focus, delegating to `voxi` for agent dictation workflows.

---

## 2. Architecture (`ubunatic/voxi`)

```
                      ┌───────────────────────────────────────────────┐
                      │              ubunatic/voxi                    │
                      │  (Standalone Linux Voice & Input Engine)      │
                      └──────────────────────┬────────────────────────┘
                                             │
      ┌──────────────────────┬───────────────┴──────────────┬──────────────────────┐
      ▼                      ▼                              ▼                      ▼
┌─────────────┐      ┌───────────────┐              ┌───────────────┐      ┌─────────────┐
│    voxi     │      │ voxi-modifierd│              │  GNOME Ext    │      │  Systemd    │
│  (Main CLI) │      │(System Daemon)│              │  (Desktop UI) │      │   Units     │
├─────────────┤      ├───────────────┤              ├───────────────┤      ├─────────────┤
│• eager      │      │• evdev reader │              │• Top bar icon │      │• voxi-eager │
│• record     │      │• /run/voxi/   │              │• Focus router │      │• voxi-mod-d │
│• monitor    │      │  modifiers    │              │• Speed slider │      │             │
│• history    │      │• 0 keylog risk│              │• History popup│      │             │
└─────────────┘      └───────────────┘              └───────────────┘      └─────────────┘
      ▲
      │ (Invoked / delegated by)
┌─────┴───────────────────────────────────────────────────────┐
│                        harnez                               │
│  (Harness Sync, AGENTS.md, Claude/Prime Config, Token Quotas│
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Work Breakdown & Completed Phases

### Phase 1: Repository Scaffolding & Domain Setup
- [x] Initialized `ubunatic/voxi` (Go module `ubunatic.com/voxi`, Cobra CLI, Make scaffolding).
- [x] Established repository conventions (`AGENTS.md`, `CLAUDE.md`, docs pipeline).
- [x] Initialized landing page in `website/index.html` and integrated into `ubunatic.com`.

### Phase 2: Core Engine & Subsystem Migration
- [x] **Audio Capture & DSP**: Migrated `AudioSegmenter`, PipeWire capture ring buffers, and VAD pause detector into `internal/audio`.
- [x] **ASR Inference Engine**: Migrated Whisper cleanup, URL/hallucination filtering into `internal/asr`.
- [x] **Modifier Daemon (`voxi-modifierd`)**:
  - Migrated evdev `EVIOCGKEY` monitoring daemon into `internal/modifiers` and `cmd/voxi-modifierd`.
  - Standardized state path to `/run/voxi/modifiers` (`0644`).
  - Packaged dedicated systemd unit `voxi-modifierd.service` (`DeviceAllow=char-input r`).
- [x] **Synthetic Typing & Gating**:
  - Migrated `TypeText`, `BuildDotoolCommands`, and `WaitModifiersReleased` into `internal/typing`.
  - Migrated atomic clipboard fallback (`wl-copy` + paste).

### Phase 3: CLI, TUI Monitor & Desktop Companion
- [x] **CLI Surface**:
  ```text
  voxi mode [eager|batch|streaming]
  voxi record [start|stop|toggle|status]
  voxi eager [--threshold] [--silence] [--pre-roll] [--daemon]
  voxi monitor / voxi top [--watch]
  voxi history [list|copy|retype|clear|record]
  voxi config [get|set] <KEY> <VAL>
  ```
- [x] **Btop-Style Monitor**:
  - Migrated interactive dashboard with `RuneDisplayWidth`/`StringDisplayWidth` math into `internal/monitor`.
  - Live CPU/GPU sparklines, AMD VRAM sysfs parser, and transcription latency metrics.
- [x] **GNOME Extension**:
  - Moved `contrib/gnome-shell-extension` to `voxi/contrib/gnome-shell-extension`.
  - Re-branded UUID to `voxi@ubunatic.com`.

### Phase 4: Harnez Decoupling & Thin Integration
- [x] Removed `internal/tools/voice_*.go`, `internal/tools/modifier_*.go`, and `cmd/harnez-modifierd` from `harnez`.
- [x] Updated `harnez tools voice-input` to act as a lightweight wrapper delegating to `voxi`.
- [x] Updated evergreen documentation (`docs/VoiceInput.md`, `docs/VoiceInputArchitecture.md`) to reference `voxi` as an external tool dependency.
- [x] Retained token quota tracking (`harnez usage`) and harness synchronization (`harnez apply`/`init`) in `harnez`.

### Phase 5: Packaging & Verification
- [x] Created `Makefile` recipes in `voxi` (`make install`, `make install-modifierd`, `make install-user-services`).
- [x] Built and verified all unit tests across both `voxi` and `harnez`.
- [x] Synced `voxi` website into `ubunatic.com` via `uman`.
