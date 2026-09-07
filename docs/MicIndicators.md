# Mic Indicators — Linux Desktop Microphone Privacy Indicators and Live Input Signal Meters

This document surveys how major Linux desktop environments implement the
microphone-in-use privacy indicator and live input-signal meters,
which properties each indicator reads, which streams are exempted, and
what portable strategy harnez can use for its own live meter.

Evidence quality is marked throughout:

- **[CONFIRMED]** — directly read from upstream source code or live canary
- **[INFERRED]** — reasoning from API documentation or closely related evidence
- **[UNCONFIRMED]** — plausible hypothesis not yet validated on a live system

---

## 1. Glossary

| Term | Definition |
|---|---|
| **source-output** | PulseAudio term for an active capture client consuming frames from a source; surfaced by `pactl list source-outputs` |
| **application.id** | PA/PW property: reverse-DNS app identifier set by the client (e.g. `org.gnome.VolumeControl`) |
| **application.name** | PA/PW property: human-readable client name as registered with the server |
| **media.role** | PA/PW property: stream's purpose classification (`music`, `video`, `phone`, `game`, `test`, `production`, `a11y`, etc.) |
| **media.name** | PA/PW property: stream's human-readable name as assigned by the client (e.g. `"Peak detect"`) |
| **resample.peaks** | PipeWire-native property: instead of delivering raw PCM, the server computes and returns per-period peak amplitude values; used by gvc/libgnome-volume-control for level metering |
| **node.passive** | PipeWire property: the session manager creates passive links from this node and it does not keep a sink/source busy or running on its own |
| **node.virtual** | PipeWire property: marks a node as a virtual (software-synthesised) stream; KDE uses this to exclude streams from the mic indicator |
| **PA_STREAM_PEAK_DETECT** | PulseAudio C-API flag: create a source output that yields only one float peak per period; maps to `resample.peaks` in PipeWire's PulseAudio-compat layer |
| **portal session** | xdg-desktop-portal Camera or Microphone portal session: grants sandboxed (Flatpak/Snap/browser) access to capture; separate from unsandboxed PA/PW streams |
| **pipewire-pulse** | PipeWire's PulseAudio-compatibility library/daemon; `pactl` and `parec` talk to PipeWire via this layer; most PA properties still work |
| **GVC / libgnome-volume-control** | GNOME's shared audio control library (`gvc`): used by gnome-shell and gnome-control-center |

---

## 2. GNOME

### 2.1 Indicator mechanism — `js/ui/status/volume.js` [CONFIRMED]

