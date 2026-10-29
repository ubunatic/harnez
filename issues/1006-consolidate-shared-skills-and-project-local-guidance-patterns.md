# 1006 — Consolidate shared skills and project-local guidance patterns

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Skill distribution / project configuration
**Related**: 994, 981

---

## 1. Problem & Motivation
Shared Harnez skills need reusable trigger/workflow instructions, but projects often need their own technology-specific requirements, goals, and reference material. Existing patterns should be assessed and aligned rather than forcing every project's guidance into a centrally managed skill or leaving incompatible conventions.

## 2. Technical Specification / Findings
`harnez apply` currently preserves unmarked `docup` skill directories containing `references/` and suggests an ownership marker. On this host, the Codex copy has additional category guides and a different `SKILL.md`; other copies contain testing and architecture guides. Separately, issue 994's `issue-research` uses `.harnez/research.yaml` for project-specific goals and constraints. Other skills may use similar patterns; discover them rather than assuming these are the only ones.

Compare centrally managed references, project-owned Markdown, structured YAML with schema, and hybrids: ownership and update safety, discovery and precedence, portability across agent harnesses, validation, readability, migration and customization. Do not assume YAML or central ownership is inherently preferable; distinguish generic skill guidance from project-owned content.

## 3. Implementation & Verification Plan
/goal Survey representative skills and multiple repositories, document their actual shared/local guidance patterns, and recommend a coherent approach for ownership, discovery and formats, including what Harnez should manage versus merely read. Propose an incremental migration and validation strategy, or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, clear the goal and record the failure and resume point here.

Include Docup and issue-research; identify other real examples and preserve project customizations. Assess whether structured project configuration, loose Markdown references, or a combination best fits each use. Record tradeoffs and decision points; do not implement a broad migration before the approach is agreed.
