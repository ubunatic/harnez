# 623 — Short peer-addressable names for interactive agent sessions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Usability / Agent Orchestration
**Related**: #612, #620, #622

---

/goal A session started with `harnez agent start -i` carries one short name that is the same in
Harnez and in the provider's own session title (claude, codex, agy), so the user can tell a host
"message your peer <name>" and it resolves; stop and report when blocked on a user decision or
denied permission.

## 1. Problem & Motivation

User practice (2026-09-28): with several claude sessions open, the user runs `/rename` to give
each a short name, usually the repo name (e.g. `loom`, `neus`). Then they can tell one agent
"send a message to your peer agent `loom`", and it works. With extra effort it also works in agy,
which can message other agents too. Short names make it easy to say which agent should talk to which.

Harnez today auto-generates names like `quiet-badger`, and only claude receives the name
(`claude --name`). Codex and agy titles stay unrelated, so the user renames by hand in each tool.

## 2. Technical Specification / Findings

- Default name suggestion: the repo/dir basename when free (`neus`), else the generated name.
  User decides whether that becomes the default or stays opt-in via `--name`.
- Propagate the name to each provider's session title where a launch flag exists; canary
  codex and agy (both have `/rename`; check for a CLI flag or a store field before any TUI typing).
- A `harnez agent rename --name <old> <new>` that updates Harnez and, where possible, the provider.
- #612 (peer discovery) resolves names to conversation IDs; this ticket supplies the names.
  #612 already found agy presence locks at `~/.gemini/antigravity-cli/presence/<id>.lock`,
  which is also a likely ID source for #622.

## 3. Implementation & Verification Plan

- Canary per provider: how to set the title at launch or later.
- Implement default name + propagation + rename, with tests using the fake runner.
- Manual: two `start -i` sessions in different repos, ask one to message the other by name.