GNOME Shell's microphone-in-use indicator is implemented in
[`js/ui/status/volume.js`](https://gitlab.gnome.org/GNOME/gnome-shell/-/blob/main/js/ui/status/volume.js),
class `InputStreamSlider`, method `_maybeShowInput()` (lines 416–435 in the
`main` branch as of 2026-09-05).

The exact predicate that controls whether the top-bar indicator appears:

```js
// skip gnome-volume-control and pavucontrol which appear
// as recording because they show the input level
const skippedApps = [
    'org.gnome.VolumeControl',
    'org.PulseAudio.pavucontrol',
];

showInput =
    this._control.get_sources().some(source =>
        source.state === Gvc.MixerStreamState.RUNNING) &&
    this._control.get_source_outputs().some(output =>
        !skippedApps.includes(output.get_application_id()));
```

What this means:

- The indicator reads `application.id` on every source-output returned by
  the GVC mixer control (which queries `pipewire-pulse`).
- Any stream whose `application.id` is `org.gnome.VolumeControl` or
  `org.PulseAudio.pavucontrol` is silently excluded.
- The indicator also requires at least one source to be in the RUNNING state
  (i.e. a device is actually active), so transient events before a device
  is awake are suppressed.
- There is **no** filtering on `media.role`, `media.name`, `node.passive`,
  or `node.virtual` — only `application.id` matters.

The indicator is part of `InputIndicator` / `InputStreamSlider`. There is no
separate C process watching PipeWire node states for the microphone (unlike
the camera, which has `shell-camera-monitor.c` watching `media.role=Camera`
node running states).

`InputIndicator._updatePrivacyIndicator()` adds or removes the CSS class
`privacy-indicator` from the top-bar icon based on whether the default source
is muted. The indicator appearance (orange dot) is CSS-driven once
`InputStreamSlider` marks itself visible.

### 2.2 Live input signal meter — GVC peak-detect stream [CONFIRMED]

gnome-control-center's Sound panel (via `libgnome-volume-control`) opens a
`PA_STREAM_PEAK_DETECT` stream via the PulseAudio C API. This is a real
`pa_source_output` that appears in `pactl list source-outputs`, but carries:

- `application.id = org.gnome.VolumeControl` (exempts it from the indicator)
- `media.name = "Peak detect"` (set internally by PulseAudio's `source-output.c`)

Under PipeWire's PulseAudio compat layer, `PA_STREAM_PEAK_DETECT` maps to
`resample.peaks = true` — the server delivers one float peak value per period
instead of raw PCM. Sample format: `float32le`, ~25 Hz (one float ≈ 4 bytes
every 40 ms), mono.

### 2.3 Live canary result (issue 248, Fedora GNOME, 2026-09-05) [CONFIRMED]

A `parec` stream tagged `application.id=org.gnome.VolumeControl` left the
microphone indicator off. Adding an ordinary `parec` recorder made the
indicator turn on while the exempt stream remained active. Stopping both
turned the indicator off. This confirms the `application.id` check is the
single deciding predicate.

### 2.4 Distro variation

- Upstream GNOME Shell has no separate microphone-monitor component; `volume.js`
  is the only built-in mic indicator.
- Ubuntu GNOME patches gnome-shell for other features; no confirmed difference
  in the `skippedApps` predicate has been identified.
- Flatpak/portal-mediated capture: portal sessions do not create ordinary
  source-outputs visible to `pactl list source-outputs`. The portal itself
  does not light the volume.js indicator.

---

## 3. KDE Plasma

### 3.1 Indicator mechanism — `microphoneindicator.cpp` [CONFIRMED]

KDE Plasma's microphone-in-use indicator is implemented in
[`src/qml/microphoneindicator.cpp`](https://invent.kde.org/plasma/plasma-pa/-/blob/master/src/qml/microphoneindicator.cpp)
within `plasma-pa`, the Plasma PulseAudio/Audio applet.

The exact exemption predicate (lines 293–308):

```cpp
static const int s_virtualStreamRole =
    m_sourceOutputModel->role(QByteArrayLiteral("VirtualStream"));

for (int i = 0; i < m_sourceOutputModel->rowCount(); ++i) {
    const QModelIndex idx = m_sourceOutputModel->index(i);

    if (idx.data(s_virtualStreamRole).toBool()) {
        continue;  // skip virtual streams
    }
    indices.append(idx);
}
```

What this means:

- KDE reads the `VirtualStream` role from `PulseAudioQt::SourceOutputModel`.
- `VirtualStream` maps to the PipeWire property `node.virtual = true`.
- Any source-output marked `node.virtual = true` is excluded from the list
  that drives the system tray microphone indicator (a `KStatusNotifierItem`).
- KDE does **not** maintain a hardcoded `application.id` allowlist.
- The exclusion mechanism is portable to any stream that can set `node.virtual`.

Setting via `parec`:

```bash
parec --property=node.virtual=true ...
```

[INFERRED] — the `parec --property` flag is confirmed available (issue 248
§2.3), but whether `node.virtual` is forwarded by `pipewire-pulse` and honoured
by the KDE `SourceOutputModel` requires a live canary check on a Plasma 6 machine.

### 3.2 Live input signal meter

KDE System Settings > Audio > Input Devices includes a level meter (Plasma 6+).
The stream is opened internally by `plasma-pa` / PulseAudioQt.
[INFERRED] The stream likely carries `node.virtual = true` since the indicator's
own `recordingApplications()` would otherwise light itself up.

### 3.3 Distro variation

All standard Kubuntu / Fedora KDE / openSUSE Plasma releases ship unpatched
`plasma-pa`; upstream behaviour applies. No known distro patches to
`microphoneindicator.cpp` have been identified.

---

## 4. Cinnamon (Linux Mint)

### 4.1 Indicator mechanism [INFERRED — low confidence]

Cinnamon does **not** include a built-in system-level microphone-in-use
indicator. Secondary sources (community Q&A, Linux Mint forums through 2024)
consistently state that no mic-in-use icon appears in the Cinnamon panel when
an application opens a capture stream. The Cinnamon Sound applet
(`cs-sound-effects`) controls volume but does not monitor source-outputs for
a privacy indicator.

For harnez on Cinnamon: there is no indicator to suppress; the live meter
stream does not create a privacy UX problem on this desktop. Verification
requires a live Cinnamon session (Linux Mint 22.x) — not performed.

### 4.2 Live input signal meter

No live signal-level meter for the microphone exists in Cinnamon's Sound
applet or cinnamon-control-center. `pavucontrol` is the standard tool.
[INFERRED]

---

## 5. Xfce

### 5.1 Indicator mechanism — `xfce4-pulseaudio-plugin` [INFERRED]

Xfce's panel audio indicator is `xfce4-pulseaudio-plugin`. Version 0.4.4
(2022) added a recording indicator icon (red microphone) when any source-output
is active. Later versions (0.4.5+, 0.5.x in 2025) use Source Output Info to
reduce flicker.

- The plugin does **not** maintain an `application.id` or `node.virtual`
  allowlist — it shows the indicator for any non-empty source-output list.
- No upstream mechanism to exempt a stream has been identified from secondary
  sources or the plugin's public documentation.
- [UNCONFIRMED] whether setting `node.virtual = true` suppresses the Xfce
  recording icon, because the plugin is not known to check this property.
- Source code is at `https://gitlab.xfce.org/panel-plugins/xfce4-pulseaudio-plugin`;
  direct primary-source inspection was not completed due to connection timeouts.

Treat Xfce as a "no exemption available" environment until a live canary confirms
or denies `node.virtual` suppression.

### 5.2 Live input signal meter

No built-in live signal-level meter in the standard Xfce panel plugin.
`pavucontrol` is the standard tool.

---

## 6. COSMIC (System76 Pop!\_OS)

### 6.1 Indicator mechanism [INFERRED — very low confidence]

COSMIC (stable 2025) does **not** include a built-in microphone-in-use privacy
indicator as a core desktop feature. Community sources (Reddit through 2025) and
System76 documentation confirm this gap; community applets
(`cosmic-ext-applet-privacy-indicator`) add the feature with varying implementations.

For harnez on COSMIC: until a built-in indicator ships, the live meter stream
is unlikely to cause a privacy UX issue.

### 6.2 Live input signal meter

No built-in live input-signal meter in COSMIC's audio settings has been confirmed.
`pavucontrol` is expected to work via PipeWire.

---

## 7. Cross-cutting: xdg-desktop-portal

xdg-desktop-portal mediates **sandboxed** application microphone access
(Flatpak, snap, some browsers). It does not provide a universal mic-in-use
indicator across all desktops; each desktop provides its own indicator.

For unsandboxed apps (harnez, bare `parec`): the portal is not involved.
Unsandboxed PA/PW streams bypass the portal entirely and appear directly as
source-outputs in `pactl list source-outputs`. Portal exemption is not
applicable for harnez's architecture.

---

## 8. Decision Matrix — Strategies for harnez

| Strategy | GNOME | KDE | Cinnamon | Xfce | COSMIC |
|---|---|---|---|---|---|
| **A. Tag `application.id=org.gnome.VolumeControl`** | Exempt (confirmed) | No effect | No indicator | Likely still triggers | No indicator |
| **B. Set `node.virtual=true`** | No effect | Exempt (inferred) | No indicator | Unconfirmed | No indicator |
| **C. Both A and B simultaneously** | Exempt (A covers GNOME) | Exempt (B covers KDE) | No indicator | Unknown | No indicator |
| **D. No special tags — accept indicator** | Triggers indicator | Triggers indicator | No indicator | Triggers indicator | No indicator |

**Strategy C is the best portable approach for GNOME + KDE coverage:**
combining both properties is additive — GNOME checks `application.id`,
KDE checks `node.virtual`, and the two do not interfere with each other.

Xfce is the remaining open gap. Neither A nor B's effect on
`xfce4-pulseaudio-plugin` has been confirmed by a live canary.

---

## 9. The "Trusted App Identity" Question

Is imitating `org.gnome.VolumeControl` a stable public convention,
a private implementation detail, or a security/privacy anti-pattern?

1. **Not a public API.** GNOME's `skippedApps` is an internal comment-explained
   list. GNOME has never documented it as an extension point for third parties.

2. **Designed for system utilities only.** The list (gnome-volume-control,
   pavucontrol) consists of trusted audio management tools. harnez is not in
   that category.

3. **Fragile:** GNOME could remove or rename entries without notice. A future
   version that validates `application.id` against D-Bus registration or
   Flatpak sandboxing could break this silently.

4. **Privacy concern:** If a malicious app tags itself `org.gnome.VolumeControl`
   to hide its capture activity, the suppression is indistinguishable from
   legitimate gnome-control-center usage. harnez is not malicious, but the
   pattern normalises identity spoofing.

5. **The canary (issue 248) shows it works today** as a probe and GNOME-specific
   adapter. It is not a stable portable mechanism.

**Recommendation:** Use `org.gnome.VolumeControl` only as a GNOME-specific
adapter. Document its fragility in production commits. KDE's `node.virtual`
approach is structurally cleaner — a declared stream attribute rather than
identity impersonation.

---

## 10. Live Input Signal Meters — Summary

| Desktop | Meter location | Data source | Protocol |
|---|---|---|---|
| **GNOME** | Settings > Sound > Input | GVC peak-detect stream (`PA_STREAM_PEAK_DETECT`) | pipewire-pulse / PulseAudio |
| **KDE** | System Settings > Audio | PulseAudioQt peak stream | pipewire-pulse |
| **Cinnamon** | Not present | — | — |
| **Xfce** | Not present | — | — |
| **COSMIC** | Not confirmed | — | — |

GNOME's stream: `float32le` ~25 Hz; `application.id=org.gnome.VolumeControl`;
`media.name="Peak detect"`.

harnez's current implementation (`internal/usage/miclive.go`) uses `parec` at
8 kHz mono s16le, reads raw PCM in 200 ms chunks, computes RMS in-memory — equivalent
to (but simpler than) `PA_STREAM_PEAK_DETECT`; no audio is retained.

---

## 11. Minimal Canary Commands

Three-state check: meter alone → meter + ordinary recorder → cleanup.
The existing
[`scripts/canary-gnome-mic-indicator.sh`](../scripts/canary-gnome-mic-indicator.sh)
covers Strategy A for GNOME. Ad hoc commands for other strategies:

### Strategy A — GNOME `application.id` exemption (confirmed)

```bash
# State 1: meter alone — indicator should be OFF
parec --raw --format=s16le --rate=8000 --channels=1 \
      --property=application.id=org.gnome.VolumeControl &
METER_PID=$!
sleep 5
pactl list short source-outputs   # one row, tagged org.gnome.VolumeControl

# State 2: + ordinary recorder — indicator should turn ON
parec --raw --format=s16le --rate=8000 --channels=1 \
      --property=application.id=io.my.test.recorder &
REC_PID=$!
sleep 2
pactl list short source-outputs   # two rows

# State 3: cleanup
kill "$REC_PID" "$METER_PID"
wait
pactl list short source-outputs   # empty
```

### Strategy B — KDE `node.virtual` exemption (requires canary confirmation)

```bash
# State 1: meter alone — indicator should be OFF on KDE, ON on GNOME
parec --raw --format=s16le --rate=8000 --channels=1 \
      --property=node.virtual=true &
METER_PID=$!
sleep 5
wpctl status | grep -A5 Sources
pactl list short source-outputs

# State 2: + ordinary recorder
parec --raw --format=s16le --rate=8000 --channels=1 &
REC_PID=$!
sleep 2
pactl list short source-outputs

# State 3: cleanup
kill "$REC_PID" "$METER_PID"
wait
```

### Strategy C — Combined (recommended portable approach)

```bash
# State 1: meter alone
parec --raw --format=s16le --rate=8000 --channels=1 \
      --property=application.id=org.gnome.VolumeControl \
      --property=node.virtual=true &
METER_PID=$!
sleep 5
pactl list source-outputs | grep -E 'application\.id|node\.virtual'

# State 2: + ordinary recorder
parec --raw --format=s16le --rate=8000 --channels=1 &
REC_PID=$!
sleep 2

# State 3: cleanup
kill "$REC_PID" "$METER_PID"
wait
```

---

## 12. PipeWire-native vs. pipewire-pulse vs. Legacy PulseAudio

| Layer | `node.virtual` | `application.id` |
|---|---|---|
| **PipeWire native API** | Direct | Direct |
| **pipewire-pulse** (`parec`, `pactl`) | via `--property=node.virtual=true` [inferred] | via `--property=application.id=...` [confirmed] |
| **Legacy PulseAudio (no PW)** | Not mapped | Confirmed |

On legacy PulseAudio without PipeWire: `node.virtual` does not exist; Strategy
B/C loses its KDE exemption. Strategy A still works for GNOME-with-PA.
Legacy PulseAudio is increasingly rare as of 2024.

---

## 13. harnez Recommendation

1. **Add `--property=application.id=org.gnome.VolumeControl`** to the `parec`
   invocation in `miclive.go` — confirmed by live canary (issue 248) to suppress
   the GNOME indicator without masking genuine concurrent recording.

2. **Also add `--property=node.virtual=true`** — inferred to suppress KDE
   Plasma's indicator; requires a live canary on a Plasma 6 machine before
   treating as confirmed.

3. **Update the `Recording` heuristic** (`internal/usage/mic.go`): filter out
   source-outputs whose `application.id` is `org.gnome.VolumeControl` or
   `org.PulseAudio.pavucontrol` (and optionally `media.name = "Peak detect"`) to
   avoid false "recording on" readings when GNOME Settings is open simultaneously.

4. **Xfce canary required** before claiming Xfce support. Run Strategy C on a
   live Xfce session and observe whether the panel plugin recording icon appears.

5. **Do not claim Cinnamon or COSMIC suppression** — no indicator to suppress
   has been confirmed, which is a non-issue but should be stated rather than assumed.

6. **Document the identity-spoofing concern** in any production commit that uses
   `org.gnome.VolumeControl`. A standardised PipeWire metadata channel or portal
   mechanism is the right long-term solution, but none exists as of 2026.

---

## 14. Open Questions

| Question | Confidence | Resolution path |
|---|---|---|
| Does `node.virtual=true` via `parec --property` suppress KDE Plasma 6 indicator? | Low [INFERRED] | Live canary on Kubuntu/Fedora KDE Plasma 6 |
| Does Xfce's `xfce4-pulseaudio-plugin` check `node.virtual`? | Very low [UNCONFIRMED] | Read plugin source + live canary on Xfce |
| Does Ubuntu GNOME patch `volume.js` to change `skippedApps`? | Low [UNCONFIRMED] | Inspect Ubuntu gnome-shell package diff |
| Does Cinnamon have any hidden recording indicator? | Low | Live session on Linux Mint 22 |
| COSMIC native indicator timeline? | Unknown | Monitor System76/COSMIC release notes |
| Does `node.virtual=true` affect the GNOME privacy-indicator CSS class? | Low [INFERRED: no effect] | GNOME uses `application.id` only; CSS class is mute-state driven |
| PulseAudioQt `VirtualStream` role — reads `node.virtual` from PW or PA compat? | Medium | Read PulseAudioQt source at `frameworks/pulseaudio-qt` |

---

## 15. Live Input Meter Ballistics & Decoupled TUI Architecture

Building a live voice indicator in a terminal UI requires balancing instantaneous onset responsiveness, fluid visual continuity, and low CPU overhead.

### 15.1 Logarithmic dBFS Meter Scaling (Issue [[257](../issues/257-scale-live-mic-input-level-meter-logarithmically-in-dbfs-to-reflect-audible-speech.md)])
Linear PCM amplitudes compress audible human speech (typically 30–50 dB below full-scale) into the bottom 1–5% of a level bar.
- `harnez usage --watch` scales input logarithmically from **`-60 dBFS` (0%) to `0 dBFS` (100%)**:
  $$\text{level} = \max\left(0.0, \, 1.0 + \frac{20 \log_{10}(\text{RMS}) - \text{PeakOffset}}{60}\right)$$
- Whisper/silence registers around 5–15%; conversational speech spans 35–70%; shouting approaches 90–100%.

### 15.2 Low-Latency Rolling Window (Issue [[258](../issues/258-dynamic-refresh-rate-for-live-mic-meter-high-frequency-ui-redraw-on-speech-activity.md)])
To eliminate perceptible audio buffer lag:
- A rolling window buffer maintains samples across `window-seconds` (default: `0.1s` / 100ms).
- The aggregation metric (`value: "max"`) takes the peak sample within that window, guaranteeing immediate onset detection on voice start.

### 15.3 Equalizer Release Ballistics (Issue [[260](../issues/260-live-mic-meter-equalizer-style-smooth-falloff-visual-decay-for-fluid-voice-dynamics.md)])
A raw 100ms max aggregation without release ballistics collapses abruptly to zero between syllables, creating a strobe-like jitter.
`internal/usage/miclive.go` applies professional VU/equalizer ballistics:
1. **Instant Peak Attack**: If $\text{targetLevel} \ge \text{displayedLevel}$, the displayed level immediately jumps to the peak with zero lag.
2. **Smooth Visual Decay**: If $\text{targetLevel} < \text{displayedLevel}$, the displayed level decays exponentially over elapsed time:
   $$\text{decayedLevel} = \text{prevLevel} \times \exp\left(-\frac{\Delta t}{\tau}\right)$$
   where $\tau = \text{decay-ms}$ (default `150ms`).
3. **Clean Floor Snap**: When decayed level drops below $0.5\%$, it snaps cleanly to $0.0\%$.

### 15.4 Decoupled Hardware Load Sampling & Coalesced Redraw (Issue [[259](../issues/259-decouple-hardware-load-timeline-sampling-from-high-fps-tui-redraw-cadence.md)])
- **Dynamic Framerate**: Redraw rate bumps dynamically from normal cadence (~4 FPS / 250ms) to high-FPS (~20 FPS / 50ms) when voice activity is detected (`high-fps: "auto"`).
- **Decoupled Timeline Sparklines**: High-FPS TUI repaints must not accelerate historical sparkline timelines or overburden CPU/GPU collectors. Hardware timeline sampling is strictly paced at `minLoadSampleInterval = 1s` in `internal/usage/load.go`.
- **Redraw Throttling**: `redrawThrottler` enforces a strict 20 FPS rendering ceiling, debouncing and coalescing bursty background signals.

---

*Last updated: 2026-09-07. Covers upstream GNOME Shell main branch, plasma-pa master branch, and harnez live meter architecture.*
*Primary sources: GNOME Shell `volume.js` (read directly from gitlab.gnome.org),*
*plasma-pa `microphoneindicator.cpp` (read directly from invent.kde.org),*
*issue 248 live canary result on Fedora GNOME, and issues 257–260 implementation.*

