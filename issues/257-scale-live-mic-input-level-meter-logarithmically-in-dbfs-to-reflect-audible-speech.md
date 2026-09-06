# 257 — Scale Live Mic Input Level Meter Logarithmically in dBFS to Reflect Audible Speech

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: UX / Agentic Ergonomics
**Related**: [[245-show-live-mic-input-signal-level-peak-rms-alongside-configured-volume-in-usage-watch]], [[244-show-system-mic-level-and-recording-on-off-as-new-usage-watch-tui-box]], `internal/usage/miclive.go`

---

## 1. Problem & Motivation

Ticket 245 introduced a live in-memory peak/RMS signal meter in `harnez usage --watch` (`internal/usage/miclive.go`) to display whether sound is reaching the microphone.

However, during normal speech dictation (e.g., using voice tools like `voxi` or speaking at regular conversational volume), the live indicator in `harnez usage --watch` stays pinned at **1–2% max** (and rarely exceeds 5–8% on loud peaks), giving the misleading impression that the microphone is barely registering sound or failing.

This mirrors the exact UX limitation found in GNOME Settings' input level meter, where users observe a practically empty meter despite clear transcription and normal recording.

---

## 2. Technical Findings & Root Cause

In `internal/usage/miclive.go`, `micLiveAmplitudeFromPCM16LE` calculates the 0–100 level as a raw linear ratio against int16 full-scale:

```go
rms := math.Sqrt(sumSq / float64(n))
level := rms / 32768 * 100
```

### Why Linear Full-Scale Fails for Audio Meters:
1. **Digital Headroom & Conversational Speech Levels**:
   - Digital full-scale ($0\text{ dBFS}$) corresponds to raw int16 maximum ($32,768$).
   - Standard speech recorded with typical ADC/hardware gain headroom sits around $-45\text{ dBFS}$ to $-20\text{ dBFS}$.
   - Telemetry from real recording sessions confirms:
     - Conversational speech mean RMS: $\approx 150 - 600$ ($\approx -46\text{ dBFS}$ to $-35\text{ dBFS}$).
     - Peak RMS on loud syllables: $\approx 2,500 - 2,750$ ($\approx -22\text{ dBFS}$ to $-21.5\text{ dBFS}$).
2. **Linear Math Compression**:
   - $\text{RMS } 180 / 32,768 \times 100 = \mathbf{0.55\%}$
   - $\text{RMS } 500 / 32,768 \times 100 = \mathbf{1.52\%}$
   - $\text{Peak RMS } 2,700 / 32,768 \times 100 = \mathbf{8.24\%}$

Because human hearing is logarithmic and audio metering standard practice (e.g., VU meters in `pavucontrol`, OBS, DAWs) maps decibels across a range such as $-60\text{ dBFS}$ to $0\text{ dBFS}$, a linear scaling squashes virtually all audible voice activity into the bottom $1 - 2\%$ of the visual UI bar.

---

## 3. Desired Behavior & Proposed Solution

Update `micLiveAmplitudeFromPCM16LE` (or provide a calibrated mapping function) to scale the RMS level across a perceptual logarithmic/dBFS dynamic range before mapping to the 0–100 TUI bar.

### Suggested dBFS Scaling:
Map RMS dBFS from a floor of $-60\text{ dBFS}$ (or $-50\text{ dBFS}$) to $0\text{ dBFS}$:
$$\text{dBFS} = 20 \times \log_{10}\left(\frac{\text{RMS}}{32768}\right)$$
$$\text{Level}_{0..100} = \operatorname{clamp}\left(\frac{\text{dBFS} - \text{dBFS}_{\text{min}}}{0 - \text{dBFS}_{\text{min}}} \times 100,\; 0,\; 100\right)$$

With $\text{dBFS}_{\text{min}} = -60\text{ dBFS}$:
- Ambient silence (RMS $< 32$, $\le -60\text{ dBFS}$) $\to 0\%$
- Quiet speech (RMS $\approx 180$, $-45\text{ dBFS}$) $\to \approx 25\%$
- Normal speech (RMS $\approx 550$, $-35\text{ dBFS}$) $\to \approx 42\%$
- Strong speech / peaks (RMS $\approx 2,700$, $-22\text{ dBFS}$) $\to \approx 63\%$
- Near clipping (RMS $\approx 20,000$, $-4\text{ dBFS}$) $\to \approx 93\%$

This provides an intuitive, responsive VU meter in the TUI box that reflects active voice presence without altering raw underlying capture logic.

---

## 4. Verification Plan

1. **Unit tests in `internal/usage/mic_test.go`**:
   - Verify silence buffer returns $0\%$.
   - Verify conversational test vectors (RMS $\sim 200 - 1000$) produce reasonable UI levels ($25\% - 50\%$).
   - Verify full-scale square/sine waves clamp cleanly at $100\%$.
2. **Interactive TUI verification**:
   - Run `harnez usage --watch --mic` while speaking at normal volume and verify the live level bar dynamically bounces between $20\%$ and $60\%$.
