# 849 — harnez apply does not refresh an existing installed skill after its source changes

**Status**: Open
**Priority**: P2 (Medium) — skill edits silently never reach agents
**Severity**: Major
**Category**: Bug
**Related**: 716 (skill install), 2eac2b3e (lean-sprint edit that did not arrive)

---

/goal After editing a skill source (`docs/commands/*.md` mapped in `config.yaml`), `make install && harnez
apply` updates the installed `~/.claude/skills/<name>/SKILL.md`, or `apply` reports why it skipped it;
verified on the live install, or stop and report when blocked.

## Observed (2026-10-22)

1. Edited `docs/commands/lean-sprint.md` (commit 2eac2b3e), ran `make install` (binary 02:13) and
   `harnez apply` (exit 0; its summary lists `lean-sprint` under skills).
2. `~/.claude/skills/lean-sprint/SKILL.md` still has mtime 14:20 the previous day and lacks the new
   line (`grep -c 'git-integration files'` → 0). `harnez diff` shows no drift for it.

## Open questions

- Does `apply` write skills only when missing, or treat the existing file as user-owned (a hash or
  marker mismatch)? If it is deliberate, `apply`/`diff` must say so and offer a way to update.
- Same for Codex/Pi skill targets?
