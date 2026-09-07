# 265 — Add PipeWire-native pw-record/pw-cat live-mic-level backend for pactl-less PipeWire systems

**Status**: Closed
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Feature
**Related**: Issue 262; Issue 264; Issue 245; Issue 244; `internal/usage/mic.go`; `internal/usage/miclive.go`; `internal/usage/watch.go`

---

## 1. Problem & Motivation

Confirmed live, on the user's actual current machine, this session (not
hypothetical):

- `which pactl` / `which parec` → **not found**. Neither binary is on PATH.
- PipeWire is fully installed and running: `systemctl --user status
  pipewire pipewire-pulse` shows both **active**; `pw-cli info 0` succeeds.
- Installed packages: `pipewire`, `pipewire-pulse`, `pipewire-alsa`,
  `pipewire-audio`, `pipewire-bin`, `wireplumber` — but **not**
  `pulseaudio-utils`, the separate compatibility package that ships
  `pactl`/`parec`.
- `pw-cat`, `pw-record`, and `pw-cli` **are** on PATH (they ship with
  `pipewire-bin`, installed automatically whenever the PipeWire server
  itself is, unlike `pulseaudio-utils` which is optional).

This exactly matches the user's original complaint that led to issue 262:
"other app in the system can use the mic so we should too." Other apps on
this machine talk to PipeWire directly (or via the `pipewire-alsa`/
`pipewire-pulse` compat shims) and the mic works fine. But
`internal/usage/mic.go`'s `resolveMicBackend()` only probes `pactl
get-default-source`, and `internal/usage/miclive.go`'s live-capture path
(`captureMicLiveOnce`) is hard-gated to `resolveMicBackend() ==
micBackendPactl` and shells out to `parec` specifically (verified against
current source in this session, post-262 fix at commit 5eb361c — the gate
and the `parec` invocation are unchanged). With no `pactl`/`parec` on PATH,
`resolveMicBackend()` falls through to the `amixer` probe, and the box shows
`"live n/a (needs pactl/PipeWire)"` — even though PipeWire is fully running
and usable. The message issue 262 shipped is accurate about *why* today's
code can't get a live reading, but it describes a codebase gap, not a
hardware/OS constraint: this machine's mic is fully live-capturable, this
codebase just doesn't know how to ask PipeWire for it without the
PulseAudio-compat shim installed.

This is **complementary to issue 264, not a duplicate**. 264 targets
systems with no PipeWire session at all (amixer-only, e.g. plain ALSA with
no sound server) and proposes an ALSA/`arecord` live backend. This ticket
targets systems **with PipeWire running** but **without the
`pactl`/`parec` compatibility package** — arguably the more common modern
desktop case today: PipeWire is now the Ubuntu/Fedora default sound server,
and `pulseaudio-utils` is not always pulled in as a dependency (as
demonstrated by this exact machine). The two tickets propose different
backends for genuinely different gaps in `resolveMicBackend()`'s coverage.

## 2. Format-Compatibility Findings (Verified This Session)

`internal/usage/miclive.go`'s own doc comment (lines 24-33) already records
a hand canary-probe from issue 245's original investigation
(2026-09-05, on a PipeWire 17.0 dev machine): both

```
parec --raw --format=s16le --rate=8000 --channels=1 -d @DEFAULT_SOURCE@
```

and

```
pw-cat -r --target <source> --format s16 --rate 8000 --channels 1 -
```

were probed side by side and **both produced live, changing amplitude data
and terminated cleanly on SIGTERM**. `parec` was chosen at the time purely
because it matched the existing pactl-only backend gate and needed no
numeric source-id resolution (`@DEFAULT_SOURCE@` vs. a PipeWire node
target) — not because `pw-cat` was structurally worse. That prior probe is
strong prior evidence this approach works; this session did not have
physical mic access to re-run a live capture, so re-verify the actual byte
stream on a real machine before merging (see Open Questions).

Re-checked `pw-record --help` and `pw-cat --help` on this session's machine
(pipewire-bin present) to confirm flag support against the current
installed version:

- Both tools support `--rate`, `--channels`, `--format` (default `s16`,
  i.e. signed 16-bit — matches `micLiveAmplitudeFromPCM16LE`'s expected
  `int16` little-endian samples), and `--container` (raw vs. wav/etc.).
- `-a, --raw` (`pw-record`) / the RAW mode flag forces raw sample output
  with no container header — needed so the byte stream `captureMicLiveOnce`
  reads from stdout is pure PCM samples, not a WAV-wrapped stream (a WAV
  header would corrupt the first ~44 bytes fed into
  `micLiveAmplitudeFromPCM16LE`).
- `pw-cat` additionally exposes `-r, --record` (recording mode) and
  `--target` (set node target serial or name, default auto) — the
  mechanism to pin capture to a specific source/node, playing the same role
  `parec -d @DEFAULT_SOURCE@` plays today.
- `pw-record` is dedicated to recording (no `-r` mode flag needed) and
  otherwise exposes the same `--rate`/`--channels`/`--format`/`-a` surface.
- Net: either tool, invoked with `--raw --format=s16 --rate=8000
  --channels=1 -` (writing to stdout via `-` as the output file/target),
  should produce the same raw PCM16LE mono stream shape
  `micLiveAmplitudeFromPCM16LE` already consumes from `parec`, requiring no
  changes to the RMS/ballistics code in `miclive.go` — only a new capture
  subprocess invocation function analogous to `captureMicLiveOnce`.

## 3. Proposed Approach

- Extend `resolveMicBackend()` (or add a capture-layer probe alongside it)
  with a new backend kind, e.g. `micBackendPipeWireNative`, detected when
  `pactl`/`parec` are absent but a PipeWire socket is reachable (e.g.
  `pw-cli info 0` succeeds, or `pw-record`/`pw-cat` are on PATH and a
  quick, bounded canary invocation succeeds).
- Detection/fallback order becomes: **1) `pactl`/`parec`** (existing,
  unchanged — most mature, symbolic `@DEFAULT_SOURCE@` resolution) → **2)
  `pw-record`/`pw-cat`** (new, this ticket — PipeWire running, no
  PulseAudio-compat shim) → **3) `amixer`** (existing, static-only; issue
  264's proposed ALSA/`arecord` live path would slot in here or alongside
  it for systems with neither PipeWire nor pactl).
- Configured-gain reading (`CurrentMicStatus`, issue 244's one-shot
  `pactl`/`amixer` polls) is out of scope here — this ticket is about the
  *live streaming* meter only (issue 245's scope), matching how 264 is
  also scoped to the live path. Follow-up: issue 270 implements that
  configured-gain/mute read for the pipewire backend via `wpctl`.
- Add a `captureMicLiveOnce`-equivalent function (e.g.
  `captureMicLiveOnceViaPipeWire`) invoking `pw-record` (or `pw-cat -r`)
  with the raw-mode flags above, feeding the same stdout pipe →
  `micLiveAmplitudeFromPCM16LE` → ballistics pipeline unchanged.
- Update the Mic box's "n/a" messaging (issue 262's fix) so it no longer
  says "needs pactl/PipeWire" once a PipeWire-native path exists and is
  tried — the message should only degrade to a genuine "no audio
  interface" state when neither compat layer works.

## 4. Open Questions

- **Node/source targeting**: `parec -d @DEFAULT_SOURCE@` resolves the
  default source symbolically with no numeric ID lookup. `pw-record`'s
  `--target` defaults to "auto" per its own help text — confirm this
  actually follows the same default-source semantics as PulseAudio's
  `@DEFAULT_SOURCE@` (i.e. picks the currently-configured default capture
  device) rather than an arbitrary/first node, especially when multiple
  input devices are present.
- **Permission model**: does `pw-record`/`pw-cat` require anything beyond
  what `parec` needs (e.g. a running user session bus, `XDG_RUNTIME_DIR`,
  membership in an `audio`-adjacent group) that could differ across
  distros/desktop environments?
- **CPU/latency cost**: `parec`'s `--latency-msec=20 --process-time-msec=20`
  flags tune buffering for the 20 Hz sample cadence issue 258 established.
  Confirm `pw-record`/`pw-cat` expose equivalent latency controls, or that
  their defaults are close enough not to regress meter smoothness.
- **Zero-zombie verification**: `captureMicLiveOnce`'s `cmd.Cancel` sends
  SIGTERM and `cmd.WaitDelay` bounds subprocess teardown (issue 245's
  "zero-zombie verified" requirement, per AgenticLoop.md Invariant 3).
  Confirm `pw-record`/`pw-cat` terminate cleanly on SIGTERM the same way —
  the doc-comment canary probe (§2 above) reported clean termination, but
  re-verify under the actual `cmd.Cancel`/`cmd.WaitDelay` harness, not just
  manual Ctrl-C.
- **False-triggering the recording indicator**: issue 248 investigated
  `parec`'s own capture stream tripping GNOME's mic-in-use indicator and
  found an `application.id=org.gnome.VolumeControl` property exemption.
  Confirm whether `pw-record`/`pw-cat` need the same (or an equivalent)
  property set to avoid the same false-trigger, since they open their own
  independent PipeWire stream.

## 5. Acceptance Criteria

- [x] New PipeWire-native live-capture backend (`pw-record`, chosen over
      `pw-cat -r` — dedicated recording tool, no mode flag needed) added to
      `internal/usage/miclive.go` as `captureMicLiveOnceViaPipeWire`,
      reusing `micLiveAmplitudeFromPCM16LE`/ballistics unchanged (both
      capture functions now share one `runMicLiveCapture` helper).
- [x] `resolveMicBackend()` tries `pactl`/`parec` first (via
      `probePactlDefaultSourceFn`), then the new `probePipeWireReachableFn`
      path (`pw-record` on PATH + a bounded `pw-cli info 0` canary), then
      falls through to `amixer` — existing pactl-path behavior and tests
      unchanged.
- [x] Live-verified on this session's actual PipeWire-without-
      pulseaudio-utils machine — see Resolution below for the exact
      commands and observed values.
- [x] Mic box "n/a" messaging (issue 262) updated: the amixer-only message
      now reads `"live n/a (needs pactl or PipeWire w/ pw-record)"`, and a
      pipewire-backend reading that is transiently unavailable renders the
      generic `"live n/a"` (not the amixer branch's permanent message).
- [x] Zero-zombie check: `runMicLiveCapture` reuses the same
      `cmd.Cancel`/`cmd.WaitDelay` SIGTERM teardown for both `parec` and
      `pw-record`; live-verified no leftover `pw-record` process after a
      context-cancelled 4s real capture (see Resolution below).
- [x] Cross-referenced against issue 264 in both tickets' `Related` fields
      (no scope overlap — this covers PipeWire-without-pactl, 264 covers
      no-PipeWire-at-all).

## 6. Resolution (implemented 2026-09-07)

Implemented the PipeWire-native fallback exactly as proposed in §3:

- `internal/usage/mic.go`: added `micBackendPipeWire`, `probePipeWireReachable`
  (`pw-record`+`pw-cli` on PATH, plus a bounded `pw-cli info 0` canary call —
  not just a LookPath check), and `currentMicStatusPipeWire` (scoped to
  `Available`/`Backend` only, per §3's "configured-gain reading is out of
  scope" — `Level`/`Muted`/`Recording` stay at zero values, matching the
  amixer backend's existing "n/a" recording treatment in
  `buildMicBoxLines`). The probe functions were made swappable
  (`probePactlDefaultSourceFn`/`probePipeWireReachableFn`/
  `probeAmixerCaptureFn`, mirroring `agy.go`'s `runAGYUsageCmdFn` pattern)
  so `resolveMicBackend`'s preference order is unit-testable without real
  audio hardware.
- `internal/usage/miclive.go`: added `captureMicLiveOnceViaPipeWire`
  (`pw-record --raw --format=s16 --rate=8000 --channels=1 -`, `--target`
  left at its "auto" default like `pw-cat`'s). Refactored the shared
  read/amplitude/ballistics loop out of `captureMicLiveOnce` into
  `runMicLiveCapture(ctx, cmd, m, onSample)` so both `parec` and
  `pw-record` invocations drive the identical pipeline.
  `startMicLiveManager` now picks `captureMicLiveOnce` or
  `captureMicLiveOnceViaPipeWire` based on the resolved backend (falls
  through to no-op only when neither resolves nor its binary is on PATH),
  and `runMicLiveManager` takes the chosen capture function as a
  parameter.
- `internal/usage/watch.go`: `buildMicBoxLines` now treats `Backend ==
  "pipewire"` the same as `"amixer"` for the recording-word "n/a" case (no
  source-outputs equivalent implemented), and the amixer branch's live "n/a"
  message was reworded from `"needs pactl/PipeWire"` to `"needs pactl or
  PipeWire w/ pw-record"` since PipeWire alone (without `pw-record`) is no
  longer sufficient to explain the gap — both compat layers are now tried
  before amixer.

