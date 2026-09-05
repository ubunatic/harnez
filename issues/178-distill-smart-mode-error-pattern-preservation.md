# 178 — Distill: Opt-In Smart Mode with Classifier-Driven Error Pattern Preservation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Distillation / Ergonomics
**Related**: [[066-native-go-command-output-distillation]], [[174-shell-execution-quality-scoring-and-error-pattern-detection]], `internal/distill/distill.go`, `internal/telemetry/score.go`, `cmd/harnez/distill.go`

---

## 1. Problem & Motivation

`harnez distill` currently supports filtering modes: `auto`, `gotest`, `git`, and `raw`.
In `auto` and `raw` modes, distillation performs static noise reduction (ANSI stripping, passing test truncation, repeated line collapsing).

However, when commands fail with complex errors (such as Go runtime panics, multi-goroutine stack traces, Python tracebacks, or compiler type-mismatch blocks), standard head/tail line-capping or generic deduping risks truncating the exact stack frame or root-cause line the agent needs to diagnose the failure.

In ticket 174, we introduced [`ScoreShell`](../internal/telemetry/score.go), a pure library function capable of identifying:
- Go panics / fatal runtime errors / goroutine stack dumps
- Go compiler / type / syntax errors
- Go test failures / assertion errors
- Python tracebacks & Node unhandled promise rejections

Reusing this classifier inside `distill` can enable a **Smart / Adaptive Distillation** mode that dynamically preserves high-value error context while aggressively pruning irrelevant output.

## 2. Technical Specification

### 1. Opt-In Behavior & Mode Definition
The default distillation mode (`auto`) must remain simple and predictable (no surprising dynamic omissions).
Introduce a dedicated opt-in mode:
- **CLI Flag / Mode**: `harnez distill --mode smart` (or `harnez distill --smart`)
- **Type**: `distill.ModeSmart Mode = "smart"` in `internal/distill/distill.go`

### 2. Error-Aware Distillation Pipeline
When `ModeSmart` is active:
1. **Error Scan**: Run `telemetry.ScoreShell(output, exitCode)` to classify output quality (1–5) and identify error signatures.
2. **Adaptive Filtering Rules**:
   - **On Panic / Fatal Crash (Score 1)**:
     - Detect and fully preserve the crashing goroutine stack trace, panic header, and panic location.
     - Prune background idle goroutines (`goroutine [chan receive]`, `goroutine [select]`, etc.) to minimize token bloat without losing the offending crash trace.
   - **On Build / Syntax Error (Score 2)**:
     - Pin all compiler error lines and their immediate source context lines.
     - Drop noisy tool banner lines or irrelevant environment logs.
   - **On Test Failure (Score 3)**:
     - Run `FilterGoTest` while ensuring failure blocks and assertion diffs are never truncated by `MaxLines` thresholds.
   - **On Clean Success (Score 5)**:
     - Apply standard high-compression noise reduction (stripping ANSI, collapsing routine progress indicators).

### 3. CLI & Hook Integration
- Add `--mode smart` to `harnez distill`.
- Support configuring `distill.mode: smart` in `config.yaml` or harness hook definitions.

## 3. Constraints
- **Zero regression in default mode**: `mode=auto`, `mode=gotest`, `mode=git`, and `mode=raw` behavior must remain byte-for-byte unchanged.
- Smart mode must be purely deterministic and covered by fixture-based unit tests.

## 4. Verification Plan

1. Unit-test `ModeSmart` in `internal/distill/distill_test.go` with fixture outputs:
   - Multi-goroutine panic dump (verifies crash goroutine is kept while 50 idle goroutines are pruned).
   - Go compiler error with 200 lines of build noise (verifies compile error lines are pinned).
   - Python traceback in a large build log.
2. Verify `harnez distill --mode smart -- go test ./...` on failing and passing test suites.
3. Verify `make test` passes cleanly.

---

## 5. Implementation Plan

### Blocking design decision: do NOT import `internal/telemetry` from `internal/distill`

§2.2 says "run `telemetry.ScoreShell(...)`" inside distill. There is no import *cycle*
(`internal/telemetry` does not import `internal/distill`), so it would compile — but
`internal/telemetry` pulls `database/sql` plus a SQLite driver, and `internal/distill` currently has
**zero** internal or heavy third-party imports (verified: no `ubunatic.com/harnez/internal` imports
in `internal/distill/*.go`). Making a pure text-filter package depend on the telemetry database
layer is the wrong direction and would slow every `harnez distill` invocation's startup.

Resolve it by extracting the classifier before writing any smart-mode logic:

1. Move `internal/telemetry/score.go` (the regex vars + `ScoreShell`) into a new leaf package
   `internal/scoring` — pure regex, no dependencies, ~100 lines.
2. Leave `telemetry.ScoreShell` as a one-line delegating wrapper so `cmd/harnez/distill.go:153`,
   `internal/telemetry/classify.go`, and existing telemetry tests keep working untouched.
3. `internal/distill` then imports `internal/scoring` only.

Do this as its own commit with tests green before step 1 below — it is a pure move and keeps the
smart-mode diff readable.

### Second design decision: `ScoreShell` returns a score, not a match location

