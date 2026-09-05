# 248 — Investigate Replicating GNOME's Non-Triggering Mic Level Meter to Stop harnez's Own `parec` Stream False-Triggering the Recording Indicator

**Status**: Open — Filed follow-up research ticket on GNOME mic meter mechanism
**Priority**: P2 (Medium)
**Severity**: Minor (UX/trust regression, real workaround exists — toggle the Mic box off)
**Category**: UX / Agentic Ergonomics / Privacy Perception
**Related**: [[244-show-system-mic-level-and-recording-on-off-as-new-usage-watch-tui-box]],
[[245-show-live-mic-input-signal-level-peak-rms-alongside-configured-volume-in-usage-watch]]
(shipped `internal/usage/miclive.go`'s `parec`-based live peak/RMS meter and 245's own
`Recording` detection), `docs/practices/Canary.md`

---

## 1. Problem & Motivation

User feedback after trying 245's live mic meter live on this Linux/PipeWire (GNOME) machine:

> harnez's own live meter holds open a `parec` capture stream on the default source, which makes
> GNOME's system-level "microphone is in use" recording indicator (the icon in the top bar /
> notification area) light up — and it also flips harnez's own `Recording` field to `true`
> (`internal/usage/mic.go:172-173`, `pactl list short source-outputs` non-empty), i.e. harnez's mic
> meter now triggers its own "recording is on" reading against itself.

By contrast, GNOME Settings' own Sound > Input panel shows a live, continuously-moving level meter
but does **not** trigger that same system recording indicator. The user wants to know concretely
how GNOME does this, and whether harnez can replicate it, before deciding whether/how to change
`miclive.go`.

## 2. Research Findings

### 2.1 How GNOME's own level meter works (confirmed)

