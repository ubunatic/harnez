# 728 — Installed skills get two frontmatter blocks when the source has its own

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: `internal/claude/apply.go` `genSkillContent`, issues/726 (found there)

---

## 1. Problem & Motivation

`genSkillContent` always writes a generated frontmatter block, then appends the source body. When
the source file (`docs/commands/*.md`) starts with its own frontmatter, the installed `SKILL.md`
has two blocks. Example, `~/.claude/skills/harnez-init/SKILL.md`: generated `name`/`description`,
then a second block with `disable-model-invocation: true`. Agents read only the first block, so
the second block's settings are silently ignored and its text shows up in the skill body.

## 2. Technical Specification / Findings

- Affects every own skill whose source has frontmatter (harnez-init confirmed; harnez-handoff too).
- Invocation control is set separately via `config.yaml` `debloat:` (`user-invocable-only`), so
  the lost `disable-model-invocation` may be redundant; check before choosing a fix.

## 3. Implementation & Verification Plan

/goal Every installed own skill has exactly one frontmatter block, with source keys either merged
into it or removed from the sources; a test covers a source with frontmatter. Stop and report when
blocked on a user decision or denied permission.

Check live code and recent commits first.