The adaptive rules in §2.2 need to know *which lines* matched, not just a 1-5 verdict.
`ScoreShell` returns `(int, string)` — the string is a human note ("go runtime panic / fatal
error"), not line offsets. Branching smart mode on the note string would be brittle string matching
on a message intended for humans.

Add to `internal/scoring` a second, additive function used only by distill:

```go
type Signature struct {
    Score int
    Kind  string // stable enum-ish id: "go-panic", "go-build", "go-test-fail", "py-traceback", ...
    Lines []int  // 0-based indices of matching lines
}
func Classify(output string, exitCode int) Signature
```

`ScoreShell` stays exactly as-is (its callers and its telemetry semantics must not shift). `Kind` is
a stable identifier distinct from the human note — do not reuse the note text as a switch key.

### Step-by-step

1. **`internal/scoring` extraction** (above) + `internal/scoring/scoring_test.go` carrying over the
   existing score tests. Verify `go test ./...` green with zero behaviour change.
2. **`internal/scoring`: add `Classify`** returning `Signature` with per-line indices. Table tests
   over the same fixtures.
3. **`internal/distill/distill.go`**: add `ModeSmart Mode = "smart"` to the const block (line 17-22).
   Add a `case ModeSmart:` to `Distill`'s switch (line 259) calling a new
   `FilterSmart(s string, exitCode int) string`. **Do not** add `ModeSmart` to `DetectMode` or
   `DetectModeFromArgs` — §3's zero-regression constraint means `auto` must never resolve to smart.
4. **`internal/distill/smart.go`** — `FilterSmart`, the adaptive rules:
   - `scoring.Classify` first. Switch on `Kind`, not on `Score` — score 1 covers panics, segfaults,
     Python tracebacks and command-not-found, which need different treatment.
   - `go-panic`: keep the panic header + the first goroutine block (the crashing one); drop
     subsequent `goroutine N [chan receive]` / `[select]` / `[IO wait]` / `[sleep]` blocks. A
     goroutine block runs from `^goroutine \d+ \[` to the next blank line — parse it that way rather
     than with a regex over the whole dump.
   - `go-build`: keep every matched line plus 2 lines of trailing context; drop everything else
     except the first and last few lines.
   - `go-test-fail`: delegate to the existing `FilterGoTest`, then **exempt** the retained failure
     block from the later `MaxLines` truncation.
   - clean success (score 5): fall through to today's `auto` behaviour verbatim — reuse
     `DetectMode` + the existing filters, do not write a parallel path.
5. **Pinned-line protection.** This is the part §2.2 implies but does not state: `FilterHeadTail`
   (line 157) is applied *after* the mode filter and will happily cut the very lines smart mode just
   worked to preserve. Add a `FilterHeadTailPinned(lines []string, maxLines int, pinned map[int]bool)`
   variant, or have `FilterSmart` return already-capped output and have `Distill` skip `MaxLines`
   when `mode == ModeSmart`. **Prefer the latter** — simpler, and it keeps the pinning logic in one
   place instead of threading indices through the generic truncator. Whichever is chosen, the
   `MaxBytes` hard cap (line 272) must still apply, since it is a safety limit, not a heuristic.
6. **`cmd/harnez/distill.go`**: update the `--mode` flag help string (line 57) to
   `"filter mode: auto, gotest, git, raw, smart"`. Add validation rejecting an unknown `--mode`
   value — currently `distill.Mode(mode)` accepts any string and silently falls through to no
   structured filter, so a typo'd `--mode smrt` would look like it worked. Small fix, in scope here
   because this ticket is what makes the mode set worth validating.
7. **Config/hook integration** (§2.3): add a `distill.mode` key read by `harnez exec hook`'s
   internal distill call. Confirm where `harnez exec hook` composes distill (per `config.yaml:99-107`
   it does so internally) and thread the configured mode through there. Keep the default `auto`.

### Verification

- `internal/distill/smart_test.go` with the three fixtures §4 names, plus a fourth: **a clean
  passing `go test` run**, asserting `ModeSmart` output is byte-identical to `ModeAuto` output. That
  is the sharpest possible statement of §3's no-regression constraint.
- Assert the goroutine-pruning case by count: 50 idle goroutines in, crashing goroutine retained,
  idle ones absent. Assert on specific retained/absent line content, not just on output length.
- Existing `internal/distill/distill_test.go` must pass unmodified — if any existing assertion needs
  changing, that is a regression, not a test update.
- `harnez distill --mode smart -- go test ./...` on both a passing and a deliberately-failing
  package.

### Key Decisions / Tradeoffs

- **Extract the classifier rather than cross-import.** Costs one mechanical refactor commit; buys a
  distill package that stays dependency-free and a classifier both callers share instead of two
  drifting regex sets.
- **Switch on `Kind`, not `Score`.** Score is a telemetry quality metric; overloading it as a
  dispatch key couples two things that will diverge.
- **Opt-in only, never reachable from `auto`.** Directly per §2.1.
- **Skip `MaxLines` in smart mode rather than pin indices through the truncator.** Less machinery;
  `MaxBytes` still bounds the worst case.

### Risks / Open Questions

- **Smart mode can output more than auto mode** on a large panic, since preservation beats capping.
  That is the intended trade (a truncated stack trace is worthless), but it should be stated in the
  `--mode` help text so it is not a surprise, and `MaxBytes` must remain enforced.
- **Goroutine-block parsing is format-dependent** on Go's runtime dump layout. Keep the parser
  tolerant: if the structure does not match expectations, fall back to auto behaviour rather than
  producing mangled output.
- **Open question**: should `harnez exec hook` default to smart once it is proven? Out of scope
  here — §2.1 pins the default to `auto`, and changing that later is a separate, evidence-backed
  decision.

### Scope

**Medium.** Three commits: classifier extraction (small, mechanical), `Classify` + smart filter
(the bulk), CLI/config wiring (small). The goroutine-block pruner is the only genuinely fiddly part.
