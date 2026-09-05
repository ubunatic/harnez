# 251 — Best-Effort Suppression of Desktop Mic Indicators for Live Meter Stream via Known App Properties and IDs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor (privacy/ergonomics polish: prevents false recording indicator on GNOME/KDE)
**Category**: Architecture / UX
**Related**: [[248-investigate-replicating-gnome-s-non-triggering-mic-level-meter-to-stop-harnez-s-own-parec-stream-false-triggering-the-recording-indicator]],
[[250-research-linux-desktop-microphone-indicators-and-live-input-signal-meters]],
[[244-show-system-mic-level-and-recording-on-off-as-new-usage-watch-tui-box]],
[[245-show-live-mic-input-signal-level-peak-rms-alongside-configured-volume-in-usage-watch]],
`docs/MicIndicators.md`

---

## 1. Problem & Motivation

Ticket 245 introduced a live peak/RMS microphone input level meter in `harnez usage --watch`
(`internal/usage/miclive.go`) by streaming raw PCM samples via a background `parec` subprocess.

However, running this live meter produces two UX/privacy issues:
1. **Desktop Privacy Indicator False Alarm**: Holding open an ordinary `parec` capture stream makes
   GNOME Shell (and KDE Plasma) light up the system-wide microphone-in-use privacy indicator in the
   top bar/panel. This creates user anxiety and defeats the purpose of the privacy indicator.
2. **Self-Triggering `Recording` Status**: `internal/usage/mic.go:currentMicStatusPactl` currently
   detects active recording by checking whether `pactl list short source-outputs` is non-empty.
   Because harnez's own `miclive` meter opens a source-output, it causes harnez to report its own
   `Recording` status as `true` (`on`), self-triggering against itself even when no other app is
   recording.

Investigation tickets 248 and 250 (and `docs/MicIndicators.md`) identified how GNOME and KDE exempt
their own volume meters and validated the GNOME exemption live via `scripts/canary-gnome-mic-indicator.sh`.
This ticket implements those findings in harnez's production codebase.

## 2. Technical Design & Findings

From `docs/MicIndicators.md`:

### 2.1 Stream Property Injection in `miclive.go`
- **GNOME Shell (`volume.js`)**: Excludes capture streams where `application.id` matches
  `org.gnome.VolumeControl` or `org.PulseAudio.pavucontrol`.
- **KDE Plasma (`plasma-pa`)**: Excludes capture streams marked as `VirtualStream` / `node.virtual = true`.
- **Combined Portable Strategy**: Passing both properties to `parec`:
  ```bash
  parec --raw --format=s16le --rate=8000 --channels=1 -d @DEFAULT_SOURCE@ \
    --property=application.id=org.gnome.VolumeControl \
    --property=node.virtual=true \
    --stream-name="Peak detect"
  ```
  This suppresses the indicator across both GNOME and KDE without interfering with each other or
  masking concurrent ordinary recording apps.

### 2.2 Self-Aware Recording Detection in `mic.go`
`currentMicStatusPactl` should not naively treat any active source-output as third-party recording.
Instead, it should parse source-output properties (e.g. from `pactl list source-outputs` or structured
output) and filter out:
- Streams with `application.id = "org.gnome.VolumeControl"` or `"org.PulseAudio.pavucontrol"`
- Streams with `node.virtual = "true"` / `VirtualStream`
- Streams with `media.name = "Peak detect"` / `media.role = "peak-detect"`

When all remaining source-outputs are excluded/meter streams, `MicStatus.Recording` will report `false`.
When an ordinary app (e.g. browser, OBS, speech-to-text, or recording tool) is active, it will report `true`.

## 3. Scope of Implementation

1. **`internal/usage/miclive.go`**:
   - Update `captureMicLiveOnce` to pass the appropriate `--property` arguments (`application.id=org.gnome.VolumeControl`, `node.virtual=true`, and stream identification) to `exec.CommandContext("parec", ...)`.
2. **`internal/usage/mic.go`**:
   - Update `currentMicStatusPactl` to inspect source-outputs with awareness of exempt/meter streams so that neither harnez's own meter stream nor system volume control meters trigger `MicStatus.Recording = true`.
3. **Unit Tests**:
   - Add unit tests in `internal/usage/mic_test.go` verifying the parsing and filtering of `pactl list source-outputs` fixtures (empty, exempt-only, genuine recording app present, mixed).
   - Ensure zero-zombie subprocess management and error degradation invariants remain satisfied.

## 4. Acceptance Criteria

- [ ] `harnez usage --watch --mic` live meter stream does not cause the GNOME Shell (or KDE) top-bar microphone indicator to illuminate on its own.
- [ ] Concurrently launching a separate, ordinary recording stream (e.g. `parec /tmp/test.wav` or browser recording) while `harnez usage --watch --mic` is active properly turns on the desktop indicator and flips `MicStatus.Recording` to `true`.
- [ ] `MicStatus.Recording` reports `false` when only harnez's meter (or desktop settings meters) is active.
- [ ] If `pactl` is unavailable or on non-PipeWire/PulseAudio systems, degrades gracefully without error.
- [ ] All existing tests and new unit tests pass (`make check`).

## 5. Verification

- **Automated**: `go test -v ./internal/usage/...` covering source-output parsing and stream properties.
- **Canary Check**: `scripts/test-canary-gnome-mic-indicator.sh` passes.
- **Manual Verification**: Launch `harnez usage --watch --mic` on Fedora GNOME; verify top-bar indicator stays dark; start concurrent `parec /dev/null`, verify indicator lights up and `Recording: on` displays; terminate `parec`, verify indicator extinguishes and `Recording: off` displays.

