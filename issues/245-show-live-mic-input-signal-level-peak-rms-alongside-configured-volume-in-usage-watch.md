# 245 — Show Live Mic Input Signal Level (Peak/RMS) Alongside Configured Volume in `usage --watch`

**Status**: Open — user feedback after seeing 244 live: volume/gain isn't the same as real signal level
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
