<!-- harnez:topic: Fleet-wide 24h survey (2026-09-03→04) across harnez, lmcoder, ubunatic.com, voxi, uman, psync — 95 commits, ~15k line delta, six independently-driven sprints observed from git history rather than a single session -->
# Fleet-Wide 24h Cross-Project Survey (2026-09-03 → 2026-09-04)

**Scope**: every project under `~/projects/` with commit activity in the trailing 24
hours, **excluding `smarthome`** (a separate agent is writing its own dedicated story
there; that project's slice is left as a placeholder below, to be blended in once that
story lands). Six repos had activity: `harnez`, `lmcoder`, `ubunatic.com`, `voxi`,
`uman`, `psync`.

This is not a single session's retrospective — it's a **meta-survey** assembled after
the fact from `git log`/`git diff --stat` across repos that were each worked by their
own (mostly separate) agent sessions. That changes what candor can mean here: I can
report what the commit history and issue trackers show, but I wasn't present for the
false starts, dead ends, or in-session corrections that never made it into a commit
message. Where a commit message itself documents a bug or regression, that's reported
as ground truth; where I'd otherwise be reconstructing an agent's internal experience,
I say so explicitly instead of inventing texture.

## 1. Header & Context

- **Window**: 2026-09-03 ~02:00 → 2026-09-04 (git `--since="24 hours ago"` at time of
  writing).
- **Repos surveyed**: `harnez` (63 commits), `ubunatic.com` (15), `lmcoder` (10),
  `voxi` (4), `uman` (2), `psync` (1) — 95 commits total, none excluded for the survey
  other than `smarthome` per instruction.
- **Repos with git history but zero activity in window**: `books`, `business`, `cati`,
  `comfyconf`, `conreel`, `diagnose`, `emojig`, `fontwidth`, `goha`, `harnez.org`,
  `homeserver`, `loom`, `mdview`, `pdf-doctor`, `proctop`, `spriteview`, `trafficsim`,
  `uzu`, `venile.de`, `videos`, `vimconf`, `wayreel`, `webman`, `ziggo`, `zterm`.
- **Working-tree state**: all six active repos are clean (`git status --porcelain`
  empty) — nothing was left uncommitted at the time of this survey, consistent with
  the standing rule to always commit completed ticket work.
- **smarthome**: excluded from this document by request. 55 commits landed there in
  the same window (highest single-repo count of any repo, including the ones covered
  here) — placeholder only; content to be merged in once that project's own story is
  done.

## 2. Executive Summary

