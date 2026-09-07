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
      reading. **Not done** — no access to the user's machine or any real
      mic hardware in this sandboxed environment; see §6 for what was
      verified from code alone and what still needs the user.
- [x] Either the live meter is fixed to show for the user's backend/setup, or
      — if the amixer backend genuinely cannot stream live peak/RMS — the Mic
      box clearly communicates *why* (distinct message from generic
      "unavailable"/"n/a") instead of silently omitting the live line.
- [x] Regression/unit test covering the specific gate that was found to be
      the cause (backend selection, section/host gating, or capture
      subprocess failure).
- [ ] `docs/MicIndicators.md` (issue 250) and/or this repo's mic-related
      tickets updated if the investigation surfaces a genuine implementation
      gap not already tracked by issues 244/245/250/251/253. **N/A** — no
      new implementation gap was found beyond the messaging fix in §6, which
      is fully described here.

## 6. Resolution (investigation against the three candidate gates)

Re-read `miclive.go`, `mic.go`, `watch.go`, and `indicatorsspec.go` in full
against the three candidate gates from §2. Two of the three are working as
designed and were left untouched; the third gate (amixer backend) is a real,
already-documented limitation whose *messaging* was the actual bug — fixed.

1. **Backend gate (`startMicLiveManager`, miclive.go:219-230)** — confirmed:
   live capture is started only when `resolveMicBackend() == micBackendPactl`
   and `parec` is on `PATH`; the amixer path returns a manager that never
   spawns anything, matching the file's own doc comment. This is deliberate
   (issue 244/245: plain ALSA has no `parec`-equivalent streaming API) —
   **not a bug**, but its silent "n/a" was indistinguishable from a
   transient pactl reconnect, which *is* the bug (see below).
2. **Section/host gate (`watch.go:2720`,
   `wantMicLive := activeSec.Mic && currentHost == ""`)** — confirmed real,
   and confirmed **not a bug**: `buildWatchFrameAt` (watch.go ~1861-1870)
   already gates the whole `micStatus` fetch on `opt.Host == ""` for the
   same reason (a remote host's audio device isn't observable over the
   existing `--host` snapshot machinery) — the live-capture gate is
   consistent with that existing, intentional local-only design, not a
   separate bug.
3. **Availability gate (`watch.go:1913`,
   `sec.Mic && micStatus.Available`)** — confirmed **not a bug**:
   `CurrentMicStatus()` (mic.go:147-156) sets `Available: true` for *either*
   backend that resolves (pactl or amixer) — `resolveMicBackend()` only
   returns `micBackendNone` (which hides the whole box) when neither `pactl`
   nor `amixer` produced anything. An amixer-only machine still shows the
   Mic box with a working configured-gain bar; only the live sub-line is
   affected, per gate 1.

**Root cause of the reported UX gap**: on an amixer-only system (no
PipeWire/PulseAudio), the live line correctly can never populate (gate 1,
by design) but rendered the exact same dim `"live n/a"` placeholder as a
pactl system's transient "still (re)connecting" state — a user with a
working mic and a working configured-gain bar had no way to tell "this will
never work here" from "this is about to start working." Fixed in
`buildMicBoxLines` (watch.go): the live line now renders
`"live n/a (needs pactl/PipeWire)"` specifically when `st.Backend ==
"amixer"`, leaving the plain `"live n/a"` for the pactl-but-not-yet-flowing
case. Added `TestBuildMicBoxLinesLiveUnavailableAmixerExplainsWhy` asserting
both branches (mic_test.go).

**What could not be confirmed in this sandboxed environment** (no real audio
hardware, no access to the user's machine): which backend
`resolveMicBackend()` actually picks for the user, whether their Mic section
is toggled on, and whether they were viewing a remote host — i.e. whether
the reported symptom is in fact the amixer case fixed here, or something
else not yet identified. `go test ./...` passing (see commit) is not
sufficient evidence for a hook/environment-resolution-dependent feature like
this per `docs/AgenticLoop.md`'s review standard — a human should run
`harnez usage --watch` for real and report back which of the two live-line
messages they see, and whether it now matches their actual backend.
Leaving Status **Open** pending that live confirmation; reopen/adjust scope
if it turns out their symptom isn't the amixer case.

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
