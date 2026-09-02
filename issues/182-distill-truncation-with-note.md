# 182 — `harnez distill` Should Truncate Very Long Output With an Explicit Note

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [issues/178](178-distill-smart-mode-error-pattern-preservation.md) (smart-mode error pattern preservation)

---

## 1. Problem & Motivation

`harnez distill` currently has no explicit cap on output length for pathologically large tool
output (e.g. a build log with thousands of repeated warning lines, a huge JSON dump). Unbounded
output defeats the purpose of distillation (keeping noisy output out of agent context) and can
blow context budgets exactly in the cases distillation exists to prevent.

## 2. Technical Specification / Findings

Needs research before implementation, specifically:

- Whether truncating with a visible marker (e.g. `... [truncated N lines / M bytes] ...`) risks
  confusing agents into thinking the truncation marker itself is meaningful output, or into
  retrying the same command expecting different (untruncated) results.
- How comparable tools handle this. Named for comparison: RTK (Anthropic's internal/eval tool
  reference in this space) and other CLI-output-shaping tools — research what truncation-notice
  conventions they use (position of the note — head, tail, or both; exact wording; whether they
  offer a way to fetch the untruncated original).
- Whether the note should be machine-parseable (a fixed sentinel string) so agents can reliably
  detect "this was truncated, don't treat it as complete" versus a human-readable free-text note.

## 3. Implementation & Verification Plan

1. Research phase: survey how 2-3 comparable tools (RTK and others identified during research)
   signal truncation in tool output; write findings into this ticket's Technical Specification
   section before writing code.
2. Add a configurable max-output cap to `harnez distill` (length in bytes or lines); when
   exceeded, truncate and append a clearly-delimited note stating what was cut and by how much.
3. Decide truncation placement (keep the head, the tail, or both with the middle elided) based on
   what's more useful for typical distilled content (likely: keep both ends for build/test logs
   where the final error matters most).
4. Add a unit test asserting the note is present and the byte/line count is respected.
5. Verify with `go test ./...`; run `make install`.