Six repos shipped 95 commits and roughly +22,800/−1,692 lines in a single day, spread
across two tightly related workstreams (`harnez` observability/telemetry work feeding
`ubunatic.com`'s public dashboard) and three loosely-coupled feature pushes
(`lmcoder` GPU memory tuning, `voxi` streaming ASR playback, `uman` CLI polish) plus
one small maintenance ticket (`psync`).

The dominant pattern across the fleet is **ticket-first, small-commit discipline**:
almost every feature commit is paired with a `tracker:`/`docs(issue-N)` commit either
just before (spec) or just after (close-out/follow-up), and every repo's `issues/`
tree stayed in sync with `README.md` via `harnez index`. The `harnez` repo itself
supplied the day's biggest single thread — a telemetry classifier that got rebuilt
three times in one window (cloud CLI → local SLM → session-based) chasing real bugs
found only by running it live, which then fed directly into `ubunatic.com`'s new
`/telemetry` dashboard and a zero-prose privacy default.

## 3. What Worked Well

- **Issue-driven traceability at scale**: 30+ of the 95 commits are pure
  tracker/docs commits (file ticket → do work → close/follow-up). This makes the
  `harnez find` / `issues/README.md` index a reliable table of contents for the day
  even across repos, which is exactly what this survey leaned on instead of re-reading
  every diff.
- **Cross-repo pipeline discipline**: `harnez`'s telemetry classifier
  (`internal/telemetry/classify.go`) and `lmcoder`'s local-model serving work landed
  first, and `ubunatic.com`'s `/telemetry` dashboard consumed the sanitized export as
  a downstream client rather than duplicating logic — see harnez issues 204/212/216
  and `ubunatic.com` issues 029/030.
- **Fast follow-up on self-discovered bugs**: `harnez` issue 218 (usage-bar quota
  columns) went from regression report → fix → close in the same window (`f2c13d6` →
  `e232aee`/`bf51b25`/`f0f5cef` → `b6dc46f`), and issue 210 (Braille load-chart
  visibility) similarly closed same-day.
- **Deliberate spec staging before code** in `lmcoder`: `fa2aaf8` (file tickets
  086/087) preceded `45ef5ec` (implement `machine status`/`machine setup`), and the
  GTT tuning work was refined twice more same-day (`6733887`, `9ca7da6`, `3b7434d`)
  ending in a documentation commit (`44c7d42`) capturing the tuning-tier rationale —
  spec, build, harden, document, in that order.
- **`voxi`'s intermediate-review commit** (`98dcfe5`, "record intermediate
  implementation review") between initial feature work and the larger follow-up
  commit is a good pattern: a checkpoint written mid-flight rather than only at the
  end, so context isn't lost if the session had been interrupted.

## 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)

Everything below is drawn from commit messages that self-report a bug, regression, or
false start — not inferred:

- **`harnez` telemetry classifier churned through three architectures in one day**
  and two of the three had live bugs the initial implementation missed:
  - `52afd33` switched the classifier from the cloud Claude CLI to a local `lmcoder`
    endpoint (issue 216).
  - `0e093ce` — "fix classifier hang/silent-failure bugs found running --classify
    live" — the switch above shipped with hangs and silent failures that unit tests
    did not catch; only caught by actually running `--classify` against live data.
  - `efbeb8c` then had to switch the default model again (to `qwen3-4b`) and fix a
    **stale production timeout** value.
  - `0ddec50` reworked the whole thing a third time, from batched JSON arrays to one
    growing session — a structural rewrite, not just a bug fix, suggesting the
    original batching design was the wrong shape for the problem from the start.
  - Net effect: three real bugs (hang, silent failure, stale timeout) shipped past
    whatever test suite existed and were only caught by live runs — the same failure
    pattern flagged in the 2026-08-31
    `harnez-tool-observability-and-the-real-environment-verification-gap` study
    ("green tests, non-functional in real usage") recurred here for a different
    subsystem.
- **`harnez` usage-bar layout regressed twice in the same window**: issue 218
  (`f2c13d6`) reports quota-column and remote-chart-history regressions that needed
  three separate fixes (`e232aee` stabilize at 100%, `bf51b25` size for real
  durations, `f0f5cef` pad remote histories) before close-out. Issue 210
  (`49bda3d`) separately reports the Braille load baseline going invisible,
  fixed in `3978cac`. Two independent visual-regression tickets in one day in the
  same rendering subsystem (`internal/usage`, `rograph`) — no automated visual
  regression check caught either; both were user- or live-observation-driven.
- **`harnez` issue 226**, filed same day, is itself a **meta-bug in the telemetry
  system that this survey's own protocol depends on**: intentionally-failed shell
  calls (e.g. a test deliberately checking a command fails) were polluting the
  unrated-failure counter used by the Tool Feedback Protocol nag. Fixed same day
  (`6446341`), but worth flagging here since this very session was nagged by that
  counter mid-survey.
- **`harnez` issue 202**: `find issues next --reserve` could print a placeholder
  filename that diverges from the slug a caller derives by hand from the title —
  a footgun for exactly the ticket-filing workflow this whole fleet relies on.
  Fixed (`c4d6a67`) by having the command print the exact reserved path instead of
  letting callers re-derive it.
- **`lmcoder` GTT/kernel-parameter handling needed a correctness fix mid-stream**:
  `ee08885` — "emit exclusively `ttm.pages_limit` to prevent kernel parameter
  conflicts" — landed immediately after `6733887` added support for the modern
  parameter *alongside* the deprecated `amdgpu.gttsize`; emitting both at once was
  wrong and had to be corrected the same day.
- **No cross-project incident this survey could find beyond the above** — the fleet's
  own issue trackers are the record, and every self-reported regression above was
  closed same-day with a paired ticket. The absence of unresolved red flags in six
  repos' worth of history is itself worth treating skeptically: this survey did not
  run any test suite itself, so "all green" here means "all green in the commit
  messages," not independently re-verified.

## 5. Quality & Invariants Audit

| Dimension | Observation |
|---|---|
| **Architecture & module separation** | Held up under scrutiny where visible: `harnez` kept `internal/privacy` free of `internal/telemetry`/`internal/usage` imports (per the 2026-09-03 study), and the classifier rewrite (`0ddec50`) stayed inside `internal/telemetry` without leaking into callers. Cannot verify `lmcoder`/`voxi` internals without a deeper read — not attempted for this survey. |
| **Idempotency** | Not directly testable from git history alone. Indirect evidence: `harnez index` was re-run after every ticket-filing commit and produced only the expected `issues/README.md` diff each time (no spurious churn visible in the stat output), which is the idempotency signature you'd want. |
| **Backward compatibility** | `lmcoder`'s `6733887`→`ee08885` pair is a compatibility-relevant case: it explicitly supports the modern `ttm.pages_limit` kernel param *alongside* the deprecated `amdgpu.gttsize` rather than a hard cutover, then had to correct the *emission* logic (not the detection logic) to avoid conflicts — i.e. backward-compat intent was right, first implementation of it was not. |
| **Test coverage & verification** | Every `harnez` and `lmcoder` feature commit in this window carries a paired `_test.go` file (e.g. `exec_test.go`, `classify_test.go`, `machine_test.go`, `indicatorsspec_test.go`). `voxi` likewise ships `_test.go` alongside `chunks.go`, `command.go`, `feedback.go`. This survey did not execute any of these suites — coverage presence was confirmed by file listing, not by running `go test ./...`, so "tests exist" is verified but "tests pass right now" is not. |
| **Docs/ticket sync** | Strong: `issues/README.md` was updated in the same or an adjacent commit for essentially every ticket touched, across all six repos. |

## 6. Efficiency & Velocity Assessment

- **95 commits / ~24h across 6 repos** ≈ 4 commits/hour fleet-wide, but this was
  almost certainly several concurrent sessions rather than one continuous thread —
  `harnez` alone produced 63 commits, which at any serial human-paced review rate
  would not fit in 24h alongside five other repos' worth of equally detailed work.
  This is circumstantial evidence for the multi-agent/parallel-session model this
  workspace's docs already assume, not a new finding.
  Global feedback memory [[feedback_no_parallel_agents]] says *this* orchestrating
  session should not spawn parallel agents itself — it says nothing about whether
  separate independently-launched sessions across repos ran in parallel, and the
  git timestamps are consistent with that being the case.
- **`harnez`'s 63 commits in one repo in one day** is the highest velocity in the
  survey and matches the pattern already documented in the 2026-08-29 "A Day of Fresh
  Sprints" study — small, ticket-scoped commits landing continuously rather than a
  few large ones.
- **Rework cost was real but bounded**: the telemetry classifier's three-architecture
  churn (§4) cost extra commits but each iteration was smaller than the last
  (104 → 244 → mixed lines changed) and every iteration shipped with tests, so the
  rework was visible and reviewable rather than silently discarded.
- **`ubunatic.com`'s 15 commits** show a full feature-to-ship arc in one day: spec
  tickets (029/030) → dashboard feature (`f1545d2`, `cc5a171`) → data-origin docs
  (`74e36ea`) → tooling swap from Python to Go (`1bdfc92`, mirrored by a
  Go-promotion ticket filed in three separate repos — `harnez` 221, `ubunatic.com`
  031, and implicitly `lmcoder`'s Go-only tooling) → privacy close-out (`ac85ffb`) →
  story write-up (`7630df0`). That's a same-day idea-to-retrospective loop.

## 7. Key Learnings & Evergreen Upstream

- **The "green tests, live-only bug" pattern is now a repeated finding, not a one-off.**
  The 2026-08-31 `harnez-tool-observability` study found it for capture hooks; this
  window's telemetry classifier rewrite (`0e093ce`) found it again for a network
  client with hang/silent-failure modes. Candidate evergreen rule: **any component
  that talks to an external process or endpoint (LLM CLI, local model server, IPC)
  needs at least one test that exercises the real transport, not just a mocked
  interface** — mirrors the `[[feedback_no_worktrees]]`-style lesson that mocked
  paths and real paths diverge in ways unit tests alone don't catch.
- **A cross-repo initiative ("promote Go, demote Python for scripts/tooling") was
  filed as three separate near-duplicate tickets** (`harnez` 221, `ubunatic.com`
  031, implied in `lmcoder`) rather than one tracked decision with per-repo
  follow-ups. Worth raising with the user: should recurring cross-repo policy
  decisions get a single home (e.g. a `harnez` or workspace-level ADR) that other
  repos' tickets reference, instead of re-litigating the same rationale per repo?
- **Visual/layout regressions (`harnez` issues 210, 218) still have no automated
  guard** — both were caught by live observation, not CI. If `internal/usage`
  keeps regressing on the same class of bug (column width, chart baseline
  visibility), a golden-output or snapshot test for the terminal renderer would
  likely pay for itself faster than another live-catch cycle.
- **This survey format itself has a limit worth naming**: reconstructing "what
  worked" and "what nearly failed" from commit messages alone is only as honest as
  the commit messages are. Every repo here writes unusually detailed messages (this
  is clearly a convention already in place), which is what made this survey
  possible at all — that convention is itself worth protecting as an evergreen rule
  rather than assuming it's incidental.

## 8. File & Diff Summary

Aggregate `git diff --shortstat` totals per repo for the window:

| Repo | Commits | Files touched | Insertions | Deletions |
|---|---|---|---|---|
| `harnez` | 63 | 188 | +7,502 | −591 |
| `ubunatic.com` | 15 | 96 | +5,454 | −759 |
| `lmcoder` | 10 | 36 | +2,171 | −223 |
| `voxi` | 4 | 70 | +7,502 | −101 |
| `uman` | 2 | 4 | +85 | −4 |
| `psync` | 1 | 2 | +106 | −14 |
| **Total** | **95** | **396** | **~22,820** | **~1,692** |

Key files/areas by repo:

- **`harnez`**: `internal/telemetry/classify.go` (rewritten 3x), `internal/usage/{watch,indicatorsspec}.go`, `internal/sessionstate/sessionstate.go`, `cmd/harnez/exec.go`; issues 202–226 filed/closed.
- **`lmcoder`**: `cmd/lmcoder/machine_{setup,status}.go` (new `machine status`/`machine setup` commands, GTT/`ttm.pages_limit` tuning), `docs/MemoryTuning.md`; issues 085–087.
- **`ubunatic.com`**: new `/telemetry` and `/smarthome` subpages, `scripts/telemetry/main.go`, `scripts/extract-prose/` (replacing a Python script), `docs/stories/005-...`; issues 029–031.
- **`voxi`**: `internal/chunks/` (new package: chunked playback), `internal/devsample/` (dev sample recorder/transcribe/lineedit), `internal/audio/audio.go`; issues 034, 050–055.
- **`uman`**: `issues/019-chg-age-column-clarity.md`, `issues/020-no-color-flag.md` — ticket filing only, no code this window.
- **`psync`**: `issues/012-vm-dirty-tuning-hint.md` — ticket filing only.

Working trees for all six repos were clean at time of writing (no uncommitted
changes to report).

---

## `smarthome`: blended in from its own story

The placeholder above has been resolved: `smarthome`'s own retrospective,
`smarthome/docs/studies/2026-09-04-three-days-to-a-public-release.md` (separate repo,
not directly linkable from here), is complete. Its scope is wider than this survey's
24h window — a
full 53-hour, three-day arc from `git init` to a signed public release — so rather
than duplicate its analysis here, this section cross-links the pieces relevant to
the fleet-wide picture:

- **Scale**: 112 commits, 111 files, +11,270/−24 lines, 27 tickets (20 closed), 45
  unit tests, ending in a signed public APK live at `ubunatic.com/smarthome/` and
  running on two household devices — the single highest-throughput repo of any
  surveyed here, fleet-wide or not.
- **Headline pattern, shared with `harnez`'s telemetry-classifier churn (§4 above)**:
  every real bug in `smarthome` (OkHttp crash, `wss://` port fallback, QR scanner
  orientation, a shipped-then-withdrawn pin/unpin feature) surfaced on a real device,
  in modules with zero automated tests — the same "green suite, live-only failure"
  shape as `harnez`'s classifier, just caught by canary-first discipline instead of
  live discovery in most cases.
- **A live, currently-open gap**: the published site's Privacy Policy link 404s in
  production (smarthome study §4.1, ticket 028) — caught by the retrospective itself,
  not by any test or review gate. This directly motivated the new "assert link
  liveness, not presence" release-exit-criterion added to
  [`GoRelease.md`](../practices/GoRelease.md) §6 as part of this survey's follow-up.
- **Tracker drift**: four `smarthome` tickets read `In Progress` for shipped,
  live-verified work (study §4.2) — the same failure mode as this survey's own
  observation that closing tickets doesn't happen automatically once work ships. Now
  captured as an explicit invariant in
  [`IssueTracking.md`](../practices/IssueTracking.md) §5 and
  [`AgenticLoop.md`](../practices/AgenticLoop.md) Phase 5.
- **Evidence that generalized past `smarthome` itself**: its `harnez stats` data
  (`Edit` 11.1% failure vs. `apply_patch` 4.2% over 132 calls) turned an
  intuition-based editing rule into an empirically backed one, folded into
  `AgenticLoop.md`'s anti-patterns list.

See the `smarthome` study directly for the full canary-first development narrative,
the security-cleanup post-mortem (real household credentials baked into source for
two days before a human-gated sanitization pass), and the four research tickets that
correctly concluded "don't build this."
