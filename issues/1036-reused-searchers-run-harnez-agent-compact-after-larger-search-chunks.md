# 1036 — Reused searchers: run harnez agent compact after larger search chunks

**Status**: Closed — Compaction rule landed in factory-sprint Phase 1 and lean-sprint section 1 (5026b0ac). make lint: same 213 pre-existing issues, 3 glossary errors in issues/1031 unrelated to this change. Live Pre/Post compaction verification moved to 1038.
**Closed-By**: claude:haiku:high, session dev-1037
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 987 (on-demand `harnez agent compact`), 973 (Haiku 80k compaction default), 1023, 1038 (threshold trigger, resume hint, and the live Pre/Post verification parked from §3)

---

## 1. Problem & Motivation
`/factory-sprint` keeps one Searcher (`search-<ticket>`) and resumes it across milestones
to get prompt-cache hits. Each round of searching adds file reads and tool output to its
context, so a reused cheap Searcher (Haiku/luna) quickly nears its compaction threshold
(Haiku: 80k; input above 100k costs 5x). Automatic compaction then fires at an arbitrary
point, often in the middle of a search, and the host gets nothing back. The sprint skills
never tell the host to compact a reused Searcher on purpose.

## 2. Technical Specification / Findings
- `harnez agent compact` exists and works for every provider (987).
- `docs/commands/factory-sprint.md` Phase 1 resumes `search-<ticket>` with no compaction step.
- `/lean-sprint` Searchers have the same problem whenever they are resumed rather than started fresh.

## 3. Implementation & Verification Plan
- Add a rule to the factory-sprint skill (Phase 1) and the lean-sprint skill (§1): after
  a large search round (e.g. one milestone's discovery, or many file reads) and before the
  next resume, run `HTO=0 harnez agent compact --name search-<ticket>`.
- Choose the trigger: always compact between milestones, or compact only above a token
  fraction taken from `harnez agent status --json`. The threshold should come from the spec,
  not be hard-coded (docs/Spec.md).
- Optional: have `harnez agent resume` print a hint when the session's context is above
  that fraction of its threshold.
- Verify: run a two-milestone factory sprint; `agent status --json` shows a Pre/Post
  compaction pair between milestones, and the second resume finishes without automatic
  compaction.
