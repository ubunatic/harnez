# 362 — LLM-invocation canary harness to automate lite-doc behavioral scoring

**Status**: Open
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

## 3. Acceptance Criteria

- [ ] A runnable harness (script or `harnez` subcommand) takes a `fixtures.yaml` + two doc variant
      paths and produces a scored pass/fail report per fixture, matching issue 359's manual
      `results.md` format.
- [ ] Documented cost/rate-limit behavior if it calls a real LLM API.
- [ ] Re-run issue 359's `scripts/canary-agenticloop-lite/fixtures.yaml` through the new harness
      and compare against the manual `results.md` scoring — note any discrepancies.
- [ ] `go test ./...` and `scripts/smoke-test.sh` pass (if implemented as a `harnez` subcommand).
