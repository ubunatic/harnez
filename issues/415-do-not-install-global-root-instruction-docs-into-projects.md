# 415 — Do not install global root instruction docs into projects

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: `harnez apply`, `~/.claude/CLAUDE.md`, `~/.prime/agent/AGENTS.md`

---

## Problem & Motivation

Repository documentation must not be installed from Harnez-managed project docs
into global instruction roots. In particular, Harnez must not create, overwrite,
or otherwise manage `/home/uwe/.claude/CLAUDE.md` or equivalent global root files.

Global roots are user-owned and may contain cross-project instructions. Treating
them as generated outputs of this repository creates scope confusion and risks
overwriting unrelated user configuration.

## Scope

- Remove management of global root instruction documents from Harnez apply/init
  flows, including Claude and equivalent agent roots.
- Ensure repository docs are not copied into global root `CLAUDE.md`/`AGENTS.md`
  files as part of installation or synchronization.
- Preserve management of explicitly supported global skills, commands, hooks, and
  settings where those remain in scope.
- Add migration/repair behavior that stops managing previously generated root
  docs without deleting user-owned content.

## Acceptance Criteria

- `harnez apply` does not install or overwrite global root instruction docs.
- `harnez init` does not manage global root instruction docs.
- Existing global root docs are left untouched by apply, init, and repair flows.
- Tests prove the exclusion for Claude and equivalent global roots.
- Existing in-scope global artifacts continue to be managed and verified.
