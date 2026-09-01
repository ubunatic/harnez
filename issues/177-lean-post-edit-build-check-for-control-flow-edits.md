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
