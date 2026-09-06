# 260 — Live Mic Meter: Equalizer-Style Smooth Falloff / Visual Decay for Fluid Voice Dynamics

**Status**: Open — filed via /issue
**Priority**: P2 (Medium)
**Severity**: Minor (visual polish: eliminates jerky/strobe bar movement while retaining zero-lag onset)
**Category**: UX / Agentic Ergonomics
**Related**: [[258-dynamic-refresh-rate-for-live-mic-meter-high-frequency-ui-redraw-on-speech-activity]],
[[257-scale-live-mic-input-level-meter-logarithmically-in-dbfs-to-reflect-audible-speech]],
[[259-decouple-hardware-load-timeline-sampling-from-high-fps-tui-redraw-cadence]],
`internal/usage/miclive.go`, `spec/indicators.yaml`

---

## 1. Problem & Motivation

The rolling 100ms `max` aggregation and low-latency audio flags resolved previous buffer lag: voice onsets
now register immediately in `harnez usage --watch`.

However, without visual release ballistics, the level bar collapses abruptly to zero the moment sound pauses
between syllables. This makes the live bar feel jumpy, nervous, and strobe-like rather than fluid and legible.

Like professional equalizer displays, spectrum visualizers, and DAW VU meters, an intuitive audio level meter
combines **instantaneous attack** (immediate rise to peaks) with a **smooth, fast visual falloff/decay** (releasing
smoothly over ~150–250ms) so speech rhythms feel natural, fluid, and continuous.

---

## 2. Technical Design & Architecture

### 2.1 Post-Aggregation Equalizer Falloff Ballistics
In `internal/usage/miclive.go` `micLiveMeter.update`:
1. Calculate the raw window-aggregated target level (e.g. `max` of samples across `window-seconds`, e.g. 100ms).
2. Apply time-decay ballistics against the previous displayed level:
   - **Instant Rise**: If $\text{targetLevel} \ge \text{prevLevel}$, displayed level immediately snaps to $\text{targetLevel}$.
   - **Smooth Visual Decay**: If $\text{targetLevel} < \text{prevLevel}$, displayed level falls exponentially based on elapsed time:
     $$\text{decayedLevel} = \text{prevLevel} \times \exp\left(-\frac{\Delta t}{\tau}\right)$$
     (where $\tau \approx 100 - 150\text{ms}$ yields a clean ~200ms visual release half-life).
   - **Clean Floor Snap**: When decayed level drops below $0.5\%$, snap cleanly to $0.0\%$.

### 2.2 Spec-Driven Falloff Parameters
In `spec/indicators.yaml` / `spec/schemas/indicators.schema.json`, expose optional decay tuning under `mic-live`:
- `decay-ms`: visual decay half-life / release duration in milliseconds (default: `150` or `200ms`, `0` for raw instantaneous snap).

---

## 3. Scope of Implementation

1. **`internal/usage/miclive.go`**:
   - Implement time-elapsed exponential decay on top of the windowed metric in `micLiveMeter.update`.
2. **`spec/indicators.yaml` & `spec/schemas/indicators.schema.json`**:
   - Add `decay-ms` parameter to `mic-live` configuration.
3. **`internal/usage/indicatorsspec.go`**:
   - Expose `DecayDuration()` from `micLiveSpec`.
4. **`internal/usage/miclive_test.go`**:
   - Add unit tests verifying instant attack on rising levels, smooth exponential falloff across consecutive time steps, and clean snapping to zero at silence.

---

## 4. Acceptance Criteria

- [ ] Voice onsets jump instantly to peak level with zero perceptible lag.
- [ ] During speech pauses and between syllables, the bar falls off smoothly over ~150–250ms rather than flashing abruptly to zero.
- [ ] Prolonged silence settles cleanly to 0%.
- [ ] `decay-ms` is spec-driven with sensible defaults.
- [ ] All existing and new tests pass (`make check`).

---

## 5. Verification

- **Automated**: `go test -v ./internal/usage/...` testing ballistics release curves over discrete time increments.
- **Manual Verification**: Run `harnez usage --watch --mic`; speak sentences and observe smooth, fluid equalizer-like bar motion without jumpiness or lag.

