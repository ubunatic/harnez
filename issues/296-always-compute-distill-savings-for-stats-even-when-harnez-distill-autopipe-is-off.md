# 296 — Always compute distill savings for stats even when HARNEZ_DISTILL_AUTOPIPE is off

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Telemetry / Distillation
**Related**: [[173-record-distillation-byte-savings-in-telemetry]], [[178-distill-smart-mode-error-pattern-preservation]], `cmd/harnez/distill.go`, `cmd/harnez/exec.go`, `internal/telemetry/schema.go`, `internal/telemetry/query.go`

---

## 1. Problem & Motivation

User request (close to verbatim): "Always safe distill stats even when HARNEZ_DISTILL_AUTOPIPE is
off, so we see the expected savings. This may need a new col in the stats [showing] if the distill
was on/off, or better, the distill mode/level."

`HARNEZ_DISTILL_AUTOPIPE` defaults to unset/off (`cmd/harnez/distill.go:69`), so on a typical
session no Bash command is ever routed through distill, and `harnez stats`/`harnez gain` report
"distillation byte savings: no rows with distillation data" (`cmd/harnez/stats.go:306`) even though
distill would have found real savings had it run. The user wants stats to reflect the savings
distill *would* achieve regardless of the autopipe toggle, and suggests either a boolean
autopipe-on/off column or (their preferred alternative) recording the distill mode/level per call.

## 2. Verified Against Current Code

Confirmed accurate, with exact locations:

- The wired PreToolUse/Bash hook is `harnez exec hook`, not `harnez distill hook` directly. Its
  handler (`cmd/harnez/exec.go`, hook entrypoint around line 590) only sets `--distill` on the
  rewritten command when `isDistillAutopipeEnabled()` is true AND `distill.MatchesNoisy(command)`
  (`cmd/harnez/exec.go:604-606`). `isDistillAutopipeEnabled()` (`exec.go:618-621`) reads the same
  `HARNEZ_DISTILL_AUTOPIPE` env var as `distill.go`'s own opt-in gate
  (`distillAutopipeEnv`, `cmd/harnez/distill.go:69`).
- In `harnez exec`'s telemetry recording (`cmd/harnez/exec.go:271-320`), `RawBytes` is always
  populated, but `DistilledBytes` is only computed (non-nil) when `opts.Distill != ""`
  (`distillActive`, line 272) — which for hook-driven calls only happens when autopipe rewrote the
  command with `--distill`. With autopipe off, every Bash call's `tool_calls` row gets
  `distilled_bytes = NULL`.
- `harnez distill -- <cmd>` (the explicit wrapper path, `runDistillWrapper`,
  `cmd/harnez/distill.go:132-160`) always computes and records `DistilledBytes` via
  `distill.DistillWithMetrics` and `recordExecTelemetry(execOptions{Tool: "distill"}, ...)`
  regardless of the autopipe env var — so **the gap is specifically in the hook autopipe path**,
  not the explicit `harnez distill`/`harnez exec --distill` wrapper paths. This matches the
  secondhand hypothesis.
- `internal/telemetry/schema.go` (`tool_calls` table, lines ~40-45) has `raw_bytes` and
  `distilled_bytes` columns only — no column recording autopipe-enabled state or distill mode
  ("auto"/"gotest"/"git"/"raw") per call. `harnez stats`'s distillation section
  (`cmd/harnez/stats.go:304-309`) and `internal/telemetry/query.go`'s `DistillationSavings`
  (lines 257-368) restrict to rows where `distilled_bytes IS NOT NULL`, so they can only ever
  report on the subset of calls that were actually distilled.
- Issue 173 ("Record Distillation Byte Savings in Tool Telemetry", closed) built exactly this
  existing plumbing — it did not address the "measure even when off" gap, since at the time it was
  written the assumption was that `distilled_bytes` should stay `NULL` when a command wasn't
  actually distilled (see its own Constraints section). This ticket is a distinct follow-up, not a
  duplicate.
- Issue 178 (open) is about a smart/classifier distill mode, unrelated to this always-measure gap.

## 3. Scope & Open Questions (not decided here — record honestly, do not invent an implementation)

- **What "safe" measurement means**: presumably running distill's analysis on every Bash call's
  output purely to compute what it *would* have saved — without rewriting the command or altering
  what the agent actually sees — regardless of `HARNEZ_DISTILL_AUTOPIPE`. This needs an explicit
  design decision: is this passive measurement done post-hoc in `harnez exec`'s telemetry-recording
  path (it already has the raw output in hand from `c.CombinedOutput()`/streaming capture), rather
  than at the PreToolUse rewrite stage?
- **Performance/safety cost**: running distill's parsing/pattern logic on every call's output, even
  when not rewriting, adds CPU work to every Bash call. Is this cost acceptable unconditionally, or
  should it be capped/skipped above some output size?
- **Schema/telemetry change**: does `tool_calls` need a new column? Two directions were proposed by
  the user, neither decided:
  - a boolean/enum for autopipe-on/off state per call, or
  - (user's stated preference) the distill mode/level ("auto", "gotest", "git", "raw") that was
    used or would have been used for that call.
  Either requires a schema migration (see the nullable `distilled_bytes` migration precedent in
  `internal/telemetry/schema.go` history, issue 118) and updates to `harnez stats`/`harnez gain`
  reporting to break down real vs. hypothetical savings, and possibly by mode.
- **Interaction with existing paths**: should "always measure" apply only to the hook autopipe path
  (the actual gap), or also add a would-be-mode/would-be-savings measurement to explicit
  `harnez exec`/`harnez distill` invocations that already pass `--distill` or run standalone,
  where real savings are already recorded today?
- **Naming**: is "distilled_bytes" repurposed to mean "would-save bytes" when autopipe is off, or
  does a hypothetical-savings figure need its own column so real (behavior-affecting) and
  hypothetical (measurement-only) savings are never conflated in `harnez stats` output?

## 4. Verification Guidance

- Add/extend a test in `internal/telemetry` or `cmd/harnez` covering a Bash call recorded with
  `HARNEZ_DISTILL_AUTOPIPE` unset that still yields a non-null distillation-savings figure (real or
  hypothetical, per whatever design is chosen) in the `tool_calls` row.
- Run `harnez stats` after a live session with `HARNEZ_DISTILL_AUTOPIPE` off and confirm it no
  longer reports "no rows with distillation data" when noisy commands (go test, git status, etc.)
  were run.
- If a new column is added, verify `harnez index`/schema migration handles pre-existing databases
  without the column (mirroring the nullable-column migration precedent from issue 118).
