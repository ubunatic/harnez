# harnez bench cleanup, agy provider, generic Bench.md

**Status**: Closed — M1-M3 delivered
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

## M1 delivered (ba735a8)

Bench.md evergreen; dated results moved to docs/studies/2026-09-25-bench-read-conditions.md. Host review OK.

## M2 delivered (54dd787)

Provider table in internal/bench/agent.go, `--read card` added. Host: diff OK, make test-q1 green.

### M3 Pre-Work / Required Refinements

- `card` prompt in tasks.yaml lacks the "open each `See @<png>` with your image-capable file
  reader and read the text from the image" instruction that `auto` has. Add it (shared wording).
- An `agy` provider already exists (`agy -p ... --output-format json`, ParseAgy) but runs agy
  unmetered. M3 = route that existing provider through agymeter (env via AgyLaunchEnv, session
  id), and take tokens/turns from meter records; do not add a second agy provider. Say in the
  report what ParseAgy reports today vs the meter and which wins.

## M3 delivered (a24f3fb)

agy provider runs through agymeter with a per-run session id; input/total tokens and turns come
from the meter. Live checks (one prompt each): haiku 19,880 in; luna 16,043 in; flash37 low meter
15,614 in / 15,939 total over 2 requests (CLI JSON: 15,517 in, 1 turn). Host: diff OK,
make test-q1 green, installed.

## Carry-over for the read benchmark

- Meter "turns" count every agy request, including agy's own helper-model calls (2 requests for
  one answer in the live check). For context growth, filter meter rows to the benchmarked model
  and report helper calls separately.