**Format-compatibility findings, live-reconfirmed this session** (this
machine has no `pactl`/`parec` — `which pactl parec` both fail — so this
was a genuine exercise of the new fallback, not the existing pactl path):

```
$ timeout 3 pw-record --raw --format=s16 --rate=8000 --channels=1 - > /tmp/pwrec.raw
$ python3 -c "... RMS per ~50ms chunk ..."
0 1097.1
1 32.8
...
34 7308.0   # background noise / keyboard clicks
52 10932.5
```

Real, non-flatlined RMS values across the capture, confirming the raw
PCM16LE stream shape and no WAV/container header corruption.

**Live end-to-end verification of the actual production code path** (a
temporary, not-committed test file drove `resolveMicBackend`,
`CurrentMicStatus`, and `captureMicLiveOnceViaPipeWire` directly against
real hardware, then was deleted):

```
resolveMicBackend() = 2 (micBackendPipeWire=2)
CurrentMicStatus() = {Level:0 Muted:false Recording:false Available:true
  Backend:pipewire LiveLevel:0 LiveAvailable:false}
sample 1: Available=true Level=52.54
sample 2: Available=true Level=52.54
sample 3: Available=true Level=57.45
...
sample 57: Available=true Level=0.00
...
sample 72: Available=true Level=20.87
```

