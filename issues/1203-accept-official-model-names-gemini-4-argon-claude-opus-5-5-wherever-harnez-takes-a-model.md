# 1203 — Accept official model names (gemini-4-argon, claude-opus-5-5) wherever harnez takes a model

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 822 (agy:argon model), 835

---

## 1. Problem & Motivation

Agent feedback (2026-11-09): "The model name was wrong: harnez calls it argon:low, not
gemini-4-argon." Agents and users know the vendor's official model ID, but harnez only resolves
its own keys and aliases (`agy:argon`, `argon`, `argon:low`). The official ID is already in
`spec/agent.yaml` as `name: gemini-4-argon`, yet a model flag given that ID fails, and the caller
wastes a turn looking up the harnez alias.

## 2. Goal

/goal Every harnez command that takes a model (`agent start --model`, dispatch specs, model picks)
also resolves the official model ID from each entry's `name` field in `spec/agent.yaml`
(e.g. `gemini-4-argon` → `agy:argon`), optionally with a `:<tier>` suffix. Stop and report when an
official name maps to more than one entry and the right choice is the owner's.

## 3. Acceptance

- `harnez agent start --model gemini-4-argon` and `--model gemini-4-argon:low` resolve to
  `agy:argon` (dry run or resolver test).
- Ambiguous official names (one `name` shared by several provider entries) are reported as an
  error listing the candidates, never resolved silently.
- An unknown name's error lists close matches, including official names.
- No new Go defaults: resolution reads `spec/agent.yaml` only.

## 4. Notes

- Check the live resolver and recent commits before starting; the alias code may have moved.
- Unverified: whether several entries share one `name` (e.g. the same Claude model on two
  providers); decide the tie rule only after listing them.

## Searcher findings (s1203, codex:luna:low)

- One shared resolver: `internal/subagent/driver.go:720` `resolveModelIn`; agent start via
  `cmd/harnez/agent_run.go:222`; dispatch defaults/role targets `internal/subagent/agentspec.go:282`; picks :196.
- Aliases: `modelHasAlias` driver.go:881. Unknown-model errors driver.go:790 and :822 list all known specs,
  no ranked close matches.
- No duplicate `name:` values in `spec/agent.yaml` today.
- Tests: `internal/subagent/driver_test.go:47`, `agentspec_test.go:394`, `purpose_test.go:18`; golden
  `internal/subagent/testdata/known-model-specs.golden`.

## Decision

- 2026-11-10 host (decide mode): existing keys and aliases match first; the official `name` is tried
  only when nothing else matched. Ambiguity is tested with a synthetic duplicate entry. Close matches
  rank by edit distance over keys, aliases and names (top 3). Reason: keeps every current resolution unchanged.
