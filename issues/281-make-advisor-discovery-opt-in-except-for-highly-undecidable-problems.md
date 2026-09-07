# 281 — Make advisor discovery opt-in except for highly undecidable problems

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/AgenticLoop.md`, sprint workflow/skill behavior

---

## 1. Problem & Motivation

The advisor-discovery phase can spawn multiple overlapping advisors even when the
task is reasonably decidable. A prior required sprint launched two advisors that
both inherited a large context and consumed tokens quickly, adding cost and
coordination overhead without improving the result. The workflow should make
advisor/subagent discovery opt-in when explicitly requested, while retaining an
exception for problems that are genuinely highly ambiguous or undecidable without
independent exploration.

## 2. Technical Specification / Findings

Update the canonical agentic workflow documentation (likely `docs/AgenticLoop.md`)
and any corresponding sprint skill guidance so that:

- advisors are not routinely required for ordinary work;
- explicit user/developer request is the normal trigger for advisor discovery; and
- the agent may still use advisors when the problem is materially ambiguous,
  underdetermined, or otherwise very difficult to decide from the available
  context, with that judgment made explicit.

Define the boundary clearly enough to avoid treating every non-trivial task as
"undecidable," and preserve any review or safety requirements that remain
independent of advisory discovery.

## 3. Implementation & Verification Plan

1. Revise `docs/AgenticLoop.md` and the relevant sprint instructions to encode the
   opt-in default and narrow undecidability exception.
2. Check the resulting guidance for consistency across repository docs and skills,
   including wording about mandatory sprint phases.
3. Exercise a normal decidable task and a genuinely ambiguous task (or add focused
   documentation checks) to verify that the former does not imply advisor spawning
   while the latter permits it.