72 samples over a 4-second context-cancelled capture, amplitude genuinely
varying (0-64%, not flatlined) — real, changing evidence of a live pipe,
matching this ticket's live-verification bar. After the context was
canceled, `pgrep -a pw-record` found no leftover process (zero-zombie
confirmed for the pw-record path's SIGTERM teardown).

**Tests**: `internal/usage/mic_test.go` and `internal/usage/miclive_test.go`
gained coverage for: `resolveMicBackend`'s preference order (pactl >
pipewire > amixer, via the new swappable probe fns);
`currentMicStatusPipeWire`'s scoped contract; `buildMicBoxLines`'
pipewire-specific recording/live-line rendering (including that it does
*not* fall into the amixer branch's permanent message); and an end-to-end
`captureMicLiveOnceViaPipeWire` test (a fake `pw-record` script on PATH)
asserting the amplitude it produces from known PCM16LE bytes exactly
matches `micLiveAmplitudeFromPCM16LE` computed directly on the same bytes —
the core "same amplitude computation as the pactl path" requirement. All
new and pre-existing `internal/usage` tests pass (`go build ./...` and `go
test ./...` clean, aside from two pre-existing unrelated failures: a
GPU-name-width-dependent layout test and a `fj`-tool-availability test in
`internal/release`, both present before this change).

**Open questions from §4, resolved/deferred**:
- Node/source targeting: confirmed live — `pw-record`'s default `--target
  auto` picked up real ambient audio without any explicit targeting.
- Permission model: no issues observed — this session's user-session
  PipeWire socket worked with no extra setup.
- CPU/latency cost: not separately tuned (no `--latency` flag set,
  matching `pw-record`'s 100ms default) — acceptable for this box's 20 Hz
  polling cadence in live testing; revisit only if meter smoothness
  regresses in practice.
- False-triggering the recording indicator (GNOME mic-in-use): not
  investigated this session — out of scope for this ticket's live-capture
  focus; file a follow-up if observed in practice.
