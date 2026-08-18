# Extract Standalone Voice Input Engine (`ubunatic/voxi`)

- **Status:** Proposed / Architectural Extraction Plan
- **Related Issues:** 020 (Voice Input), 021 (Fluent Streaming), 022 (Transcriber UI), 026 (Continuous Eager Streaming), 027 (Turn-Taking), 028 (LLM Cleanup)

## 1. Context & Motivation

Through issues 020–028, Harnez developed a complete, high-performance voice-input stack for Linux/Wayland:
- Continuous eager sentence streaming with rolling Whisper inference and zero pause drops.
- A physical modifier monitoring daemon (`harnez-modifierd`) with sub-10ns release gating.
- Direct synthetic keystroke injection via `dotoolc` and atomic clipboard fallback.
- An interactive btop-styled TUI resource and latency monitor.
- A GNOME Shell companion extension and desktop focus coordinator.

### The Architectural Conflict
As analyzed in our architectural retrospective, this stack now comprises kernel input drivers, system-scope root daemons, audio DSP pipelines, and GPU Vulkan monitors. This creates significant domain drift from Harnez's core mission (declarative Claude/Prime agent harness configuration).

**Objective:** Extract the entire voice input engine into a dedicated, standalone repository and binary: **`ubunatic/voxi`** (`voxi`). Harnez will return to its lean harness-management focus and simply configure and orchestrate `voxi` for agent workflows.

---

## 2. Target Architecture (`ubunatic/voxi`)

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
│• eager      │      │• evdev reader │              │• Top bar icon │      │• voxi.service│
│• record     │      │• /run/voxi/   │              │• Focus router │      │• voxi-eager │
│• monitor    │      │  modifiers    │              │• Speed slider │      │• voxi-mod-d │
│• history    │      │• 0 keylog risk│              │• History popup│      │             │
└─────────────┘      └───────────────┘              └───────────────┘      └─────────────┘
      ▲
      │ (Invoked / configured by)
┌─────┴───────────────────────────────────────────────────────┐
│                        harnez                               │
│  (Harness Sync, AGENTS.md, Claude/Prime Config, Token Quotas│
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Work Breakdown & Phased Execution Plan

### Phase 1: Repository Scaffolding & Domain Setup
- [ ] Initialize `ubunatic/voxi` (Go module `ubunatic.com/voxi`, Cobra CLI, Make scaffolding).
- [ ] Establish repository conventions (`AGENTS.md`, `CLAUDE.md`, docs pipeline).
- [ ] Define standalone YAML specifications for audio pipelines and driver chains.

### Phase 2: Core Engine & Subsystem Migration
- [ ] **Audio Capture & DSP**: Migrate `AudioSegmenter`, PipeWire capture ring buffers, and VAD pause detector.
- [ ] **ASR Inference Engine**: Migrate Vulkan Whisper worker and Parakeet ONNX streaming runner.
- [ ] **Modifier Daemon (`voxi-modifierd`)**:
  - Migrate evdev `EVIOCGKEY` monitoring daemon.
  - Standardize state path to `/run/voxi/modifiers` (`0644`).
  - Package dedicated systemd unit `voxi-modifierd.service` (`DeviceAllow=char-input r`).
- [ ] **Synthetic Typing & Gating**:
  - Migrate `TypeText`, `BuildDotoolCommands`, and `WaitModifiersReleased`.
  - Migrate atomic clipboard fallback (`wl-copy` + paste).

### Phase 3: CLI, TUI Monitor & Desktop Companion
- [ ] **CLI Surface**:
  ```text
  voxi mode [eager|batch|streaming]
  voxi record [start|stop|toggle|status]
  voxi top / voxi monitor [--watch]
  voxi history [list|copy|retype|clear]
  voxi config [get|set] <KEY> <VAL>
  ```
- [ ] **Btop-Style Monitor**:
  - Migrate interactive dashboard with `RuneDisplayWidth`/`StringDisplayWidth` math.
  - Live CPU/GPU sparklines, AMD VRAM sysfs parser, and transcription latency metrics.
- [ ] **GNOME Extension**:
  - Move `contrib/gnome-shell-extension` to `voxi/contrib/gnome-shell-extension`.
  - Re-brand UUID to `voxi@ubunatic.com`.

### Phase 4: Harnez Decoupling & Thin Integration
- [ ] Remove `internal/tools/voice_*.go` and `cmd/harnez-modifierd` from `harnez`.
- [ ] Update `harnez tools voice-input` to act as a lightweight wrapper/installer for `voxi`.
- [ ] Update evergreen documentation (`docs/VoiceInput.md`, `docs/VoiceInputArchitecture.md`) to reference `voxi` as an external tool dependency.
- [ ] Retain token quota tracking (`harnez usage`) and harness synchronization (`harnez apply`/`init`) in `harnez`.

### Phase 5: Packaging & Distribution
- [ ] Add `goreleaser` configuration for `voxi` and `voxi-modifierd` binary releases.
- [ ] Create systemd install recipes (`sudo make install-system` / user units).
- [ ] Provide unified Fedora/Arch setup scripts.

---

## 4. Success Criteria

1. **Clean Separation**: `harnez` binary drops all audio DSP, evdev ioctl, and GPU monitoring code; binary size decreases and domain focus is restored.
2. **Zero Feature Regression**: `voxi` provides identical sub-250ms eager streaming latency, modifier gating safety, and TUI visualization.
3. **Independent Release Cycles**: `voxi` can be versioned and released as a general-purpose Linux voice input tool independent of AI agent harness tooling.
