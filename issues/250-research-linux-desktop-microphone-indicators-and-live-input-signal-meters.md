# 250 — Research Linux Desktop Microphone Indicators and Live Input Signal Meters

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor (cross-desktop privacy UX and portability uncertainty)
**Category**: Architecture
**Related**: [[248-investigate-replicating-gnome-s-non-triggering-mic-level-meter-to-stop-harnez-s-own-parec-stream-false-triggering-the-recording-indicator]],
[[244-show-system-mic-level-and-recording-on-off-as-new-usage-watch-tui-box]],
[[245-show-live-mic-input-signal-level-peak-rms-alongside-configured-volume-in-usage-watch]],
`docs/Canary.md`

---

## 1. Problem & Motivation

Issue 248 confirmed on Fedora GNOME that a capture stream tagged
`application.id=org.gnome.VolumeControl` can supply real microphone samples without lighting
GNOME Shell's microphone-in-use indicator, while a concurrent ordinary recorder still lights the
indicator. That result is desktop-specific. Harnez should not assume that GNOME's allowlist,
PulseAudio compatibility behavior, or privacy-indicator policy transfers to other modern Linux
desktops and distributions.

Research how major Linux desktop environments and representative distributions implement both:

1. the privacy/status indicator that tells the user an application is using the microphone; and
2. a live input-signal meter showing the amplitude of sound currently reaching the microphone
   (peak/RMS/noise activity), distinct from the configured input-volume/gain slider.

The goal is a sourced portability model for harnez's live meter, not an implementation. Preserve
uncertainty where behavior depends on desktop version, distro patches, audio server, portal use, or
application sandboxing.

## 2. Research Scope

Cover current supported releases of the main desktop families and representative distro packaging,
at minimum:

- GNOME on Fedora and Ubuntu;
- KDE Plasma on Fedora KDE or Kubuntu and one independently packaged Plasma distribution such as
  openSUSE;
- Cinnamon on Linux Mint;
- Xfce on a representative distribution;
- COSMIC where its stable/current implementation provides either feature.

Add other material desktops only when primary-source evidence is available. Treat the desktop
environment as the primary implementation unit, then identify distro patches or configuration that
changes its behavior. Do not multiply identical upstream behavior into unsupported distro-specific
claims.

For each covered environment, determine:

- Whether a microphone-in-use indicator exists by default, where it appears, and which releases
  provide it.
- What exact event or state drives visibility: audio-server streams/nodes, device running state,
  portals, permissions, application metadata, stream roles, or another mechanism.
- Which streams or applications are excluded and the exact matching key/value or policy involved.
- Whether the system settings UI exposes a live input-signal meter and how it obtains actual signal
  amplitude without either showing the privacy indicator or masking genuine concurrent recording.
- Whether the meter consumes raw PCM, peak-detect samples, PipeWire metadata, or another API; note
  sample format/rate and passive/monitor properties when evidenced.
- Differences across PipeWire-native operation, `pipewire-pulse`, and legacy PulseAudio where still
  supported.
- Whether Flatpak/portal-mediated capture changes indicator semantics.
- Whether imitating a trusted settings application's identity is a stable public convention, a
  private implementation detail, or a privacy/security anti-pattern that harnez should avoid.

## 3. Evidence Standards

- Prefer upstream desktop, audio-server, portal, and distro packaging source code, commit history,
  release notes, and official documentation.
- Cite exact source paths, predicates, properties, and version/commit references. Secondary sources
  may locate evidence but must not be the sole basis for technical conclusions.
- Clearly distinguish code-proven behavior, live-canary observations, and inference.
- Do not infer indicator behavior from the mere existence of a source-output or PipeWire node.
- Do not claim that a level meter is privacy-indicator-safe without showing both why its own stream
  is exempt and why an ordinary concurrent recorder remains visible.
- Use bounded, non-recording source inspection first. Any live probe must capture only transient
  amplitude data, write no retained audio, and clean up all streams and helper processes.

## 4. Deliverables & Acceptance Criteria

- A concise comparison organized by desktop environment and qualified by distribution/version,
  covering all minimum targets or explicitly documenting unavailable evidence.
- For every supported claim, direct primary-source references to the indicator predicate and live
  meter implementation.
- A normalized glossary of cross-stack concepts such as application ID, media role/name,
  source-output, PipeWire node/link state, peak detect, passive node, and portal session.
- A decision section grouping environments into reusable strategies rather than proposing one
  hard-coded GNOME workaround for all Linux systems.
- A recommendation for harnez: supported portable mechanism, per-desktop adapters, GNOME-only
  behavior, or no indicator exemption when the available mechanism depends on impersonating a
  privileged/trusted application identity.
- Minimal canary commands or scripts for each viable strategy, including a three-state check:
  meter alone, meter plus ordinary recorder, then cleanup.
- Research findings recorded in this ticket or a linked study, with unresolved questions and
  confidence levels stated explicitly. No production-code change belongs to this ticket.