GNOME's Sound Settings level meter comes from `libgnome-volume-control` (the `gvc` library shared
by gnome-control-center's Sound panel and gnome-shell's volume slider), which historically opens
its peak-metering stream via PulseAudio's client API with the **`PA_STREAM_PEAK_DETECT`** stream
flag (`pa_stream_connect_record()`). This is a real, long-standing PulseAudio feature, not GNOME-
specific: it creates a stream at a throttled ~25Hz float32le mono rate that yields only a scalar
peak value per sample instead of raw audio, to minimize CPU cost
([Monitoring Audio Levels with PulseAudio — menno.io](https://menno.io/posts/pulseaudio_monitoring/)).

Critically — and this directly explains the discrepancy the user is seeing — **this peak-detect
stream is still a genuine `pa_source_output` / real capture client**, not some non-consuming,
purely-introspective read. PulseAudio's own `source-output.c` sets a literal, recognizable
identity on it: `media.name = "Peak detect"` (confirmed via a PulseAudio debug-log excerpt showing
this exact property assignment in `source-output.c`,
[pastebin.com/WYnA0xrF](https://pastebin.com/WYnA0xrF)). This means `pactl list short
source-outputs` almost certainly lists GNOME's own peak-detect stream too, the same way it lists
harnez's `parec` stream — GNOME is not avoiding the source-outputs list structurally, it is running
a stream that some *other, filtering* consumer (see 2.2) knows to recognize and ignore by name/role,
which harnez's naive "any source-output → recording on" heuristic (245) does not do.

Under PipeWire's PulseAudio-compatibility layer (`pipewire-pulse`, what `pactl`/`parec` on this
machine actually talk to), `PA_STREAM_PEAK_DETECT` is understood to map to PipeWire's own
**`resample.peaks`** stream property — documented in `pipewire-props(7)`
([docs.pipewire.org/page_man_pipewire-props_7.html](https://docs.pipewire.org/page_man_pipewire-props_7.html))
as: *"Instead of actually resampling, produce peak amplitude values as output. This is used for
volume monitoring, where it is set as a property of the 'recording' stream."* PipeWire's own docs
explicitly call it a **"recording" stream** — reinforcing that this is not a magic non-recording
mode, just a real capture stream tagged for identification/exemption elsewhere.

A second PipeWire property surfaced during this research, **`node.passive`** — *"This is a passive
node and so it should not keep sinks/sources busy. This property makes the session manager create
passive links to the sink/sources"* (same `pipewire-props(7)` source) — is a plausible complementary
mechanism (a passive link might not count toward a "busy"/"in use" state the way a normal capture
link does), but this session could **not** confirm gvc/gnome-control-center actually sets
`node.passive` on its peak-detect stream; PulseAudio's `pa_stream_connect_record()` API (the path
gvc most commonly still uses even on PipeWire, via `pipewire-pulse`) has no direct equivalent flag
to `node.passive` — that property only has a clear meaning on PipeWire's native (non-Pulse-compat)
client API. **Flag as unconfirmed** rather than asserted.

### 2.2 What actually drives GNOME's "microphone in use" system indicator (not fully pinned down)

This session searched upstream `gnome-shell` source directly (`GNOME/gnome-shell` on GitHub/GitLab,
main branch) for a microphone equivalent of its existing camera-in-use indicator. Findings:

- `js/ui/status/camera.js` + `src/shell-camera-monitor.c` implement a **camera**-in-use indicator by
  opening a raw PipeWire registry connection and filtering nodes by `PW_KEY_MEDIA_ROLE == "Camera"`,
  then watching that node's own `PW_NODE_STATE_RUNNING` state directly — i.e. it inspects the
  *device/node's own running state*, not a client/link list, and it keys the filter off
  `media.role`, a real, precedented mechanism for "this kind of node is a Camera, treat it
  specially."
- **No equivalent `shell-microphone-monitor.c` or `microphone.js` exists in upstream gnome-shell**
  (confirmed via GitHub API directory listing of `js/ui/status/` and `src/`) — there is currently no
  vanilla-GNOME-Shell-core "mic in use" indicator analogous to the camera one.
- This means the mic-in-use icon the user is actually seeing in the top bar is most likely either
  (a) a distro-specific patch (Ubuntu and some distros are known to carry shell/session patches
  layering extra indicators — not confirmed for this exact feature within this session's search
  budget), (b) a separate portal/privacy mechanism (`xdg-desktop-portal`-mediated sandboxed-app
  microphone access, which would not apply to a bare `parec` call anyway), or (c) a GNOME Shell
  Extension already installed on this machine. **This session could not conclusively identify the
  exact component rendering the indicator the user described** — this is the single biggest open
  gap in this research.

### 2.3 Practical, testable lead (confirmed via local tooling, not yet tried live)

`parec --help` on this machine confirms two directly usable flags:

```
--stream-name=NAME                How to call this stream on the server
--property=PROPERTY=VALUE         Set the specified property to the specified value.
```

This means harnez does **not** need to link libpipewire directly to experiment — `miclive.go`'s
existing `parec` invocation could be extended with `--stream-name="Peak detect"` (mirroring
PulseAudio's own literal internal name for this exact use case) and/or `--property=media.role=...`
to mimic whatever property-based identity the real "Peak detect" stream carries, entirely via CLI
flags already available to `exec.Command`. This is a cheap, real experiment to try before any
heavier investment.

## 3. Implementation Direction (not yet built — findings-based, no code written for this ticket)

Two independent, non-code-writing threads, both realistically CLI-reachable (no libpipewire binding
needed) if the hypothesis below holds — but **must be canary-verified live before any code change**,
per `docs/practices/Canary.md`, since the exact filter driving the real system indicator on this
machine is not yet identified (§2.2):

1. **Tag harnez's `parec` stream identically to PulseAudio's own peak-detect convention** —
   `--stream-name="Peak detect"` (and/or `--property=media.role=...`, `--property=media.name=...`
   once the exact expected value is confirmed by a live before/after check) — then observe whether
   the system recording indicator still lights up. This is a pure flag change to the existing
   `exec.Command("parec", ...)` call in `miclive.go`, not a rewrite.
2. **Make harnez's own `Recording` heuristic (`internal/usage/mic.go:172-173`) self-aware** —
   regardless of whether (1) fools the system indicator, `pactl list short source-outputs` output
   includes each source-output's properties (`pactl list source-outputs` without `short` shows
   `media.name`/`application.name`); filtering out entries whose name matches harnez's own
   stream-name tag (and ideally also PulseAudio's own literal `"Peak detect"` name, to avoid
   double-counting if GNOME's own meter happens to be open at the same time) would fix the
   self-triggering half of the bug (245's own Recording field flipping on because of harnez's own
   meter) independent of whether the GNOME system indicator can be fooled at all.

**Not recommended without further evidence**: linking libpipewire directly (a Go PipeWire binding,
or cgo against `libpipewire`) to open a `node.passive`-flagged native stream — this is the most
"authentic" replication of a hypothesized mechanism that could not be confirmed in §2.1, and would
be a materially larger dependency/complexity commitment (matching 245 §2's own noted "likely overkill
for one focused box" caveat about native PipeWire client usage) for a benefit not yet shown to be
necessary once (1)/(2) above are tried.

## 4. Open Questions

- What actual component renders the mic-in-use icon the user sees (§2.2) — distro shell patch,
  GNOME extension, or a portal/privacy mechanism? Needs to be identified live on this machine
  (`gnome-extensions list --enabled`, checking for shell patches, or asking the user directly what
  desktop/distro/version this is) before assuming CLI flag (1) above will even have an effect on it.
- Does `--stream-name="Peak detect"` (or matching properties) on harnez's `parec` invocation actually
  suppress the real system indicator on this machine? Requires a live before/after check (start
  `--watch --mic`, observe indicator state, is the single most decisive next step) — not assumed
  true by this ticket.
- If GNOME's own peak-detect stream is confirmed (via `pactl list source-outputs` while GNOME
  Settings' Sound panel is open) to also appear as a normal source-output, should harnez's
  `Recording` heuristic in general (not just self-exemption) also exclude any stream named
  "Peak detect" system-wide, to avoid a false "recording on" reading whenever *any* app (not just
  harnez) opens GNOME's own meter? This would be a small, separate, generally-applicable fix to
  245's existing detection regardless of what happens with harnez's own stream.

## 5. Manual Canary

`scripts/canary-gnome-mic-indicator.sh` now provides three explicit, timed phases for testing the
desktop indicator without changing production code. Run each phase separately and watch the GNOME
top-bar indicator:

```bash
scripts/canary-gnome-mic-indicator.sh control 8
scripts/canary-gnome-mic-indicator.sh exempt 8
scripts/canary-gnome-mic-indicator.sh combined 10
```

`control` opens an ordinary uniquely identified `parec` stream. `exempt` sets
`application.id=org.gnome.VolumeControl`. `combined` starts and verifies the exempt stream first,
then waits for Enter before starting a separately identified ordinary recorder and waits for Enter
again before cleanup. These checkpoints make each expected indicator transition unambiguous. The
canary confirms the matching source-output properties and increasing PCM byte counts, bounds each
interactive prompt and recorder lifetime, and traps exit/signals to reap its processes. Calling it
with no phase only prints usage and exits; it never starts capture.

The deterministic companion check is non-audio and safe to run automatically:

```bash
scripts/test-canary-gnome-mic-indicator.sh
```

## 6. Live Result and Conclusion

The interactive canary was run on the target Fedora GNOME session. Its three synchronized states
were observed as follows:

1. The `parec` stream tagged `application.id=org.gnome.VolumeControl` left the microphone indicator
   off.
2. Adding an otherwise ordinary `parec` recorder made the indicator turn on while the exempt stream
   remained active.
3. Stopping both recorders made the indicator turn off again.

This confirms that upstream GNOME Shell's relevant exemption is the source-output's
`application.id`, not its `media.name`, `"Peak detect"` stream name, peak-detection mode, or
`node.passive`. The focused production change should tag harnez's meter stream with
`--property=application.id=org.gnome.VolumeControl`; a separately identified ordinary recorder
continues to surface correctly, so the exemption does not mask genuine concurrent recording.
