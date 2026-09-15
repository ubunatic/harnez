# Lite-doc behavioral canary results

Run via `scripts/canary-lite-doc/run.sh`. Each fixture spawns a genuinely
isolated `claude -p` session (scratch directory outside any harnez-managed
project — no local `AGENTS.md`/`CLAUDE.md` auto-injected) whose only context
is the named lite doc, gives it a coding task exercising that doc's rules,
and runs `harnez lint --check` on the real output file as the mechanical
judge.

| doc | fixture | result |
|---|---|---|
| `docs/lang/Bash.lite.md` | `fixtures/bash-deploy-check.task.md` | PASS — 0 lint findings |
| `docs/lang/Make.lite.md` | `fixtures/make-widget.task.md` | PASS — 0 lint findings |

2026-09-15: both fixtures passed on first run. Manual spot-check of the
generated files beyond what `harnez lint` covers (directory-scoping via
`git -C`, `mktemp`+`trap` cleanup, `printf` over `echo`, no gawk extensions,
`⚙️`/`🤖` sentinel usage, `check`/`check-fast`/`test` alias relationship,
graceful `sudo` degradation) also confirmed compliance.

This supersedes issue 359's manual/reasoning-based canary scoring — no
custom LLM-invocation-and-diff infrastructure was needed; `claude -p` plus
the existing `harnez lint` command was sufficient. See issue 362 (updated)
for scope notes, and issue 363 for pushing the lite docs to a denser
dos/don'ts/code-only style and re-validating with this harness.
