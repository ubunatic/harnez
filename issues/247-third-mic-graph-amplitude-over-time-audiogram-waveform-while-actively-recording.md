# 247 — Third Mic Graph: Amplitude-Over-Time Audiogram/Waveform While Actively Recording

**Status**: Open — filed via /issue
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: UX / Agentic Ergonomics
**Related**: [[244-show-system-mic-level-and-recording-on-off-as-new-usage-watch-tui-box]] (Mic box,
gain + mute + recording on/off), [[245-show-live-mic-input-signal-level-peak-rms-alongside-configured-volume-in-usage-watch]]
(live peak/RMS meter, `internal/usage/miclive.go`), `internal/usage/history.go` /
`internal/rograph` (existing sparkline/history-graph machinery already used for other usage
metrics in `harnez usage --watch` — likely reusable rendering primitives for this)

---

## 1. Problem & Motivation

Once 244 (configured volume) and 245 (live peak/RMS level) are both showing correctly, the user
wants a **third** element in the Mic box: while actively recording, a small timeline/audiogram
graph showing the amplitude signal over time (a short rolling window), not just the current
instantaneous level. This is explicitly sequenced *after* 244/245 are correct — 245's live-level
plumbing (the held-open `parec` capture + rolling RMS computation in `internal/usage/miclive.go`)
is the natural data source to sample into a rolling history buffer for this graph, rather than
building a second, separate capture path.

Note: this ticket depends on however ticket [[246]]'s research (GNOME's "no false recording
indicator" mechanism) resolves 245's side effect of triggering GNOME's system recording indicator
— whatever live-level capture mechanism harnez ends up using post-246 is what this graph should
sample from, so this ticket may need re-scoping once that's settled. Filed now to capture the
request; not blocking on 246 to exist as a ticket, but likely blocked on it for implementation
sequencing.

## 2. Desired Behavior (as requested, not yet fully scoped)

- Only shown while actively recording (per the existing `Recording`/`recording on` signal from 244)
  — matches the user's framing ("while we are actually recording, we want to see an audiogram").
- A small waveform/timeline-style graph — amplitude over a short recent time window, e.g. a
  sparkline-style rolling history similar to this repo's other usage-history graphs
  (`internal/rograph`), not a full scrolling oscilloscope.
- Positioned as a third piece of the Mic box, alongside the existing volume (244) and live level
  (245) readings.

## 3. Open Questions (exploratory — not yet resolved)

- Sampling source: reuse 245's existing RMS-per-chunk computation (already produces a numeric
  amplitude value at a steady interval) and feed it into a small ring buffer, rendered via whatever
  sparkline/graph primitive this repo already has for other metrics (check `internal/rograph` and
  `internal/usage/history.go` for the existing pattern before building a new one).
- Window length / resolution: not specified — needs a design pass, likely bounded by the Mic box's
  available width (same shrink-to-fit constraint other usage graphs already follow, e.g. issue 079).
- Whether this needs its own toggle or rides on the existing Mic box's `[8]`/`--mic` toggle from
  244 — simplest default is the latter (no new toggle), to be confirmed.

## 4. Implementation & Verification Plan

Not yet planned — sequencing depends on 246's findings (does the live-level capture mechanism
change?) and reuse of existing graph-rendering primitives rather than a bespoke waveform renderer.
