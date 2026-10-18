# harnez apply: roll out managed global instruction sections into ~/.claude/CLAUDE.md and agent globals

Status: Open
Priority: P2
Category: Feature

## Problem
The global `~/.claude/CLAUDE.md` is hand-written; `harnez apply` only injects/removes the Tool
Feedback Protocol. Cross-project rules learned in sessions (e.g. never `pkill -f` a pattern that
matches your own shell, which killed the agent's Bash call twice on 2026-10-17) can only reach
repos via `harnez init` rules (`.harnez/rules/Tools.md`), not agents working outside a repo.

## Proposal
Managed sections in config.yaml (`global_sections:`) that `apply` writes between markers into
`~/.claude/CLAUDE.md`, Codex `AGENTS.md` and AGY globals, idempotent, removable, keeping
user-written text untouched. First section: Process hygiene (pkill/pgrep rule, HTO=0 background
rule).

## Acceptance
- `harnez apply` adds the section once between markers; a second run changes nothing; removing it
  from config removes it; user text outside markers is preserved; tested on temp homes.
- The pkill rule (already in `.harnez/rules/Tools.md` via config.yaml and docs/ProcessHygiene.md)
  reaches the global files.
