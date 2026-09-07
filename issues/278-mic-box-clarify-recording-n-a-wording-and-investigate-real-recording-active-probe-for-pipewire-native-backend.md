# 278 — Mic Box: Clarify "recording n/a" Wording and Investigate Real Recording-Active Probe for PipeWire-Native Backend

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug / Feature
**Related**: Issue 277 (this ticket's direct follow-up); Issue 265; Issue 244;
Issue 262; `internal/usage/mic.go`; `internal/usage/watch.go`

---

## 1. Problem & Motivation

Direct follow-up to issue 277 (closed this session, commit `430cb62`), which
implemented a real gain/mute read for the PipeWire-native backend via
`wpctl get-volume @DEFAULT_AUDIO_SOURCE@` but explicitly left `Recording`
unimplemented/`false` for that backend — a documented scope decision, not an
oversight (see `internal/usage/mic.go:286-294`).

Two rounds of real user feedback on the resulting row, both from the same
session:

**Round 1** (verbatim):

```
[██████████] 100%   recording n/a   <-- this is the input-level slider, it moves when I move the system slider
[████░░░░░░] 43%   live             <-- this is the live noise
```

> "this is the input-level slider, it moves when I move the system slider"
> ... "But 'n/a' here makes no sense to me"

**Round 2** (earlier, same session):

> "it does not change when I am actually recording sth."

Read together, these are two distinct problems, not one:

1. The row's *wording* reads as if `n/a` qualifies the level/percentage
   itself ("we don't know the level"), when in fact the level is real and
   moving — `n/a` only applies to the separate, currently-unimplemented
   "is something actively capturing right now" sub-signal. This is
   confusing on its face, independent of whether the underlying capability
   ever gets implemented.
2. The user's second round of feedback is exactly the gap 277 already
   named and scoped out: there is currently no real recording-active
   detection for the PipeWire-native backend at all, so "it does not
   change when I am actually recording" is expected-per-277 behavior, not
   a regression — but it is a real capability gap a user hit twice.

## 2. Current Code (read, not speculated)

- `internal/usage/watch.go`, `buildMicBoxLines` (`internal/usage/watch.go:1221`):
  - Line 1226 renders the real gain bar via `rograph.RenderBar(st.Level, ...)`.
  - Lines 1228-1240: `recordingWord` defaults to `"off"`/`"on"` from
    `st.Recording`, but is force-overridden to `"n/a"` whenever
    `st.Backend == "amixer" || st.Backend == "pipewire"` (line 1232).
  - Line 1242: `line := fmt.Sprintf("%s   recording %s", bar, recordingWord)`
    — this is the exact line producing `"[bar] NN%   recording n/a"`, with
    the real, moving percentage immediately adjacent to the unqualified
    `"n/a"` that both feedback rounds flagged as confusing.
- `internal/usage/mic.go`, `currentMicStatusPipeWire` (`internal/usage/mic.go:263-312`):
  the doc comment (295-312) already states the rationale for leaving
  `Recording` unimplemented: `pactl list short source-outputs` (used by
  `currentMicStatusPactl`, mic.go:243-261, specifically line 257-258) has
  no direct one-shot `wpctl` equivalent; the closest substitute would be
  filtering `pw-dump`'s node graph for stream nodes linked to the default
  source — judged too large a parsing surface for issue 265/277's scope.
  `currentMicStatusPactl` (mic.go:257-258) is the reference implementation
  to mirror in shape (not literally reusable) if Part 2 below is picked up:

  ```go
  if soOut, err := exec.Command("pactl", "list", "short", "source-outputs").Output(); err == nil {
      st.Recording = strings.TrimSpace(string(soOut)) != ""
  }
  ```

## 3. Scope — Two Separately Doable Parts

### Part 1 — Labeling/wording fix (small, does not require any new probe)

Fix the row's wording so `"n/a"` cannot be read as qualifying the level
bar itself. Concretely, in `buildMicBoxLines` (watch.go:1221-1242):

- Make clear `"n/a"` means "recording-active detection not available on
  this backend", not "we don't know the level" — the level is right there
  on the same line, clearly not `n/a`. Options include (implementer's
  judgment, not prescribed): moving the `n/a` sub-field to its own
  qualifier (e.g. `"recording: unsupported"` or a distinguishing marker),
  adding a one-time hint/legend, or restructuring the line so the two
  sub-signals (level vs. recording-active) are visually separated rather
  than reading as one clause.
- Separately reconsider whether the word **"recording"** is even the right
  label for what is actually an input-level/gain bar (issue 277 itself
  calls this "the input-level slider" per the user's own words) —
  independent of the trailing recording-active sub-field's wording. The
  bar is `st.Level` (configured gain/volume, from `wpctl`/`pactl`), not a
  recording-in-progress indicator; the current line conflates both
  concepts under one "recording" label.
- This part should stand alone and ship regardless of whether Part 2 below
  is ever implemented — do not block it on Part 2.

### Part 2 — Real recording-active probe for PipeWire-native backend (larger, may prove infeasible)

Close the gap issue 277 explicitly deferred: implement a real
recording-active read in `currentMicStatusPipeWire`
(`internal/usage/mic.go:263-312`), replacing the current hardcoded
`st.Recording` (always `false`, never set) with an actual probe.

Candidate mechanisms to investigate (do **not** implement or probe these
live as part of filing this ticket — this is implementation guidance for
whoever picks up Part 2):

- `wpctl status` — shows linked/connected stream clients under the
  default source in its tree output; may be parseable for "is anything
  currently linked to the default source" without needing full `pw-dump`.
- `pw-dump` filtered for stream nodes linked to the default source node —
  the substitute issue 277 itself named as the likely path, at the cost of
  a larger parsing surface (a full JSON node-graph dump vs. a one-line
  `wpctl get-volume` call).

Per `docs/practices/Canary.md` (the same discipline issue 277 followed for
`wpctl get-volume`), whoever implements this **must** canary-probe the
chosen mechanism live on real hardware before writing a parser against
assumed output — do not assume `wpctl status`'s or `pw-dump`'s format from
documentation alone.

If no simple, low-parsing-surface probe exists after genuine investigation,
that is itself a valid, documented outcome — mirror issue 277/265's own
scoping language rather than forcing a fragile implementation. In that case,
Part 1's wording fix should stand as the ticket's full resolution and Part 2
should be explicitly re-scoped out with rationale, the way 277 did for this
same gap.

## 4. Acceptance Criteria

**Part 1**:
- [ ] `buildMicBoxLines`'s rendering no longer reads as if `"n/a"`
      qualifies the level percentage/bar itself; a person unfamiliar with
      the code, looking only at the rendered row, can tell the level is a
      real reading and `n/a` refers to a separate, currently-unavailable
      "recording active" signal.
- [ ] The "recording" label vs. "input level/gain" label question (per
      §3 Part 1) is explicitly decided and reflected in the wording, not
      left as-is by default.
- [ ] Existing `buildMicBoxLines` tests updated/extended to assert the new
      wording; no unrelated rendering behavior (bar width, muted suffix,
      live line) regresses.
- [ ] `go build ./...` and `go test ./...` clean.

**Part 2** (if implemented; may be scoped out instead per §3):
- [ ] `currentMicStatusPipeWire` sets `st.Recording` from a real probe
      (`wpctl status` and/or `pw-dump`), canary-probed live per
      `docs/practices/Canary.md` before the parser is written.
- [ ] Unit tests added mirroring `mic_test.go`'s existing
      `probePactlDefaultSourceFn`/`probePipeWireReachableFn`/
      `probeAmixerCaptureFn`/`runWpctlGetVolumeFn` stub pattern for
      whichever new probe function is introduced.
- [ ] `buildMicBoxLines`'s pipewire branch (watch.go:1232) updated so a
      real `Recording` value renders `"on"`/`"off"` instead of the
      forced `"n/a"`, without breaking the amixer backend's still-`n/a`
      case (amixer genuinely has no equivalent concept — out of scope,
      per 277 §3).
- [ ] `go build ./...` and `go test ./...` clean.

## 5. Verification Guidance

- **Part 1**: no code test needed beyond the updated unit test assertions —
  confirm by inspection that the rendered row wording reads unambiguously
  (does the "n/a" now clearly attach to a distinct sub-signal, not the
  level?).
- **Part 2** (if implemented): a live end-to-end check is required, not
  just `go test ./...` passing, per this repo's own
  `docs/practices/AgenticLoop.md` hook/environment-resolution review
  standard — confirm the row's recording indicator actually changes when
  something is genuinely capturing from the mic (e.g. run `arecord` or
  `ffmpeg` against the default source, or open an app that opens the mic)
  versus when nothing is, on a real PipeWire-native machine (no
  `pactl`/`parec` on PATH, same class of machine issue 265/277 verified
  against).

## 6. Out of Scope

- The **live** row / live-capture path (issue 245/265's RMS meter) — not
  implicated by either round of feedback quoted above.
- The amixer backend's `Recording` handling — already correctly documented
  as a permanent structural limitation (244/262), not touched here.
- GNOME mic indicator suppression (issues 248/251/253) — separate concern.
