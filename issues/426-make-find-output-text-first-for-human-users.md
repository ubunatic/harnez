# 426 — Make find output text-first for human users

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**:

---

## 1. Problem & Motivation

When a human user runs `harnez find issues -n 1` in a normal terminal, the command can render a PNG even when the matched text is small. For example, a 418-match search limited to one displayed issue currently reports a 700x200 PNG and an estimated visual-token cost instead of showing the issue text.

Human users want concise text output by default, including truncation and the `--all` hint, while agents may still benefit from PNG cards for genuinely large search results. The output decision should distinguish the execution context and the amount/shape of content rather than treating every `find` result as visual-card material.

## 2. Technical Specification / Findings

- Preserve useful text behavior for normal user invocations: show the selected result as text, truncate it when appropriate, and retain the hint to use `--all` for the complete result set.
- Keep visual rendering available for agent workflows when the result is sufficiently large or otherwise benefits from visual compression.
- Define the decision inputs and thresholds explicitly, including how user versus agent context is detected and how result count, rendered text size, and `--all` affect the choice.
- Avoid regressing existing explicit rendering/format options and ensure the behavior is deterministic for the same invocation and environment.

## 3. Implementation & Verification Plan

1. **M1 — Decision policy**: Locate the `find` output-selection logic and specify a small, testable policy for human text output versus agent PNG output. Automated verification: focused unit tests cover user/agent context, small results, large results, truncation, `--all`, and explicit overrides.
2. **M2 — Implementation**: Apply the policy without changing issue search semantics or the existing text contents. Automated verification: command-level tests assert output format and the continued `--all` hint; no PNG is produced for the reported small human-user case.
3. **M3 — Regression coverage**: Add boundary cases around the selected thresholds and preserve agent handling for very large result sets. Automated verification: the relevant package tests and the repository test target pass.

Acceptance criteria:

- `harnez find issues -n 1` from a normal user terminal emits text, not a PNG, for small issue content.
- Truncated text still points users to `--all` when more matches exist.
- Large search results can still select PNG output in agent context according to documented, tested thresholds.
- Existing explicit output controls remain honored.
