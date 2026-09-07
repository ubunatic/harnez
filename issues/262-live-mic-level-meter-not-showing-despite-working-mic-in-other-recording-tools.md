# 262 — Live mic level meter not showing despite working mic in other recording tools

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Bug
**Related**: Issue 244; Issue 245; Issue 250; Issue 251; Issue 253; Issue 257; Issue 258; Issue 260; `internal/usage/miclive.go`; `internal/usage/watch.go`; `internal/usage/mic.go`; `internal/usage/indicatorsspec.go`

---

## 1. Problem & Motivation

User report (verbatim): "the live MIC level is not shown currently even though
I can record fine in my rec tools." Recording working correctly in other
apps rules out the OS mic device, permissions, and hardware as the cause —
the gap is specific to harnez's live meter path.

`harnez usage --watch`'s Mic box has two separately-sourced readings per
`internal/usage/watch.go`'s `buildMicBoxLines`:
1. A configured-gain bar from `MicStatus.Level` (issue 244, polled via
   `pactl`/`amixer`).
2. A genuine live peak/RMS reading (issue 245, streamed via a background
   `parec` capture in `internal/usage/miclive.go`).

The user's complaint is specifically about the *live* reading (2), not
necessarily the configured-gain bar (1) — worth distinguishing when
reproducing.

## 2. What Was Verified By Reading The Code (Not Yet Confirmed Against The User's Machine)

Several independent gates all have to pass for the live meter to appear, and
any one of them failing silently would produce exactly this symptom (no live
level shown, no error surfaced) even though the underlying mic itself is
fine:

- **Backend gate (`internal/usage/miclive.go:207-222`,
  `startMicLiveManager`)**: the live capture is only ever started when
  `resolveMicBackend() == micBackendPactl`. The file's own top-of-file
  comment states this is deliberate — the amixer/plain-ALSA fallback path
  (issue 244) has no streaming-capable equivalent to `parec`, "never amixer."
  If the user's system resolves to the amixer backend (e.g. no PipeWire/
  PulseAudio, or `pactl` not on `PATH`, or `pactl get-default-source` fails
  — see `internal/usage/mic.go` around lines 117-140), the configured-gain
  bar can still work (via amixer) while the live reading is permanently
  `Available: false` with no distinguishing error message — it likely just
  renders as the "n/a"/unavailable placeholder `buildMicBoxLines` documents
  for this case (`internal/usage/watch.go` ~1206-1213).
- **Section/view gate (`internal/usage/watch.go:2720`)**: `wantMicLive :=
  activeSec.Mic && currentHost == ""`. The live manager is only started when
  the Mic section is active/toggled on *and* the view is the local host
  (`currentHost == ""`) — it is unconditionally suppressed when viewing a
  remote host's data, regardless of backend. If the user is watching a
  remote host, or has the Mic section toggled off, that alone would explain
  "not shown" with no bug in the capture path itself.
- **Availability gate (`internal/usage/watch.go` ~1911-1917)**: the whole Mic
  box (both readings) is only registered in the panel list when
  `sec.Mic && micStatus.Available` — if `CurrentMicStatus` fails to resolve
  either backend at all on the user's machine, the box does not render, not
  even as an "unavailable" placeholder inside a visible box.
- **Suppression/canary context (issues 248, 250, 251, 253)**: this repo has
  ongoing, not-fully-closed work on desktop mic-indicator suppression and a
  possible separate watcher/daemon architecture for the live stream (issue
  253, Open) — worth checking whether any of that is implicated, though
  nothing found so far suggests those changes broke the meter outright.

None of the above has been confirmed as *the* root cause on the user's
machine — this is a list of verified code-level gates that could each
independently produce the reported symptom, not a diagnosis. The actual audio
backend, terminal host (local vs. remote `--host`), and Mic section toggle
state on the user's setup are unknown and need to be gathered during
reproduction.

## 3. Scope

- Reproduce with the user's actual environment: which backend does
  `resolveMicBackend()` pick (`pactl` vs `amixer`) on their machine, is the
  Mic section (`sec.Mic`) toggled on, and are they viewing local or remote
  host data (`--host`)?
- If backend is amixer: decide and implement either (a) a documented,
  intentional limitation with a clearer in-box message (e.g. "live level
  needs pactl/PipeWire" instead of a bare "n/a"), or (b) investigate a
  streaming-capable amixer/ALSA equivalent to `parec` if one exists.
- If backend is pactl and it's still not showing: instrument/log why
  `startMicLiveManager`'s capture never reaches `Available: true` (e.g.
  `parec` invocation failing, permission issue distinct from the "recording
  works in other tools" observation, PipeWire proxy differences).
- Out of scope: chart background color defaults (tracked separately in issue
  261); new mic-related features beyond fixing/clarifying why the existing
  live meter doesn't display.

## 4. Acceptance Criteria

- [ ] Root cause identified and confirmed against the user's actual machine
      (backend, section toggle, host view) — not just inferred from code
      reading.
- [ ] Either the live meter is fixed to show for the user's backend/setup, or
      — if the amixer backend genuinely cannot stream live peak/RMS — the Mic
      box clearly communicates *why* (distinct message from generic
      "unavailable"/"n/a") instead of silently omitting the live line.
- [ ] Regression/unit test covering the specific gate that was found to be
      the cause (backend selection, section/host gating, or capture
      subprocess failure).
- [ ] `docs/MicIndicators.md` (issue 250) and/or this repo's mic-related
      tickets updated if the investigation surfaces a genuine implementation
      gap not already tracked by issues 244/245/250/251/253.

## 5. Verification Guidance

- Ask the user (or reproduce locally with an amixer-only environment, e.g. a
  VM/container without PipeWire) which backend `resolveMicBackend()` selects,
  and whether `sec.Mic` is toggled on and they are viewing the local host.
- Add temporary debug logging (or a `--debug`/existing debug overlay path —
  see `opt.DebugOverlay` in `watch.go`) around `startMicLiveManager`'s
  backend check and the `parec` subprocess start/exit to see exactly where
  the pipeline stops for the user's real setup.
- Live/manual end-to-end check per this repo's own conventions for
  hook/environment-resolution-dependent features: passing `go test ./...`
  is not sufficient evidence the live meter works in the real environment —
  confirm against an actual `harnez usage --watch` session showing changing
  levels while speaking into the mic.
