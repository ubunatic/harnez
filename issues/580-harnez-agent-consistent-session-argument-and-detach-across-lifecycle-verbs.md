# 580 — harnez agent: consistent session argument and --detach across lifecycle verbs

**Status**: Closed — merged into 581
**Priority**: P3
**Severity**: Low
**Category**: Agentic Ergonomics / CLI
**Related**: 578 (policy text documents the current quirks), `docs/studies/2026-09-25-repo-manager-decisions.md`

## Problem

Seen by the repo-manager host on 2026-09-25:

- `harnez agent wait` takes the session positionally; `wait --name x` fails with "accepts 1 arg(s)".
- `harnez agent resume` needs `--name`; a positional name is taken as the prompt.
- `start` has `--detach`, `resume` does not.

578 documents these quirks in the generated Subagent Policy instead of fixing them.

## Goal

`/goal`: `wait`, `status`, `stop` and `resume` accept the session the same way (positional and
`--name`), and `resume` supports `--detach` like `start`. Update the 578 policy text to drop the
caveats once fixed.
