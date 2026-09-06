# 258 — Dynamic Refresh Rate for Live Mic Meter: High-Frequency UI Redraw on Speech Activity

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate (1 Hz UI redraw rate prevents live mic level bar from functioning as a responsive VU meter)
**Category**: UX / Performance / Agentic Ergonomics
**Related**: [[245-show-live-mic-input-signal-level-peak-rms-alongside-configured-volume-in-usage-watch]],
[[257-scale-live-mic-input-level-meter-logarithmically-in-dbfs-to-reflect-audible-speech]],
`internal/usage/watch.go`, `internal/usage/miclive.go`

---

## 1. Problem & Motivation

Tickets 245 and 257 added a live microphone PCM stream and calibrated logarithmic dBFS scaling to `harnez usage --watch`.

However, the TUI dashboard's redraw cadence in `internal/usage/watch.go` is driven primarily by `loadTicker := time.NewTicker(time.Second)` (1 Hz). As a result:
1. Even though `micLiveManager` captures audio chunks in memory every ~200ms, the screen only paints **once per second**.
2. When speaking, the live level bar only updates on 1-second boundaries, making the meter feel sluggish, choppy, and unresponsive rather than acting as a real-time audio VU meter.
3. Fast syllables and transient voice peaks are often missed completely between 1-second ticks.

The user requested that when speech/sound activity occurs, the refresh rate should dynamically bump up to provide a smooth, real-time meter experience, while preserving low CPU utilization during silence or when the Mic panel is inactive.

---

## 2. Technical Design & Architecture

### 2.1 Adaptive High-Frequency Redraw Loop
- In `internal/usage/watch.go`:
  - When the Mic panel is active (`activeSec.Mic && currentHost == ""`), connect a callback from `micLiveManager` (or an active audio ticker) into the event loop.
  - When incoming audio level is above the noise floor ($> 0\%$, speech detected), bump the redraw cadence to **10–20 Hz** (e.g. 50ms – 100ms per frame).
  - When speech ends (level returns to $0\%$), maintain high refresh rate for a short "grace period" (e.g. 500ms – 1s) and then throttle back to the baseline 1 Hz `loadTicker` cadence.

### 2.2 Ballistics & Decay (Fast Attack, Smooth Release)
- Standard audio VU meter ballistics:
  - **Instant attack**: Bar immediately jumps to the peak level of the current chunk.
  - **Smooth decay**: Instead of snapping instantly to zero between words, apply smooth exponential decay (e.g. falling by 5–10% per 50ms frame) so syllables connect naturally.

### 2.3 CPU & Battery Invariants
- When the Mic panel is toggled off, or during prolonged ambient silence, the high-frequency redraw loop must remain dormant (0% additional CPU overhead).

---

## 3. Scope of Implementation

1. **`internal/usage/miclive.go`**:
   - Provide a notification hook or stream channel when fresh audio amplitude samples are computed.
   - Implement smooth decay ballistics if computed in-engine.
2. **`internal/usage/watch.go`**:
   - Wire dynamic redraw triggers into `RunWatchWithOptions` / `draw()`.
   - Implement speech-activated fast refresh rate with idle fallback.
3. **`internal/usage/watch_test.go` & `miclive_test.go`**:
   - Unit tests for dynamic refresh rate state transitions and ballistics decay math.

---

## 4. Acceptance Criteria

- [ ] While speaking with the Mic panel open in `harnez usage --watch`, the live level bar moves smoothly and responsively at 10+ FPS.
- [ ] During silence or when the Mic panel is closed, the redraw loop throttles back to 1 Hz without wasting CPU.
- [ ] Level transitions have fast attack and smooth visual decay.
- [ ] `make check` passes cleanly.

---

## 5. Verification

- **Automated**: Unit tests covering decay mathematics and timer cadence switches in `internal/usage`.
- **Manual Verification**: Run `harnez usage --watch --mic`; speak into the microphone and observe smooth real-time bar movement; stop speaking and verify the TUI returns to idle 1 Hz refresh.

