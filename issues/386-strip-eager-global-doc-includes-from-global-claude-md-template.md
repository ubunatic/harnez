# 386 — Strip eager global doc includes from global CLAUDE.md template

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[354-collapse-duplicated-instruction-blocks-across-claude-md-agents-md-templates-into-single-source-pointer-pattern]], [[312-deduplicate-global-claude-md-sections-against-the-new-local-agents-md-managed-block]], [[311-single-managed-section-for-local-agents-md-synced-across-every-project]], [[385-tokens-command-to-count-tokens-in-files-and-directories]], [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[docs/studies/2026-08-31-instruction-distribution-audit-synthesis.md]]

---

## 1. Problem & Motivation

In Claude Code (`claude`), any `@<path>` reference inside `CLAUDE.md` functions as an eager file include directive. When initializing a session, Claude Code resolves the path and inlines the referenced file's full content directly into the initial system prompt context.

In `config.yaml`, the `agents_md.global.sections` definition for the `Instructions Hierarchy` section contains two literal `@docs/` references:
- Line 467: `CLAUDE.md); use grep or range-bounded reads — see @docs/AgenticLoop.md Invariant 6.`
- Line 480: `See @docs/Bash.md §8.`

When `harnez apply` runs, it installs:
1. `~/.claude/CLAUDE.md` containing these directives.
2. Library docs into `~/.claude/docs/`, including `AgenticLoop.md` (7,807 bytes, ~2,000 tokens) and `Bash.md` (4,820 bytes, ~1,250 tokens).

Because `docs/AgenticLoop.md` and `docs/Bash.md` are present in `~/.claude/docs/`, Claude Code resolves both references and eagerly inlines **~12.6 KB (~3,250 tokens)** into **every single session globally** across the machine. This occurs even in completely clean, non-harnez repositories, non-Go projects, or quick ad hoc CLI invocations.

This violates the Harnez architectural contract declared in `config.yaml`:
> *"Minimal Global Docs: keep this global file free of anything not relevant to every project — it applies everywhere and is glue between local and global docs only, steering the agentic setup rather than carrying project-specific content."*

Project-specific or language-specific rules belong in local `AGENTS.md` (populated via `harnez init`). Global instructions must remain lightweight glue without incurring multi-thousand-token startup overhead.

---

## 2. Technical Specification & Scope

### 2.1 Eager Includes vs. Plain Citations
- **Project-Local `AGENTS.md`**: Intentional `@docs/...` includes are used here (e.g., `- Go/Golang @docs/Go.md`, `- Bash/Shell @docs/Bash.md`) because a project explicitly opts into those conventions via `harnez init`.
- **Global `CLAUDE.md` (`~/.claude/CLAUDE.md`)**: Must **never** contain unescaped `@docs/...` directives. Mentions of practices or invariants should be cited as plain text (e.g. `(AgenticLoop Invariant 6)` or `docs/Bash.md §8` without the `@` prefix), preserving human and agent readability without triggering Claude Code's file inclusion parser.

### 2.2 Concrete Template Modifications in `config.yaml`
Under `agents_md.global.sections` (`Instructions Hierarchy`):

1. **Context Discipline bullet** (around line 467):
   ```diff
   - - Context Discipline: avoid whole-file reads on active system-prompt files (AGENTS.md,
   -   CLAUDE.md); use grep or range-bounded reads — see @docs/AgenticLoop.md Invariant 6.
   + - Context Discipline: avoid whole-file reads on active system-prompt files (AGENTS.md,
   +   CLAUDE.md); use grep or range-bounded reads (AgenticLoop Invariant 6).
   ```
2. **Working-Directory Hygiene bullet** (around line 480):
   ```diff
   - - Working-Directory Hygiene: prefer `make -C`/`git -C`/absolute paths or a subshell
   -   `(cd dir && cmd)` over bare `cd`; if you must `cd`, return to the starting directory
   -   before the tool call ends — the shell may be shared with the user's own terminal.
   -   See @docs/Bash.md §8.
   + - Working-Directory Hygiene: prefer `make -C`/`git -C`/absolute paths or a subshell
   +   `(cd dir && cmd)` over bare `cd`; if you must `cd`, return to the starting directory
   +   before the tool call ends — the shell may be shared with the user's own terminal (see docs/Bash.md §8).
   ```

### 2.3 Automated Regression Prevention
To ensure future edits do not reintroduce eager include directives into global templates, add a unit test in `internal/claude` (e.g. `global_claude_test.go`):
- Parse the embedded `config.yaml` `agents_md.global.sections`.
- Assert that no section content matches the regex `(?:^|\s)@docs/\S+`.
- Fail fast during CI / `make test-q1` if an eager include token is detected in global configurations.

---

## 3. Implementation & Verification Plan

### 3.1 Implementation Steps
1. **Edit `config.yaml`**: Remove `@` from doc references in `agents_md.global.sections` ("Instructions Hierarchy").
2. **Add Unit Test**: Implement regression test in `internal/claude/` verifying zero `@docs/` tokens across all global sections.
3. **Run Test Suite**: Execute `make test-q1` to verify all tests pass.
4. **Drift & Apply Verification**:
   - Run `harnez diff` to confirm the planned changes to `~/.claude/CLAUDE.md`.
   - Run `harnez apply` to update `~/.claude/CLAUDE.md`.
   - Verify `~/.claude/CLAUDE.md` content directly using grep.
5. **Canary Validation**:
   - Create a clean temporary directory `/tmp/harnez-canary-clean` with no `AGENTS.md` or local docs.
   - Run `claude -p "Canary prompt"` and inspect the effective prompt / token count to confirm neither `Bash.md` nor `AgenticLoop.md` are inlined.

### 3.2 Exit Criteria
- [x] `config.yaml` under `agents_md.global.sections` contains no `@docs/` eager include tokens.
- [x] Automated regression test in `internal/claude` enforces that `agents_md.global.sections` has no `@docs/` references.
- [x] `make test-q1` passes with zero regressions.
- [x] `harnez apply` cleanly syncs `~/.claude/CLAUDE.md` without diff errors.
- [x] Canary run in clean workspace confirms startup context is reduced by ~12.6 KB (~3,250 tokens).
- [x] Issue tracker (`issues/README.md`) is synchronized via `harnez index`.
