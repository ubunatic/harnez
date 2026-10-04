# 700 — Audit and refactor the skill tree using current skill-design guidance

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: [Skill design video](https://www.youtube.com/watch?v=e7TY56-yIvM)

---

## 1. Problem & Motivation
The installed skill tree has accumulated guidance that may not match current skill-design recommendations. The linked video summarizes guidance on discoverable long references, appropriate freedom per task, model-specific testing, shallow reference layouts, ordered checklists, self-checking, and explicit dependencies. Auditing the tree can improve reliable skill loading and portability.

## 2. Technical Specification / Findings
The video recommends keeping `SKILL.md` concise (under 500 lines), adding a contents list to reference files over 100 lines, linking reference files directly from the skill entry point, and separating topic-specific references. It also recommends matching instruction rigidity to task risk, testing with each intended model, using checklists when sequence matters, adding validation and correction loops, and documenting/installing script dependencies.

The audit should determine which recommendations apply to this repository's skills and tooling. The video does not establish that every recommendation applies unchanged to Harnez or its supported agent providers; record those compatibility questions during the audit rather than assuming them.

## 3. Implementation & Verification Plan
**Goal**: Audit and refactor the applicable skill tree against the linked guidance, with documented rationale for recommendations that do not fit; stop and report if a required policy or platform decision needs user input.

Review the skill inventory and references, apply relevant structural and instructional improvements, and verify links, scripts, and any stated validation steps. Keep changes scoped to skills and their supporting files. Report model-specific validation that could not be run.
