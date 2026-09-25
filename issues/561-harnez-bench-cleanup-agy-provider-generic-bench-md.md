# harnez bench cleanup, agy provider, generic Bench.md

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Bench / Tooling

## Background

`harnez bench` (`cmd/harnez/bench.go`, `internal/bench`, `docs/Bench.md`) compares reading
conditions (native, `harnez read -n`, `harnez read --auto`) on fact-finding tasks and records
provider input tokens, turns and pass/fail. It drives claude and codex only. We want to use it to
measure context growth of text reads vs PNG cards on agy, whose per-call tokens come from the agy
meter (`~/.harnez/agymeter/usage.jsonl`, `harnez stats --session <id> --calls`).

## Hard limits for the developer

- Do **not** run real benchmark sweeps. Live model calls only as a basic "can the provider be
  called" check, one short prompt per provider, and only on low models: `claude:haiku:low`, agy
  `agy:flash37:low`, `codex:luna:low`. Everything else is unit tests with fakes.
- One `make test-q1` per turn.

## M1 — generic Bench.md

- Rewrite `docs/Bench.md` as an evergreen doc: what the bench is, conditions, tasks, scoring,
  providers, how to run it, how to read results. No dated results, decisions, current quota,
  "for now" or "not wired" state (e.g. lines 41, 44 "First results (2026-09-19)", 172
  "Decision (2026-09-20)"). Move dated results/decisions into a study under `docs/studies/`
  (keep the content, link it from Bench.md in one line).
- Commit `docs(bench): ... (issue 561 M1)`.

## M2 — cleanup

- Read `internal/bench` and `cmd/harnez/bench.go`; remove dead code/flags, unclear names and
  duplicated provider logic so adding a provider is one small, obvious step. Behaviour for claude
  and codex stays the same (existing tests keep passing, assertions intact).
- Add a forced PNG condition (`harnez read -I`, never falls back to text) next to `auto`.
- Commit `refactor(bench): ... (issue 561 M2)`.

## M3 — agy provider

- Run agy through the metered path (`agymeter`, same env as `harnez agent`, see
  `subagent.AgyLaunchEnv`), tag runs with a session id, and take input/total tokens and turns
  from the meter records for that session/prompt. Model selectable (default Gemini Flash low).
- Unit tests with a fake agy and fake meter records.
- Basic live check: one short prompt per provider on the low models above, confirm tokens are
  recorded and non-zero. Report the numbers; do not run the task suite.
- Update `docs/Bench.md` providers section.
- Commit `feat(bench): agy provider (issue 561 M3)`.
