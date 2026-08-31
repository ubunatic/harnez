# 075 — ConciseMode: Dynamic Runtime Switch & AGENTS.md Synchronization (`harnez mode`)

**Status**: Closed — resolved
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics & UI Standards
**Related**: [[065-concisemode-caveman-skill-and-output-distillation]], [[134-conversemode-to-skill-conversion]], `config.yaml`, `docs/practices/ConciseMode.md`, `internal/markdown`

---

## 1. Problem & Architectural Context

`docs/practices/ConciseMode.md` defines 3 tiers of output terseness (Lite, Standard, Ultra), but operates purely as a passive reference doc. In active agent sessions, two distinct execution scopes exist:

1. **Active LLM Session (In-Flight Context)**:
   Agent harnesses (Claude Code, Antigravity, etc.) load `AGENTS.md` / system prompts only at session startup. Modifying disk files alone does NOT steer an already-running parent session because the LLM context is not refreshed mid-flight.
2. **Descendant Subagents & Future Sessions (Disk State)**:
   Spawned subagents (researchers, reviewers, workers) and subsequent sessions read `AGENTS.md` directly from disk on initialization.

To switch operational tiers seamlessly mid-session (e.g. typing `!harnez mode std` or invoking `/mode std`), the harness must satisfy **both** scopes simultaneously.

---

## 2. Dual-Mechanism Architecture

When `harnez mode <tier>` (or `harnez concise <tier>`) is executed:

### A. Immediate In-Flight Context Directive (Stdout)
Because shell escapes and tool commands inject their stdout directly into the ongoing conversation turn, the command prints an unambiguous, high-priority directive block to stdout:
```
[HARNEZ DIRECTIVE: Operational mode switched to Concise Standard (Level 2).
- Strip conversational filler, pleasantries, and connective narrative.
- Use dense telegraphic fragments: Action -> Finding -> Patch.
- Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.]
```
The active model reads this tool output in its immediate context window and shifts tone for the next turn.

### B. Persistent AGENTS.md Section Injection (Disk)
The command uses `internal/markdown.Apply()` to update a managed section in `./AGENTS.md`:
```markdown
<!-- harnez:begin Concise Mode -->
Operate at Concise Standard (Level 2) (@docs/practices/ConciseMode.md).
<!-- harnez:end Concise Mode -->
```
If tier is `off` or `default`, the section is removed or set to default. All newly spawned subagents (`research`, `self`, `reviewer`) read `AGENTS.md` upon spawning and inherit the tier automatically.

---

## 3. Command Specification

- **Command Syntax**: `harnez mode <tier>` or `harnez concise <tier>`
- **Tier Options**:
  - `lite` / `1` — Level 1: Concise Lite (Professional Terse, no fluff, complete sentences)
  - `std` / `standard` / `2` — Level 2: Concise Standard (Telegraphic fragments, zero filler)
  - `ultra` / `3` — Level 3: Concise Ultra (Diffs/status only, zero narrative)
  - `off` / `reset` / `default` — Remove the concise mode directive section from `AGENTS.md`
- **Flags**:
  - `--quiet` / `-q`: Suppress stdout directive (file update only)
  - `--file` / `-f`: Path to AGENTS.md (default: `./AGENTS.md`)
  - `--dry-run`: Display what would be printed and changed without writing disk

---

## 4. Implementation & Verification Plan

1. **Internal Logic (`internal/mode` or `internal/claude`)**:
   - Implement tier resolution and directive mapping.
   - Wire `markdown.Apply` to inject/update `<!-- harnez:begin Concise Mode -->` in `AGENTS.md`.
2. **CLI Registration (`cmd/harnez/main.go`)**:
   - Add `modeCmd` and `conciseCmd` with subcommands / arguments for `lite`, `std`, `ultra`, `off`.
3. **Skill & Slash Command Registration**:
   - Add `/mode` and `/concise-mode` in `commands/` and `config.yaml` (`skills:`).
4. **Unit & Integration Tests**:
   - Test CLI argument parsing and error handling for unknown tiers.
   - Test `AGENTS.md` section addition, replacement across tiers, and clean removal on `off`.
   - Test stdout output formatting for LLM steering.
5. **End-to-End Verification**:
   - Run `make install` and verify in an active terminal session.
