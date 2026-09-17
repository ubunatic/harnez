# 386 — Strip eager global doc includes from global CLAUDE.md template

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Context Optimization / Efficiency
**Related**: [[354-collapse-duplicated-instruction-blocks-across-claude-md-agents-md-templates-into-single-source-pointer-pattern]], [[312-deduplicate-global-claude-md-sections-against-the-new-local-agents-md-managed-block]], [[311-single-managed-section-for-local-agents-md-synced-across-every-project]]

---

## 1. Problem & Motivation

In Claude Code (`claude`), any `@path` reference inside `CLAUDE.md` is treated as an eager file include directive, resolving the file and inlining its entire contents into the initial system prompt of every session.

In `config.yaml` (`agents_md.global.sections`), the `Instructions Hierarchy` section contains two literal `@docs/` references:
1. `... see @docs/AgenticLoop.md Invariant 6.`
2. `... See @docs/Bash.md §8.`

When `harnez apply` generates `~/.claude/CLAUDE.md` and installs copyable library docs into `~/.claude/docs/`, Claude Code eagerly loads both `AgenticLoop.md` and `Bash.md` into **every single session globally** (consuming ~3,000+ tokens of startup context), even in clean non-harnez repositories.

This violates the stated Harnez design principle in `config.yaml`:
> *"Minimal Global Docs: keep this global file free of anything not relevant to every project — it applies everywhere and is glue between local and global docs only, steering the agentic setup rather than carrying project-specific content."*

## 2. Proposed Changes

1. **Remove `@` Eager Include Syntax in Global Instructions Template**:
   - In `config.yaml` (`agents_md.global.sections`), replace `@docs/AgenticLoop.md` and `@docs/Bash.md` with plain text references (e.g. `docs/AgenticLoop.md`, or defer specific invariants to local `AGENTS.md`).
   - Retain local `@docs/...` includes exclusively in project-local `AGENTS.md` (where projects explicitly opt into specific language/framework rules via `harnez init`).
2. **Re-sync via `harnez apply`**:
   - Update `~/.claude/CLAUDE.md` so Claude sessions in clean repos start with zero bundled docs.
3. **Verify via Canary**:
   - Test `claude -p` in a clean temporary repository (`/tmp/canary-clean-repo`) to verify that no global docs (`Bash.md`, `AgenticLoop.md`) are inlined unless local `AGENTS.md` requests them.

## 3. Acceptance Criteria

- [ ] `config.yaml` global `Instructions Hierarchy` contains no `@docs/...` eager include tokens.
- [ ] Running `harnez apply` updates `~/.claude/CLAUDE.md` without eager `@docs/...` includes.
- [ ] Running `claude -p` in a clean directory confirms zero unrequested docs in initial context.
- [ ] Existing `internal/claude` tests pass.
