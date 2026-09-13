# 330 — Adopt voxi `audiolevel`'s Rolling Braille Timeline Alongside the Existing Live Mic-Level Bar

**Status:** Open
**Category:** Usage TUI / Mic Indicators
**Related:** [internal/usage/miclive.go](../internal/usage/miclive.go), [internal/usage/mic.go](../internal/usage/mic.go), [internal/usage/watch.go](../internal/usage/watch.go) (`st.Level`/`st.LiveLevel` bar rendering around line 1303/1333), `ubunatic.com/voxi/audiolevel` (`Meter.EnableSparkline`, `Manager.Sparkline`, `SparklineStream`), voxi issues 109 (added the export) and 110 (`examples/miclevel`, the loom TUI demo that prompted this ticket)

---

## 1. Problem & Context

`internal/usage/miclive.go` already imports `ubunatic.com/voxi/audiolevel`
for harnez's live mic-level meter (moved there specifically so voxi and
harnez share one capture/RMS/ballistics pipeline instead of harnez keeping
a private copy — see that file's own doc comment). `watch.go` renders the
result as a scalar bar only (`rograph.RenderBar(st.Level, ...)` /
`st.LiveLevel` around lines 1303 and 1333) — a instantaneous level, no
history.

voxi's `audiolevel` package has since grown a second capability on the same
`Meter`/`Manager` this file already drives: `EnableSparkline` +
`Manager.Sparkline()` (voxi issue 109), a rolling, continuously-scrolling
Braille time-chart fed by the same capture stream at effectively zero
extra cost (`RunCapture` already calls `WritePCM` with every chunk it reads
for the sparkline once enabled — no second subprocess, no second capture
stream). voxi's `examples/miclevel` (issue 110) demonstrates this live
against a real mic: a scalar bar plus a scrolling waveform-like Braille
timeline underneath it, both from the one `Manager`.

harnez already has its own independent Braille sparkline renderer in this
same package (`renderSparkline`/`watchPercentSparkline` in `watch.go`,
used for agent activity rate and CPU/GPU/load history) — so the TUI
plumbing for showing "a Braille chart next to a labeled row" already
exists here. This ticket is specifically about wiring voxi's
*mic-level-specific* rolling timeline (tuned window/floor/ceiling constants
already calibrated for speech, not harnez's general-purpose
percent-history sparkline) into the mic display, not about building new
chart-rendering infrastructure.

## 2. Proposed Solution

- In `startMicLiveManager` (or wherever the `audiolevel.Manager` is
  constructed in `miclive.go`), call `Manager.EnableSparkline` once at
  startup with `audiolevel.SparklineOptions{SampleRate: ..., Width: ...,
  Window: ...}` — `SampleRate` must match whatever
  `ParecCommand`/`PwRecordCommand` sample rate this file's
  `resolveMicBackend`/capture wiring actually uses (see voxi's own
  `SparklineOptions.SampleRate` doc comment: a mismatch silently skews the
  window's displayed time span).
- In `watch.go`, alongside the existing `st.Level`/`st.LiveLevel` bar
  rendering, add `Manager.Sparkline()`'s output as a second row/segment in
  the Mic box — matching the bar-plus-timeline layout voxi's
  `examples/miclevel` already demonstrates, adapted to harnez's own box/row
  conventions rather than copied verbatim.
- **Version bump required first**: harnez's `go.mod` currently pins
  `ubunatic.com/voxi v0.1.2`, tagged well before voxi issue 109's sparkline
  export landed (confirmed: v0.1.7, harnez's latest available tag as of
  this writing, still predates that commit). Per
  `docs/practices/GoRelease.md`'s sibling-module convention: voxi needs a
  new tagged release including the sparkline export before harnez can
  `go get ubunatic.com/voxi@vX.Y.Z` to pick it up — this ticket is blocked
  on that release existing, not just on the code already being on voxi's
  main branch.

## 3. Open Questions

- Width/window tuning: voxi's `examples/miclevel` uses `Width: 32`,
  `Window: 3*time.Second` for a dedicated single-purpose pane. Harnez's Mic
  box shares screen space with other status rows — decide the right
  width/window for that layout rather than reusing voxi's example values
  as-is.
- Placement: a second row under the existing bar (voxi's layout), or
  inline after the percentage (matching how `stats.Sparkline` is appended
  in brackets elsewhere in `watch.go`, e.g. line ~1380)? Whichever reads
  better against harnez's existing Mic box density.
- Should the general-purpose `renderSparkline`/`watchPercentSparkline`
  machinery already in `watch.go` be reused to *format* the mic timeline
  (if it can accept a pre-rendered Braille string rather than a `[]float64`
  history), or does `Manager.Sparkline()`'s already-rendered string get
  written directly, bypassing that machinery entirely? Check before adding
  a second, parallel sparkline-rendering path if one isn't needed.

## 4. Verification Plan

Automated: `go build`/`go vet`/`go test` stay green with the new row wired
in. Manual (same category as voxi issue 110's own verification — not
something a headless test can assert): run harnez's live watch mode against
a real microphone, confirm the new timeline row visibly responds to speech
and decays afterward, and confirm the degraded/no-backend state (no
`parec`/`pw-record`) still renders without a panic or a stuck placeholder.
