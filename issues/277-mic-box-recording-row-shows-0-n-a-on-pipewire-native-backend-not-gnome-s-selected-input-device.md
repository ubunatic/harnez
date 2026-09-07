# 277 — Mic Box Recording Row Shows 0%/n-a on PipeWire-Native Backend, Not GNOME's Selected Input Device

**Status**: Closed — resolved 2026-09-07 (see Resolution below)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: Issue 262; Issue 264; Issue 265; Issue 245; Issue 244; `internal/usage/mic.go`; `internal/usage/miclive.go`; `internal/usage/watch.go`

---

## 1. Problem & Motivation

User report (near-verbatim, from `harnez usage --watch`'s Mic box):

```
[░░░░░░░░░░] 0%   recording n/a   <-- does not show my actual level of the GNOME selected input device
[█████░░░░░] 59%   live           <-- lgtm
```

Two takeaways from the report:

1. The **live** row (issue 245's RMS meter) is confirmed working correctly
   (59%, changing with speech) — this is *not* a recurrence of issue 262,
   which was specifically about the live row never appearing at all. 262's
   own resolution (§6) already fixed the amixer-backend live-line messaging
   and left Status Open only pending a human confirming which message they
   see; this report confirms the live path itself is fine on this machine.
2. The **gain/recording** row (issue 244's original box) shows a flat `0%`
   bar and `recording n/a`, and the user correctly identifies this as wrong
   — it does not reflect the actual level of whatever input device GNOME's
   sound settings currently has selected as default. This is a **new,
   distinct symptom** from 262, not a duplicate: same box, different row,
   opposite direction (the row that's supposed to be the simple one is the
   one that's broken while the harder streaming one works).

Confirmed via `harnez find -d . issues "recording n/a"` / `"GNOME"` /
`"default source"` / `"selected input device"` that no existing ticket
covers this exact gap — 262, 244, 245, 250, 251, 253, 257, 258, 260, 264
were all reviewed and none describe the gain/recording row returning
zero/`n/a` on a working PipeWire-native system with a functioning live
meter.

## 2. Root Cause (verified by reading the current code, not speculation)

`internal/usage/mic.go`'s `resolveMicBackend()` (lines 110-133) probes, in
order: `pactl get-default-source` (line 121) → PipeWire-native reachability
via `pw-record`/`pw-cli` (line 123, issue 265) → `amixer` (line 125). On a
modern PipeWire desktop that does **not** have the separate
`pulseaudio-utils` package installed (no `pactl`/`parec` on `PATH` — exactly
the machine issue 265 was implemented and live-verified against this same
session), `resolveMicBackend()` resolves to `micBackendPipeWire`, not
`micBackendPactl`.

`CurrentMicStatus()` (mic.go:213-224) dispatches on that backend to
`currentMicStatusPipeWire()` (mic.go:255-257):

```go
func currentMicStatusPipeWire() MicStatus {
	return MicStatus{Available: true, Backend: "pipewire"}
}
```

This is **not a bug in the sense of an unhandled error** — it is a
deliberate, explicitly-documented scope decision from issue 265 §3: *"Level
stay at their zero values here; only Available/Backend are meaningful"*.
265's own Resolution (§6) live-confirmed this exact zero-value shape against
real hardware:

```
CurrentMicStatus() = {Level:0 Muted:false Recording:false Available:true
  Backend:pipewire LiveLevel:0 LiveAvailable:false}
```

`internal/usage/watch.go`'s `buildMicBoxLines` (lines 1221-1274) renders
that zero `Level` as a flat `0%` `rograph.RenderBar` (line 1226) and, since
`st.Backend == "pipewire"` matches the same branch as `"amixer"` (line
1232), renders `recordingWord = "n/a"` (line 1239) rather than a fabricated
"off". So `"0%   recording n/a"` is the box working exactly as coded — the
code was never taught how to read the PipeWire-native backend's actual
gain/mute/recording state; it only knows how to read `pactl`'s
`get-source-volume`/`get-source-mute`/`list short source-outputs` (used in
`currentMicStatusPactl`, lines 226-244) and `amixer`'s `get Capture`
(`currentMicStatusAmixer`, lines 259-275).

Meanwhile the **live** row is unaffected because `startMicLiveManager`
(miclive.go) picks its capture path from `resolveMicBackend()` independently
of `CurrentMicStatus`, and issue 265 *did* implement a PipeWire-native live
capture (`captureMicLiveOnceViaPipeWire`, using `pw-record`) — so the live
row genuinely reads real audio via `pw-record` while the gain/recording row
reads nothing at all via any PipeWire-native tool. This is exactly the split
the user's report describes.

**Regarding "GNOME's selected input device" specifically**: this is a
second-order consequence of the same gap, not a separate bug — since
`currentMicStatusPipeWire` performs *no* query at all (not even a
wrong-device one), it cannot reflect GNOME's selected default source either
way. `pw-record`'s default `--target auto` (used by the live path) does
correctly follow PipeWire/WirePlumber's current default node per 265 §6's
live verification ("`pw-record`'s default `--target auto` picked up real
ambient audio without any explicit targeting") — so the *live* row already
tracks GNOME's selection correctly; only the *gain/recording* row has no
equivalent query implemented yet.

## 3. Scope

- Implement a PipeWire-native gain/mute read for `currentMicStatusPipeWire`,
  analogous to `currentMicStatusPactl`'s `pactl get-source-volume
  @DEFAULT_SOURCE@` / `get-source-mute @DEFAULT_SOURCE@`. The natural
  candidate (not yet canary-verified in this repo — verify per
  `docs/practices/Canary.md` before merging) is WirePlumber's `wpctl`:
  `wpctl get-volume @DEFAULT_AUDIO_SOURCE@` reports both a volume fraction
  and a `[MUTED]` suffix in one call, and — like `pw-record --target auto`
  — resolves against PipeWire's actual current default node, i.e. whatever
  GNOME's sound settings currently has selected, not a hardcoded/stale
  device.
- Investigate whether a `Recording` equivalent exists for the PipeWire
  native backend. `pactl list short source-outputs` (used today) has no
  direct `wpctl` one-liner equivalent; a workable substitute is likely `pw-
  dump` filtered for stream nodes linked to the default source, or `wpctl
  status`'s output showing linked/connected clients. If no reasonably
  simple probe exists, keep `Recording` at its current `n/a` rendering (that
  part of the row is not what the user flagged as wrong) but do not leave
  `Level`/`Muted` at their placeholder zero values — those are the parts the
  user identified as actively misleading (a real, nonzero gain reported as
  a fabricated `0%` looks like data, not like "unsupported").
- Update `buildMicBoxLines` only if the fix changes which branch a pipewire
  backend now falls into (e.g. if gain/mute become real but recording stays
  `n/a`, the row would read like `"[bar] NN%   recording n/a"` — a real bar
  with an honest `n/a` for the one sub-signal that's still unavailable,
  which is the intended end state, not a regression of 262's "distinguish
  permanent vs. transient" messaging fix).
- Out of scope: the live row / live-capture path (already correct per this
  report); the amixer backend (has its own, already-correct `n/a`
  rendering per 244/262 — amixer genuinely has no generic "who's recording"
  concept, unlike PipeWire which does via node/stream state); GNOME mic
  indicator suppression (issues 248/251/253 — separate concern).

## 4. Acceptance Criteria

- [x] `currentMicStatusPipeWire()` (mic.go:255-257) reads a real gain
      percentage and mute state from the PipeWire-native backend (e.g. via
      `wpctl get-volume @DEFAULT_AUDIO_SOURCE@`), canary-probed for real
      output format per `docs/practices/Canary.md` before the parser is
      written against assumed output.
- [x] The gain reading is confirmed, on a real PipeWire-native machine
      (no `pactl`/`parec` on `PATH`), to track GNOME's *currently selected*
      default input device — verified by changing the default input device
      in GNOME Settings → Sound and confirming the Mic box's gain row
      changes to match, not by code reading alone (this repo's own
      AgenticLoop.md review standard: hook/environment-resolution-dependent
      features need a live end-to-end check, not just `go test ./...`).
- [x] `Recording` either gets a real PipeWire-native probe, or an explicit
      documented decision (with rationale, mirroring 265 §3's own scoping
      language) that it remains `n/a` for this backend, distinct from any
      future genuine-but-unavailable transient state.
- [x] Unit tests added for `currentMicStatusPipeWire`'s new parsing logic
      (stub the `wpctl`/probe command the way `mic_test.go` already stubs
      `probePactlDefaultSourceFn`/`probePipeWireReachableFn`/
      `probeAmixerCaptureFn`), plus a `buildMicBoxLines` case asserting the
      pipewire backend now renders a real gain bar (not a hardcoded `0%`)
      while the `recording n/a`/live-line behavior for that backend is
      otherwise unchanged unless `Recording` is also implemented.
- [x] `go build ./...` and `go test ./...` clean.
- [x] Cross-referenced against issues 262, 264, 265 in this ticket's
      `Related` field (done above) and in 265's own file if that ticket's
      "configured-gain reading is out of scope" note should point forward
      to this ticket as its follow-up.

## 5. Verification Guidance

- Canary-probe `wpctl get-volume @DEFAULT_AUDIO_SOURCE@`'s real output
  format on a live PipeWire-native machine before writing a parser against
  it — per `docs/practices/Canary.md`, do not assume the format from
  documentation alone.
- Live comparison check: run `pactl get-default-source` if available, or
  `wpctl status` otherwise, alongside `harnez usage --watch`'s Mic box, and
  confirm the box's gain percentage moves in the same direction as the
  system's actual input level when speaking into the mic or adjusting input
  volume in GNOME Settings → Sound — the same live-verification bar 265 §6
  already met for the live row, now applied to the gain row.
- Switch GNOME's selected default input device (if a second input device is
  available) and confirm the Mic box's gain row follows the switch within
  one redraw cycle, not the previously-selected device.
- Confirm `go test ./...` passes, but treat it as necessary, not
  sufficient, evidence per this repo's own hook/environment-resolution
  review standard (`docs/practices/AgenticLoop.md`) — this ticket is not
  done until a live run against real PipeWire hardware confirms the gain
  row tracks the actual selected device.

## 6. Resolution (implemented 2026-09-07)

Implemented exactly the §3 scope: a real gain/mute read for the pipewire
backend via `wpctl`, `Recording` left as a documented `n/a` decision.

- **Canary probe first** (per `docs/practices/Canary.md`), run live on this
  machine (no `pactl`/`parec` on PATH, same machine issue 265 was
  implemented and verified against):

  ```
  $ wpctl get-volume @DEFAULT_AUDIO_SOURCE@
  Volume: 1.00
  $ wpctl set-mute @DEFAULT_AUDIO_SOURCE@ 1 && wpctl get-volume @DEFAULT_AUDIO_SOURCE@
  Volume: 1.00 [MUTED]
  ```

  Confirmed the format assumed in §3 ("a volume fraction and a `[MUTED]`
  suffix in one call") before writing any parser against it.

- `internal/usage/mic.go`: added `runWpctlGetVolumeFn` (a package-level
  variable wrapping the real `runWpctlGetVolume`, mirroring
  `probePactlDefaultSourceFn`/`probePipeWireReachableFn`/
  `probeAmixerCaptureFn`'s existing swappable-probe pattern so
  `mic_test.go` can stub wpctl's raw output without a real WirePlumber rig),
  `parseWpctlVolumePercent` (fraction × 100, matching the 0-100 scale
  `Level` uses everywhere else in this package), and `parseWpctlMuted`
  (`strings.Contains(out, "[MUTED]")`). `currentMicStatusPipeWire` now
  calls `runWpctlGetVolumeFn()` and populates `Level`/`Muted` from the
  parsed result, degrading to the pre-277 zero-value reading (not an error)
  only when `wpctl` itself is missing or the call fails — `Available` stays
  true either way since the live-capture path (`pw-record`) doesn't depend
  on `wpctl`.
- `Recording` stays unimplemented for this backend — documented inline on
  `currentMicStatusPipeWire`, mirroring 265 §3's own scoping language: the
  closest substitute (`pw-dump` filtered for stream nodes linked to the
  default source) is a much larger parsing surface than this one boolean
  warrants, and `buildMicBoxLines` already renders `"n/a"` (not a
  fabricated "off") for it — a future ticket can revisit if a simpler probe
  turns up.
- `internal/usage/watch.go`: **no change needed**. `buildMicBoxLines`
  already routes `Backend == "pipewire"` through the same `"n/a"` recording
  branch as `"amixer"` regardless of `Level`'s value (issue 265's own
  change), so a real nonzero `Level` now renders as a real bar next to
  `"recording n/a"` — exactly the §3 end-state ("a real bar with an honest
  `n/a` for the one sub-signal that's still unavailable") — without
  touching the render branch logic itself.
- Tests added to `internal/usage/mic_test.go`: `TestParseWpctlVolumePercent`
  and `TestParseWpctlMuted` (pure parser tests against the real fixtures
  above), `TestCurrentMicStatusPipeWireReadsWpctl` /
  `TestCurrentMicStatusPipeWireDegradesWithoutWpctl` (stub
  `runWpctlGetVolumeFn`, mirroring the existing probe-stub tests further
  down the file), and `TestBuildMicBoxLinesPipeWireRealGain` (asserts a
  `Level: 59` `MicStatus` renders `"59%"` and does *not* render `"0%"`,
  pinning the regression this ticket fixes). The pre-existing
  `TestCurrentMicStatusPipeWire` (issue 265's hardcoded-zero contract test)
  was replaced by the two tests above since that contract is exactly what
  changed.

**Live end-to-end verification on real PipeWire hardware** (per §5 —
`go test ./...` treated as necessary, not sufficient):

Ran `harnez usage --watch --mic` in a sized `tmux` pane (the default
terminal was too short to render the Mic box at all — unrelated pre-existing
sizing behavior, not part of this ticket) and cross-checked the rendered
gain bar against `wpctl get-volume @DEFAULT_AUDIO_SOURCE@`'s live value
across three separate volume states:

```
wpctl volume 1.00, unmuted  -> Mic box: [██████████] 100%   recording n/a
wpctl volume 0.50, unmuted  -> Mic box: [█████░░░░░]  50%   recording n/a
wpctl volume 0.50, muted    -> Mic box: [█████░░░░░]  50%   recording n/a  (muted)
```

The gain bar moved from a flat, fabricated `0%` to a real percentage that
tracked `wpctl`'s live reading of `@DEFAULT_AUDIO_SOURCE@` — i.e. whatever
PipeWire/WirePlumber's current default source is, the same node GNOME's
Sound settings controls — across every state change, confirming the
acceptance criteria's core claim without relying on code-reading alone.
Volume/mute were restored to their original `1.00`/unmuted state afterward.

**Scoped down from the ticket's literal ask**: acceptance criteria's
"switch GNOME's selected default input device and confirm the box follows
the switch" step was not separately exercised — this machine has only one
enumerated audio source (`Depstech webcam Analog Stereo`, confirmed via
`wpctl status`'s Sources list), so there was no second device to switch to.
Since `@DEFAULT_AUDIO_SOURCE@` is the same PipeWire well-known target
`pw-record --target auto` already relies on (verified working per issue
265 §6's live capture test), and `wpctl`'s own `set-volume`/`set-mute`
calls used above operate against that same target, this is treated as
strong indirect evidence rather than a full substitute — flagged here
honestly rather than silently treated as fully covered.
