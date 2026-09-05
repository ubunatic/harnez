# 177 — Lean, Scoped Post-Edit Build Check for Control-Flow-Reshaping Edits

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Practices / Prompting
**Related**: [[175-promote-structured-patching-over-fragile-edits]] (upstream prevention — better
edit tooling reduces how often a bad edit happens; this ticket is the downstream detection net for
the ones that still get through), `docs/studies/2026-09-01-usage-watch-startup-splash-and-usage-flag-redesign.md`
§2/§4 (origin: a bad multi-hunk `Edit` on `cmd/harnez/main.go` left ~45 dead lines undetected for
two turns, only caught by a full-file `Read`)

---

## 1. Problem & Motivation

A prior session's `Edit` call on `cmd/harnez/main.go` matched a smaller span than intended, leaving
dead/duplicated code the tool's own success response gave no indication of — the string match
succeeded, which says nothing about whether the surrounding function body was still structurally
valid. The mistake was only caught two turns later via a full-file `Read`. A `go build`/`go vet`
immediately after that edit would have surfaced a compiler error instead.

**This must not become "run go build after every edit."** The user explicitly warned against that:
most edits pass, so a build/vet check on every single edit is mostly noise — extra tool calls,
extra tokens, and (per the user's own stated reason for having turned off their language server)
the same distraction problem as an agent getting bombarded with lint/warning output and compulsively
trying to "fix" things that were never actually broken. The fix must be narrowly scoped and lean, or
it becomes a net loss.

## 2. Proposed Direction

1. **Scope, not universal**: only trigger a post-edit check after an edit that *reshapes control
   flow or a function body* — e.g. adds/removes a closing brace, splits or merges a function,
   moves/removes a `return`/`break`/`case`, or otherwise changes structural nesting — not for
   single-line value changes, comment edits, or narrow string substitutions. This needs a concrete,
   cheap heuristic (e.g. a brace/paren-count delta between old_string and new_string, or a check
   limited to edits touching more than N lines with a brace-count change) rather than relying on
   the agent's own judgment call every time, since that judgment call is exactly what failed in the
   original incident.
2. **Lean output, not a full compiler dump**: the check must report as little as possible — ideally
   just a pass/fail signal (exit code) or a single line, not the full `go build`/`go vet` stdout/
   stderr piped into context. Only surface the actual error text on failure (where there's a real
   fix to make); on success, report nothing beyond a one-line confirmation, or nothing at all if the
   harness can represent "check passed" without a chat-visible message.
3. Consider whether this belongs as agent-level guidance (a short, narrowly-worded addition to
   `docs/lang/Go.md` or `AGENTS.md`, conditioned on the heuristic in #1) versus a harness-level hook
   (a PostToolUse hook on `Edit` that runs `go build` only when the heuristic trips, feeding back a
   minimal result) — a hook can enforce the scoping and leanness mechanically, where a prompt-level
   instruction relies on the agent applying the same judgment call that failed originally. Lean
   toward the hook approach for that reason, but confirm feasibility (PostToolUse hooks, matcher
   scoping to `Edit`, and how to keep the heuristic cheap to evaluate) before committing to it.

## 3. Explicit Non-Goals

- Not a general lint/warning-surfacing mechanism. Do not extend this to `go vet` findings unrelated
  to whether the edit compiles, and do not surface style/lint noise at all — this is a "did I just
  break the build" tripwire, not a code-quality gate.
- Not a replacement for [[175]]'s better-edit-tooling direction — this is a safety net for edits
  that still go wrong, not a substitute for reducing how often they do.

## 4. Verification Plan

- Prototype the heuristic (brace/paren-delta or line-count-plus-structural-change detection) against
  a handful of real past edits (including the incident that motivated this ticket) to confirm it
  fires on genuine control-flow reshaping and stays quiet on ordinary edits.
- If implemented as a hook: verify it adds no visible output on a passing check, and confirm its
  own overhead (wall-clock and any unavoidable tool-call/context cost) is small relative to the
  edits it's meant to protect.

---

## 5. Implementation Plan

Go with §2.3's hook option. The prompt-level alternative fails for the reason the ticket already
identifies: it relies on the agent's judgement, and that judgement is exactly what failed in the
originating incident.

### Measured cost baseline (informs the whole design)

On this repo with a warm build cache: `go build ./...` = **0.27s**, `go vet ./cmd/harnez/` =
**0.10s**. The wall-clock cost is negligible. The cost this ticket must actually minimize is
**context tokens**, not time — so the design goal is "emit nothing on success", not "run rarely".
That reframing matters: it means the heuristic can afford to be somewhat over-eager, as long as a
passing check is genuinely invisible.

### Step 1 — `harnez buildcheck hook` (new file `cmd/harnez/buildcheck.go`)

Model it directly on the existing hook commands (`cmd/harnez/distill.go:87-130` for hook JSON
plumbing; `cmd/harnez/agyhooks.go` / `codexhooks.go` for structure). Flow:

1. Decode the PostToolUse payload from stdin. Bail silently (exit 0, no output) unless
   `tool_name` is `Edit` (and `MultiEdit`/`Write` if those matchers are wired).
2. Bail unless `tool_input.file_path` ends in `.go`. Keep v1 Go-only — the whole motivating
   incident is Go, and a language-agnostic build-command registry is premature abstraction.
3. Apply the heuristic (Step 2). Bail silently if it does not trip.
4. Run `go build ./...` from the edited file's module root (walk up for `go.mod`). Use
   `CombinedOutput`.
5. **On exit 0: print nothing, exit 0.** This is the load-bearing requirement.
6. On failure: print at most the first ~5 lines of compiler output, prefixed with a one-line
   marker, and exit 2 so Claude Code surfaces it to the model as actionable feedback rather than
   burying it in transcript-only stdout. Truncate hard — `distill.FilterHeadTail` already exists and
   can do this; reuse it rather than hand-rolling.

Gate the whole thing behind an opt-in env var (e.g. `HARNEZ_BUILDCHECK`), exactly as
`HARNEZ_DISTILL_AUTOPIPE` gates the distill hook (`cmd/harnez/distill.go:69`). That lets `apply`
install it globally while it stays a no-op until the user opts in — the established pattern here,
and the right one for a mechanism whose value is unproven.

### Step 2 — The heuristic (`internal/buildcheck/heuristic.go`, pure function)

`ShouldCheck(oldString, newString string) bool`, testable in isolation with zero I/O:

- Compute the delta in counts of `{`, `}`, `(`, `)` between `old_string` and `new_string`.
- Trip if any brace/paren delta is non-zero — an edit that changes nesting balance within its own
  matched span is precisely the "reshaped control flow" case, and is what the original incident
  produced.
- Also trip if `newString` removes or adds a line matching `^\s*(return|break|continue|case |default:)`
  — §2.1's explicit list.
- Do **not** trip on: single-line value changes, comment-only edits, or edits where both strings are
  brace-balanced and identical in structural-keyword content.

Keep it to those two rules. A line-count threshold (the ticket's alternative framing) is a worse
signal: a 40-line comment rewrite is large and harmless; a 2-line brace edit is small and fatal.

### Step 3 — Wire it

Add to `config.yaml`'s `hooks:` list:

```
  - event: PostToolUse
    matcher: Edit
    command: "harnez buildcheck hook"
```

Note the composition constraint documented at `config.yaml:102-107` and `docs/HookRewritePattern.md`
applies to `PreToolUse`/`Bash` `updatedInput` rewrites specifically — a `PostToolUse`/`Edit` hook
does not collide with the existing `harnez exec hook`. Confirm that when wiring, and record the
finding in `docs/HookRewritePattern.md` so the next person does not have to re-derive it.

### Step 4 — Verify against §4

1. `internal/buildcheck/heuristic_test.go`: table test over real past edits. **Include the
   originating incident's edit** (recoverable from git history of `cmd/harnez/main.go`, referenced
   in `docs/studies/2026-09-01-usage-watch-startup-splash-and-usage-flag-redesign.md` §2) as the
   must-trip case, plus at least 5 ordinary edits from recent history as must-not-trip cases. A
   heuristic with no measured false-positive rate is not verified.
2. Hook-level test in `cmd/harnez/buildcheck_test.go` asserting **empty stdout and stderr** on a
   passing build — the leanness requirement made mechanical, the same way `distill_test.go` /
   `agyhooks_test.go` assert hook output.
3. Manual: enable the env var, make a deliberately brace-breaking edit, confirm one short error
   surfaces; make an ordinary edit, confirm total silence.

### Key Decisions / Tradeoffs

- **Hook over prompt guidance** — mechanical scoping, no per-edit judgement call.
- **Opt-in env gate** — this mechanism's value is unproven and its failure mode (noise) is exactly
  what the user objected to. Ship it off by default and turn it on for a while before defaulting it.
- **Go-only, `go build` only, never `go vet`** — §3 bans lint surfacing outright. `go build` answers
  "did I break the build"; `go vet` answers a different question and would reintroduce the
  language-server noise problem the user described.
- **Silence on success is a tested invariant, not an intention.**

### Risks / Open Questions

- **False positives on legitimate large refactors**: an intentional function split trips the
  heuristic and runs a build that passes — costing 0.27s and zero tokens. That is an acceptable
  false positive precisely because success is silent. Worth stating explicitly, since it inverts the
  usual "keep the trigger narrow" instinct.
- **Multi-module / non-Go repos**: the `go.mod` walk-up returning nothing must be a silent no-op,
  never an error. `harnez` is installed globally, so this hook will fire in repos that are not Go at
  all.
- **Open question**: whether `Write` and `MultiEdit` should also match. `Write` replaces a whole file
  and has no `old_string` to diff, so the heuristic does not apply — recommend `Edit`-only in v1 and
  revisit if a `Write`-caused breakage is ever observed.

### Scope

**Small-to-medium** — one ~80-line command file, one ~40-line pure heuristic package, two test
files, one config hook entry. The heuristic-tuning test data is the largest single time cost.
