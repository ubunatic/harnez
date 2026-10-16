# 772 — apply: remove installed skills that were dropped from config.yaml

**Status**: Closed — apply removes dropped skills via .harnez-managed marker while preserving user skills
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 770 (refined-sprint alias added, then removed)

---

## 1. Problem & Motivation
On 2026-10-10 the `refined-sprint` skill was removed from `config.yaml` (99313fb5), but
`harnez apply` left its installed `SKILL.md` in five targets (`~/.claude/skills`,
`~/.gemini/skills`, `~/.codex/skills`, `~/.prime/agent/skills`, `~/.pi/agent/skills`), so the
skill kept showing up until the folders were deleted by hand.

## 2. Goal
/goal `harnez apply` removes skill folders it installed earlier that are no longer in
`config.yaml`, and never touches skills it did not install (user-made skills); tests cover a
dropped skill being removed and a foreign skill being kept; or stop and report when blocked on
a user decision (e.g. how to recognise harnez-installed skills) or a denied permission.

## 3. Notes
- Needs a reliable marker of harnez ownership (a manifest of installed skills, or a frontmatter
  or file marker); check what apply already records before adding one.
- Report removals in apply's output.
