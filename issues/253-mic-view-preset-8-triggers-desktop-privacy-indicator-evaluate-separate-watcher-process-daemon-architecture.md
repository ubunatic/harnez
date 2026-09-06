# 253 — Mic View (Preset 8) Triggers Desktop Privacy Indicator: Evaluate Separate Watcher Process / Daemon Architecture

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate (privacy indicator false alarm when toggling Mic panel via preset `8` or `m`)
**Category**: Architecture / UX / Privacy
**Related**: [[248-investigate-replicating-gnome-s-non-triggering-mic-level-meter-to-stop-harnez-s-own-parec-stream-false-triggering-the-recording-indicator]],
[[250-research-linux-desktop-microphone-indicators-and-live-input-signal-meters]],
[[251-best-effort-suppression-of-desktop-mic-indicators-for-live-meter-stream-via-known-app-properties-and-ids]],
[[082-agent-usage-collector-daemon]],
`docs/MicIndicators.md`, `internal/usage/miclive.go`, `internal/usage/watch.go`

---

## 1. Problem & Motivation

When the user activates the Mic panel in `harnez usage --watch` (via view preset `8` or keyboard shortcut `m`),
the system microphone recording indicator in the desktop environment (e.g. GNOME Shell top bar) lights up.

While ticket 251 addresses direct stream property injection (`application.id=org.gnome.VolumeControl`,
`node.virtual=true`) on the immediate `parec` invocation, there are structural concerns and failure modes:
1. **Process Tree & Session Binding**: Modern desktop portals and session managers (`xdg-desktop-portal`,
   PipeWire session managers) may bind the capture stream to the parent process / terminal emulator
   cgroup or PID tree rather than trusting transient PulseAudio stream properties.
2. **Foreground TUI Coupling**: Having the interactive TUI directly manage a raw audio capture child
   process can create lifecycle friction, privilege/permission prompts, and indicator triggering
   whenever the view mode changes.
3. **Decoupled Architecture Proposal**: The user suggested evaluating whether a separate background
   watcher process or daemon (or hooking into an existing system monitor feed / PipeWire node) should
   be used to isolate and decouple live level monitoring from the interactive CLI process.

## 2. Technical Evaluation & Options

### Option A: Property Spoofing via Direct `parec` (Ticket 251 Baseline)
- **Mechanism**: Pass `--property=application.id=org.gnome.VolumeControl` and `--property=node.virtual=true`
  directly to the `parec` subprocess in `internal/usage/miclive.go`.
- **Pros**: Zero background infrastructure; lightweight; already verified in isolation via
  `scripts/canary-gnome-mic-indicator.sh` on Fedora GNOME.
- **Cons**: Relies on desktop-specific allowlists; may fail if PipeWire/GNOME tightens identity verification
  or checks PID credentials via D-Bus / portal.

### Option B: Dedicated Background Watcher / Collector Daemon
- **Mechanism**: A background user service or standalone worker process (analogous to `harnez agent-collector`
  in issue 082) that captures audio level samples at low frequency and writes rolling RMS/peak metrics
  to a shared memory segment, unix socket, or state file in `$XDG_RUNTIME_DIR/harnez/`.
- **Pros**: Complete separation from the interactive TUI; can run with specific systemd service identities
  or process attributes; TUI only reads metrics from shared memory/file without touching audio streams.
- **Cons**: Additional background daemon complexity; daemon lifecycle management; potential battery/CPU
  overhead if left running when not actively monitored.

### Option C: Passive PipeWire Introspection / Audio Hook
- **Mechanism**: Hook into PipeWire's native metadata or peak detection (`resample.peaks`, `node.passive=true`)
  via a small C/Go helper or native WirePlumber/PipeWire client.
- **Pros**: Cleanest native integration; no raw PCM streaming; native PipeWire passive link semantics.
- **Cons**: Higher implementation complexity; cgo or external helper binary required.

## 3. Scope & Next Steps

1. **Verify Ticket 251 First**: Confirm whether the direct stream property injection in 251 successfully
   extinguishes the indicator when pressing `8` in `harnez usage --watch`.
2. **Assess Residual Indicator Leaks**: If the indicator still triggers (due to cgroup/desktop portal
   tracking or non-GNOME desktop environments):
   - Prototype a decoupled watcher helper/daemon or socket-based provider.
   - Evaluate hooking into existing system audio meter nodes or passive PipeWire interfaces.
3. **Document Architecture Decision**: Update `docs/MicIndicators.md` with findings on whether a
   separate process boundary is required for reliable indicator suppression.

## 4. Acceptance Criteria

- [ ] Clear diagnosis of why the indicator lights up under preset `8` (distinguishing un-aliased `parec` from portal/cgroup attribution).
- [ ] Evaluation of the separate process / daemon architecture versus direct stream property spoofing.
- [ ] Toggling preset `8` or `m` in `usage --watch` provides live mic level metering without triggering unwanted system privacy indicators.
- [ ] Documented recommendation and architecture in `docs/MicIndicators.md`.

## 5. Verification

- Interactive testing on target desktop environments (GNOME Shell, KDE Plasma) switching into and out of preset `8`.
- Validate indicator state and process tracking with `pactl list source-outputs` and desktop top-bar visual checks.

