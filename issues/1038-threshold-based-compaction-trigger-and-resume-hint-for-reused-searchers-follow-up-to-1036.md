# 1038 — Threshold-based compaction trigger and resume hint for reused searchers (follow-up to 1036)

**Status**: Backlog
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Enhancement
**Related**: [[1036]] (documented compaction step, delivered by the sprint skills), [[1037]] (`/pm`), 987 (on-demand `harnez agent compact`), 973 (Haiku 80k compaction default), 969 (`agent.compact_thresholds`)

---

## 1. Problem & Motivation
1036 added a documented rule: compact a reused Searcher between milestones or after a large search round, before the next resume. That is a manual step the host must remember. Two gaps remain: the host has no signal of how full a reused Searcher's context is when it resumes, and nothing enforces the compaction when the host forgets.

## 2. Technical Specification / Findings
- `harnez agent compact` works for every provider (987). `harnez agent status --json` reports the session's context usage.
- `agent.compact_thresholds` (969) already sets per-model compaction thresholds, and the lean-sprint skill points to it. The trigger fraction must come from there or from a new spec key, not a Go default (docs/Spec.md).
- `harnez agent resume` does not print anything about context usage today.

## 3. Implementation & Verification Plan
- Choose the trigger: compact automatically before resume when context usage is at or above a spec-defined fraction of the threshold, read from `harnez agent status --json`. Decide between automatic compaction and a warning only.
- Optional: have `harnez agent resume` print a hint when the session's context is above that fraction of its threshold.
- Verification (moved from 1036 §3): run a two-milestone factory sprint. `harnez agent status --json` shows a Pre/Post compaction pair between milestones, and the second resume finishes without automatic compaction.
