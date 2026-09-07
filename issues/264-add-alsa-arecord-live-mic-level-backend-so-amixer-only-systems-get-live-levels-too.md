# 264 — Add ALSA/arecord live-mic-level backend so amixer-only systems get live levels too

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: Issue 262; Issue 245; Issue 244; Issue 248; Issue 257; Issue 258; Issue 260; Issue 265 (complementary — PipeWire-native `pw-record`/`pw-cat` backend for systems with PipeWire but no `pactl`/`parec`; this ticket covers no-PipeWire-at-all amixer-only systems); `internal/usage/miclive.go`; `internal/usage/mic.go`; `internal/usage/watch.go`

---

## 1. Problem & Motivation

Follow-up to issue 262. That ticket fixed the Mic box's messaging on
amixer-only systems: instead of a bare, unexplained `"live n/a"`, the box now
says `"live n/a (needs pactl/PipeWire)"`. That is an honest description of
*this implementation's* current limitation — it is not a description of a
hardware or OS constraint.

User's point in opening this follow-up (verbatim): "research 'needs
pactl/PipeWire' what can we do about it, other app in the system can use the
mic so we should too." Concretely: recording apps on an amixer-only system
(no PulseAudio/PipeWire session, or one this codebase's `pactl` probe
doesn't detect) can and do read live mic input fine — the OS/hardware mic
path works. harnez's live meter is unavailable purely because its only
capture backend (`parec`) requires a PipeWire/PulseAudio server. That's a
gap in this codebase's backend coverage, not a hard constraint imposed by
Linux audio.

## 2. How Backend Detection And Live Streaming Work Today (Read From Code)

- `internal/usage/mic.go`'s `resolveMicBackend()` canary-probes, in order,
  `pactl get-default-source` (PipeWire/PulseAudio) and then `amixer get
  Capture` (plain ALSA), caching whichever succeeds first as
  `micBackendPactl` / `micBackendAmixer` / `micBackendNone`.
- **Configured-gain reading** (`CurrentMicStatus`, issue 244) works on
  *either* backend: `currentMicStatusPactl` reads `pactl get-source-volume`;
  `currentMicStatusAmixer` reads `amixer get Capture`. Both are one-shot
  polls of a static mixer setting, not a signal reading.
- **Live streaming reading** (`internal/usage/miclive.go`, issue 245) is
  gated hard on `resolveMicBackend() == micBackendPactl` in
  `startMicLiveManager` (miclive.go:219-230), plus a `parec` LookPath check.
  If either fails, the function returns a manager that never spawns the
  capture goroutine — the box degrades to `LiveAvailable: false`, no error.
- **Why amixer can't currently stream levels**: `amixer`/ALSA's simple-mixer
  interface (`amixer get Capture`) only exposes a *static configured
  gain/mute setting* — it has no equivalent to `parec`'s raw sample stream.
  There is nothing wrong with this diagnosis; the gap is that ALSA *does*
  have a separate, lower-level raw-capture path (`arecord`, part of
  `alsa-utils`) that this codebase never tries. `arecord` opens a PCM
  capture device directly (bypassing any sound server) and streams raw
  samples to stdout — structurally the same shape `parec` provides today,
  just at the ALSA layer instead of the PipeWire/PulseAudio layer.

## 3. Proposed Approach: `arecord` As A Third Live-Capture Backend

Add `arecord`-based raw PCM capture as a live-level source alongside the
existing `parec` path, so the live meter works whenever *any* usable ALSA
capture device exists — independent of whether a PipeWire/PulseAudio server
is running or detected.

Sketch, keeping the existing pactl path untouched:

- `startMicLiveManager` (miclive.go:219-230) currently early-returns unless
  `resolveMicBackend() == micBackendPactl` and `parec` is on PATH. Extend it
  to also start capture on `micBackendAmixer` when `arecord` is on PATH,
  selecting the subprocess command/args by backend (mirrors how `mic.go`
  already dispatches `CurrentMicStatus` per-backend).
- Add a `captureMicLiveOnceArecord` sibling to `captureMicLiveOnce`
  (miclive.go:279-318), invoking something like:
  `arecord -q -D default -f S16_LE -r 8000 -c 1 -t raw` — same PCM16LE mono
  8kHz shape `micLiveAmplitudeFromPCM16LE` already parses, so the RMS/dBFS
  math, ballistics, decay, and windowing (`micLiveMeter.update`) are reused
  unchanged. Only the subprocess invocation and its stdout framing differ.
- Same clean-teardown shape as the existing path: `cmd.Cancel` sends
  SIGTERM, `cmd.WaitDelay` bounds shutdown, reconnect-with-backoff via
  `runMicLiveManager`'s existing loop.
- `MicStatus.LiveAvailable`'s doc comment (mic.go:51-56) and the box's
  `"needs pactl/PipeWire"` copy (from issue 262) both need updating once a
  second backend exists — the message should become backend-aware
  ("needs arecord" / "needs parec or arecord installed") rather than
  hard-naming pactl/PipeWire as the only path.

### Why this is the right fallback, not a random alternative

Researched during this ticket (not yet a canary probe against a real
amixer-only machine — see Constraints below):

- `arecord` ships in `alsa-utils`, which is a near-universal base package on
  Linux — present by default on most distros, and present even on
  PipeWire-based systems, because PipeWire's own ALSA plugin/emulation
  layer sits on top of the same ALSA kernel interface `arecord` talks to.
  Confirmed on this dev machine (a PipeWire system): `arecord`, `aplay`, and
  `pw-cat` are all present (`alsa-utils 1.2.15.2-1ubuntu1`), i.e. installing
  PipeWire does not remove or replace `alsa-utils`.
- Other options considered and set aside:
  - `/proc/asound/card*/pcm*c/sub*/status` — only reports capture-active
    state (running/closed), no sample data; can't produce an RMS/peak
    level. Possibly useful later as a cheap "is anything capturing"
    booster for `Recording` on the amixer backend (a separate, smaller
    follow-up, not this ticket's scope), but not a levels source.
  - `pw-cat` (native PipeWire CLI, distinct from `pactl`/`parec`) — doesn't
    help *this* ticket's goal, since it still requires a PipeWire server;
    the whole point here is a path that works when no PipeWire/PulseAudio
    server is running or detected in the first place. Not pursued further.
  - No other well-known, dependency-light Linux CLI mechanism for raw
    live-level metering was found beyond the ALSA (`arecord`) and
    PipeWire/PulseAudio (`parec`/`pw-cat`) layers.

## 4. Constraints & Open Questions (Honest, Not Yet Verified)

- **Not yet canary-probed on a real amixer-only machine.** This dev machine
  has PipeWire, so `resolveMicBackend()` resolves to `micBackendPactl` here
  and the amixer path is untested in practice. Per docs/practices/Canary.md,
  implementation work on this ticket should start with a live probe of
  `arecord -q -D default -f S16_LE -r 8000 -c 1 -t raw` on an actual
  amixer-only box (or one with PipeWire's `pactl` probe forced to fail)
  before writing the Go integration, confirming: it opens the default
  capture device, streams changing amplitude data, and terminates cleanly
  on SIGTERM — the same three things issue 245's Resolution section
  recorded for `parec`/`pw-cat`.
  - Note that if this codebase's own `pactl`/PipeWire *detection* is itself
    the real gap on the user's machine (e.g. PipeWire is actually running
    but `probePactlDefaultSource()` fails for some other reason), that
    would be a different, narrower bug than "no PipeWire at all" — worth
    ruling out as part of the canary probe, since it would call for a
    detection fix instead of (or in addition to) a new backend.
- **Device name (`-D default`) may not always be right.** ALSA's `default`
  PCM may not map to the same physical source `amixer get Capture` reports
  gain for on all configurations; may need to resolve a matching device
  name rather than assuming `default` is always correct.
- **Permissions**: `arecord` needs the invoking user to have access to the
  capture device (typically the `audio` group or equivalent) — same
  permission class `parec` already depends on, not a new constraint, but
  worth confirming the failure mode (permission denied) degrades to
  `LiveAvailable: false` rather than a hang or crash.
- **CPU cost**: negligible at the same 8kHz/mono/20Hz-chunk rate already
  used for `parec` (issue 245's chunk-size rationale applies unchanged).
- **New dependency**: adds `arecord`/`alsa-utils` as a second live-capture
  binary dependency, on top of the existing `parec` (pulseaudio-utils)
  dependency. Given `alsa-utils` is effectively part of the Linux audio
  base layer (confirmed present even on this PipeWire dev machine), this is
  a low-risk addition, not a heavyweight new external requirement.

## 5. Acceptance Criteria

- [ ] Canary probe confirms `arecord` produces live, changing amplitude data
  on a real ALSA capture device and exits cleanly on SIGTERM (mirrors issue
  245's `parec`/`pw-cat` probe).
- [ ] `startMicLiveManager` starts an `arecord`-based capture goroutine when
  `resolveMicBackend() == micBackendAmixer` and `arecord` is on PATH,
  reusing `micLiveAmplitudeFromPCM16LE`/`micLiveMeter` unchanged.
- [ ] Reconnect-with-backoff and SIGTERM clean-teardown behavior parity with
  the existing `parec` path (`runMicLiveManager`'s loop, `cmd.Cancel`,
  `cmd.WaitDelay`).
- [ ] `MicStatus.LiveAvailable`'s doc comment and the Mic box's "needs
  pactl/PipeWire" copy (issue 262) are updated to be backend-aware once a
  second live backend exists.
- [ ] Unit tests for any new parsing/framing logic; existing
  `micLiveAmplitudeFromPCM16LE`/ballistics tests remain untouched since the
  math is reused as-is.
- [ ] `go test ./...` passes; manual live verification on an amixer-only (or
  forced-non-pactl) machine before closing, not just unit tests — this is a
  hook/environment-resolution-shaped feature per
  docs/practices/AgenticLoop.md's "Live/Real-Environment Verification"
  review checklist item.
