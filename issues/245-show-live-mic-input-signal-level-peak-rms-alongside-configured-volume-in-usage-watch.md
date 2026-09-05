# 245 — Show Live Mic Input Signal Level (Peak/RMS) Alongside Configured Volume in `usage --watch`

**Status**: Closed — Added live peak/RMS mic meter (parec capture, RMS in-memory, zero-zombie verified) alongside 244's gain reading
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX / Agentic Ergonomics
**Related**: [[244-show-system-mic-level-and-recording-on-off-as-new-usage-watch-tui-box]] (shipped the
Mic box's first cut — `internal/usage/mic.go`, `MicStatus.Level`, `buildMicBoxLines` in
`internal/usage/watch.go`), `docs/practices/Canary.md`

---

## 1. Problem & Motivation

Ticket 244 shipped a `Mic` box in `harnez usage --watch` showing `MicStatus.Level` — but that field
is the default input source's **configured gain** (what `pactl get-source-volume` / `amixer get
Capture` reports, i.e. GNOME Settings' "Input Volume" slider), not the actual incoming signal level.
This was a known, documented tradeoff at ship time (see `MicStatus.Level`'s doc comment and 244's
Resolution §3), but user feedback after seeing it live confirms it isn't what was actually wanted:

> "This is the microphone level I'm talking of, not the one while I'm recording... the current
> Terminal UI, it's always at 100%, so we are... looking at the volume level that is set, but not
> the recorded level that is active, like the real noise."

Compared against GNOME's own Sound Settings panel, which shows two distinct things: a static
"Input Volume" slider (the gain — what 244 currently reads) and a live, constantly-moving level
meter bar reflecting the actual sound hitting the mic right now. The user wants **both** shown,
eventually: the existing gain reading plus a genuine live peak/RMS meter.

## 2. Technical Specification / Findings

244's Resolution section already identified why this is harder than a one-shot CLI poll: PipeWire/
PulseAudio only expose real peak/RMS through a **subscribed streaming API** — GNOME's meter works
by opening a monitor stream on the source and continuously reading buffer peak values, not by
polling a property. A one-shot `pactl`/`amixer` invocation per redraw tick (244's current cadence)
structurally cannot produce this; it would require either:

- Holding a long-lived low-rate capture/monitor stream open for the lifetime of `--watch --mic`
  (e.g. via `pw-cat`/`parec` reading raw samples from the source's `.monitor`, computing a rolling
  peak/RMS in Go, without ever writing captured audio to disk — this doesn't need to record
  anything, only measure amplitude), or
- Shelling out to a small existing meter utility if one exists and is commonly available (survey
  before choosing; none is known to already be used in this repo), or
- Using PipeWire's own client library/protocol directly instead of shelling out — likely overkill
  for "one focused box."

Whichever approach is chosen must be canary-probed for real before committing (`docs/practices/
Canary.md`) — capturing raw audio, even transiently and only in memory, is a materially different
risk/complexity class than the read-only property polls 244 used, and needs verification that it:
starts/stops cleanly with `--watch`'s lifecycle (no orphaned capture subprocess — see
`AgenticLoop.md`'s Zero Zombie Guarantee), doesn't require elevated permissions, and doesn't
meaningfully increase CPU/battery use just for a status box.

## 3. Desired End State

The Mic box eventually shows **both** values side by side — the existing configured-gain reading
(kept, it answers "is the mic turned up / muted") and a new live peak/RMS meter (answers "is sound
actually reaching the mic right now") — likely as two bars or a combined line, exact layout not
specified here.

## 4. Implementation & Verification Plan

Not yet planned — this is a materially bigger feature than 244 (a held-open audio stream vs. a
one-shot poll) and should get its own design/feasibility pass (read-only advisor dispatch per
`docs/practices/AgenticLoop.md`) to pick the capture mechanism before implementation. "Eventually"
per the user — no immediate deadline stated.

## 5. Resolution

Implemented as a new `internal/usage/miclive.go` (the streaming capture manager) plus extensions
to `MicStatus` (`internal/usage/mic.go`) and `buildMicBoxLines` (`internal/usage/watch.go`), wired
into `RunWatchWithOptions`'s existing lifecycle machinery the same way issue 110's
`runRemoteLoadManager` is.

**Canary probe** (2026-09-05, PipeWire 1.x via `pactl` 17.0, dev machine): both candidates named in
§2 were tried by hand before writing any Go.

- `parec --raw --format=s16le --rate=8000 --channels=1 -d @DEFAULT_SOURCE@` — started immediately,
  no `.monitor` suffix needed (`@DEFAULT_SOURCE@` is already an input source, not a sink; the
  `.monitor` convention only applies to sink-monitor sources). A `kill -TERM` on the pid produced a
  clean exit and a fully flushed output file within 0.5s.
- `pw-cat -r --target <source-id> --format s16 --rate 8000 --channels 1 -` — also worked and also
  terminated cleanly on SIGTERM, but needs a numeric target id/name resolved separately from
  `@DEFAULT_SOURCE@`.
- Decoding the captured raw PCM (Python, ad hoc) confirmed a genuine, continuously-varying
  amplitude signal (RMS ranging ~24-760, peak ~72-6261 across 100ms windows, ambient room/fan
  noise) — not a flat/silent stream and not a static value.

**`parec` was chosen** over `pw-cat`: it needs no extra name/id resolution (`@DEFAULT_SOURCE@`
already tracks the same symbolic default source `mic.go`'s existing `pactl` calls use), and it only
runs at all on the `pactl` backend `resolveMicBackend` selects — matching requirement 5's scope cut
(the plain-ALSA `amixer` fallback has no monitor-stream equivalent, so live level is simply `n/a`
there, mirroring `Recording`'s existing `n/a` treatment on that backend).

**Mechanism**: `startMicLiveManager` derives a child `context.Context` from the watch loop's own
`sigCtx` and, if capture is possible at all here, spawns one background goroutine
(`runMicLiveManager`) that holds one `parec` subprocess open, reads ~200ms raw PCM chunks
(`micLiveChunkBytes` = 3200 bytes @ 8kHz mono s16le), and computes RMS amplitude per chunk
(`micLiveAmplitudeFromPCM16LE`, linear ratio against int16 full-scale, 0-100 clamped) — a rolling
RMS reading of the real signal, updated in memory and immediately discarded, never written to disk.
The reading is published through a small mutex-guarded `micLiveMeter`, the same
one-writer/one-reader shape `runRemoteLoadManager`/`lastRemoteLoad` already uses for the Load box's
remote-streaming path. If the subprocess exits early (source unplugged, PipeWire restarted), the
manager retries every `micLiveRetryInterval` (5s) rather than giving up for the rest of the run.

**Lifecycle / Zero Zombie Guarantee**: `micLiveMgr` is a plain local var inside
`RunWatchWithOptions` (single-goroutine-owned, like `lastSummary`/`lastRates`), started lazily the
first time `draw()` sees the Mic box actually visible in local mode
(`activeSec.Mic && currentHost == ""`) and `Stop()`-ed the instant that stops being true — toggling
the box off with `[8]` kills the capture subprocess immediately, not just on quit. A top-level
`defer` also calls `Stop()` unconditionally so a Ctrl-C/SIGTERM mid-session cannot leave one
running even if the box was never toggled off first. `micLiveManager.Stop()` cancels the
capture's context; `exec.Cmd.Cancel` is overridden to send `SIGTERM` (Go's default is `SIGKILL`,
which doesn't give `parec` a chance to release the audio device), with a 2s `WaitDelay` as a
backstop.

**Live verification** (mandatory per this ticket, done for real, not assumed):
`go run ./scripts/canary-watch-pty 8 -- --mic` under a real pty showed the Mic box with both lines
— `[██████████] 100%   recording on` (unchanged 244 gain reading) and a second `live` bar/percent
line reading `0%` in a quiet room (correctly *not* pinned at 100% the way the gain reading is — the
whole point of this ticket) — plus, in a second run captured mid-flight, `pgrep -af 'parec|pw-cat'`
showed a real `parec --raw --format=s16le --rate=8000 --channels=1 -d @DEFAULT_SOURCE@` process
running as a child of the `harnez` watch process while the box was up, and the live line visibly
transitioning from the dim `n/a` placeholder to a real `0%`/`live` reading once the first chunk
arrived. After both runs completed (canary's internal timeout SIGTERMs the `script`-wrapped
process, exercising the same `sigCtx` cancellation path a user's Ctrl-C/`q` would), `ps aux | grep
-E 'parec|pw-cat'` and `pgrep -af 'parec|pw-cat'` showed **no matching process** — only the grep
command's own argv (containing the literal search string) matched, which is not a real hit. Zero
zombie processes confirmed twice, across two separate runs.

**Testing**: `micLiveAmplitudeFromPCM16LE` is unit-tested against synthetic PCM buffers (silence →
0, full-scale square wave → ~100, a quiet vs. loud synthetic sine wave preserving loudness
ordering, and an odd-trailing-byte edge case) in `internal/usage/miclive_test.go` — pure Go, no
subprocess. `buildMicBoxLines`' new second line is covered for both the `LiveAvailable: true` and
`false` cases in `internal/usage/mic_test.go`. Per this ticket's own instruction, the subprocess
start/read/kill lifecycle itself has **no fixture-based unit test** — a clean fixture for "a real
`parec` process talking to a real (or fake) PipeWire socket" isn't practical to build without
either a real audio backend in CI or a nontrivial PipeWire test double, and forcing one would
likely be flaky rather than meaningful. That lifecycle is covered instead by the live pty
verification above, which is the honest substitute this ticket asked for rather than a forced test.

**Limitations / open concerns**:
- The RMS-to-0-100 mapping is a rough linear ratio against int16 full-scale, not a calibrated dBFS
  meter — normal speech will read as a fairly low percentage relative to true full-scale noise.
  Good enough for "is sound reaching the mic right now", not for audio engineering; a future ticket
  could add a perceptual/log scale if the raw linear reading proves hard to read.
- Only verified against ambient room/fan noise on this dev machine, not against actual recorded
  speech — no realistic way to script "the user talks into the mic" for this session's
  verification; the amplitude-varies-with-real-input behavior is nonetheless the same mechanism
  GNOME's own meter and the raw-PCM canary probe both confirmed.
- No CI coverage for the process-lifecycle path (see Testing above) — regressions there would need
  to be caught by a future live check, not `go test ./...`.
