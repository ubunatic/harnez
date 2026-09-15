# 362 — LLM-invocation canary harness to automate lite-doc behavioral scoring

**Status**: Closed
**Priority**: P3 (Low)
**Severity**: Feature
**Category**: Templates / Docs / Canary
**Related**: [Issue 359](359-pilot-agenticloop-lite-md-behavioral-canary-gate.md) (pilot that scored
its fixtures manually instead of via this harness), [docs/other/Canary.md](../docs/other/Canary.md)
(canary-first practice — probe before building, which is why this was deferred rather than built
up front)

---

## 1. Problem & Motivation

Issue 359's behavioral canary gate requires running each fixture prompt against an agent primed
with the full doc, then again primed with the lite doc, and scoring the two responses against a
mechanically-checkable pattern. No plumbing exists in this repo to programmatically invoke an LLM
and capture its response for scoring — `scripts/canary-*` are all shell/CLI-behavior canaries, not
LLM-response canaries.

The pilot (issue 359) scored `scripts/canary-agenticloop-lite/fixtures.yaml` by hand instead
(reasoning through each response manually and recording pass/fail in `results.md`), per
canary-first practice: probe the value of the mechanism before building automation for it. The
manual pass scored the lite doc 7/7 against the full doc's 7/7, suggesting the approach has enough
signal to be worth automating for future lite docs beyond this one pilot.

## 2. Proposed Fix

Build a minimal LLM-invocation harness that can:
- Read a `fixtures.yaml` (format already defined by issue 359's pilot file).
- For each fixture, run the `prompt` against an agent primed with a given doc variant's content
  only (isolate context — no other project docs).
- Capture the response text.
- Check it against `pattern`/`forbid_pattern` (regex, matching the pilot's fixture format).
- Report pass/fail per fixture per variant, and an aggregate score.

Investigate whether the `lmcoder` skill's local LLM invocation plumbing exposes a scriptable
non-interactive call suitable for this (checked but not confirmed during issue 359's advisory
pass). If not, evaluate a minimal direct API call (Claude API, respecting existing
`docs/practices/ClaudeAPI.md`-equivalent conventions if any) as a fallback, gated on cost/rate
concerns for a CI-adjacent tool.

## 3. Resolution (superseded the original proposed fix above)

The proposed fix above (custom `fixtures.yaml`-driven LLM-invocation-and-pattern-scoring harness)
turned out to be unnecessary complexity. Manual experimentation (2026-09-15, prompted by user
question "are we sure the test agent did not see the other bash docs?") found a much simpler and
more rigorous approach already available:

1. `claude -p` (the existing CLI, non-interactively) run from a scratch directory **outside any
   harnez-managed project** gives genuine isolation — no local `AGENTS.md`/`CLAUDE.md` gets
   auto-injected, unlike an in-conversation subagent spawned inside this repo (which does inherit
   the project's own instructions, invalidating an in-repo "bare agent" test).
2. `harnez lint --check` already exists and mechanically judges the generated code against real
   rules (Bash conditionals, source-over-dot, etc.) — no custom `pattern`/`forbid_pattern` regex
   format needed.

Shipped as `scripts/canary-lite-doc/run.sh`: takes a lite doc + a task-description fixture file,
spawns the isolated `claude -p` session, and runs `harnez lint --check` on the real output file.
Two fixtures (`fixtures/bash-deploy-check.task.md`, `fixtures/make-widget.task.md`) both pass —
see `scripts/canary-lite-doc/results.md`.

This also supersedes issue 359's manual/reasoning-based scoring for future lite docs: no need to
reason through what a response "would" say — the harness actually runs it.

## 3a. Acceptance Criteria (original, retained for history)

- [x] A runnable harness takes a doc + task and produces a scored pass/fail report — delivered as
      `scripts/canary-lite-doc/run.sh` (lint-based judging instead of a custom fixtures.yaml
      pattern-match format).
- [x] Documented behavior — see `scripts/canary-lite-doc/results.md`; no LLM API cost/rate-limit
      concerns apply since it shells out to the already-installed `claude` CLI, not a raw API call.
- [x] Re-validated against real docs (Bash.lite.md, Make.lite.md) rather than re-running issue
      359's AgenticLoop fixtures — AgenticLoop's rules aren't lint-checkable (no `harnez lint`
      support for prose/process docs), so the lint-based judge only applies to lintable languages
      (Bash, Make, Go, Markdown) for now; AgenticLoop-style canaries still need manual/other
      scoring.
- [x] No `go test`/`smoke-test.sh` impact — pure shell script addition, no Go code changed.
