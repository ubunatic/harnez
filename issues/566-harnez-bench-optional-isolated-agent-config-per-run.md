# 566 — harnez bench: optional isolated agent config per run

**Status**: Open
**Priority**: P3
**Severity**: Low
**Category**: Bench / Tooling
**Related**: [[565-harnez-bench-run-model-and-read-mode-matrix-lite-docs-default]]

## Background

Bench runs use a fresh temp workspace, but each agent CLI still loads the user's global setup
(`~/.claude` CLAUDE.md, hooks, skills; `~/.codex/AGENTS.md`; agy global rules). That is accepted
for now: the hooks are wanted in bench sessions.

## Goal (parked)

An opt-in mode (e.g. `--isolated`) that starts each run with a clean per-agent config dir
(`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, agy equivalent) holding only login credentials plus an
explicit, chosen set of hooks. Canary first: check each CLI still authenticates with only the
credentials copied.
