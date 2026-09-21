# 454 — Enforce explicit luna agent model selection without falling back to host provider

**Status**: Closed — explicit model specs fail closed; docs rule added

---

Reserved placeholder ticket.
# 454 — Enforce explicit luna agent model selection without falling back to host provider

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics
**Related**: `AGENTS.md`, `@docs/AgenticLoop.md`

## Problem

When the user or orchestration loop specifies `luna` (for example,
`codex:luna:low` or another low-cost worker tier), agents can substitute their
host model or a native subagent instead of dispatching the requested external
model through Harnez. This violates explicit cost and tier boundaries.

## Goal

Guarantee that an explicit model/provider specification is dispatched exactly as
requested via `harnez agent start <provider:model:tier>`. If that provider or
model is unavailable or unconfigured, fail clearly and ask for guidance rather
than silently substituting a host model.

The implementation must preserve strict mapping for specifications such as
`codex:luna:low`, and must include coverage for the unavailable-provider and
no-silent-fallback cases.

## Milestones (lean-sprint)

Other sessions (../loom, ../lmcoder) share `~/.harnez/agents`; never stop, delete, or resume sessions you did not start.

### M1 — CLI no-silent-fallback (code + tests)
- Reproduction test first: `harnez agent start <spec> <prompt>` and `resume` with an unknown provider,
  unknown model, or unavailable/unconfigured provider must return a clear non-zero error naming the
  requested spec and asking for guidance. It must never dispatch a different provider/model.
- Known-good specs (e.g. `codex:luna:low`) must map exactly to the requested provider/model/tier
  (assert the resolved provider, model, tier).
- Fix the resolution path so there is no fallback branch; cover unavailable-provider and
  no-silent-fallback cases. Use `harnez read -I`/`-L` for large files. Run `make test-q1` once per code change.
- Commit at the boundary: `feat(agent): fail closed on unavailable explicit model (issue 454 M1)`.

### M2 — Instruction/doc rule
- Add a short rule to `docs/practices/AgenticLoop.md` (and the sprint/lean-sprint command docs if they
  state dispatch behaviour): an explicitly named provider:model:tier is dispatched exactly via
  `harnez agent start`; on failure report and ask, never substitute the host model or a native subagent.
- Commit: `docs(agentic-loop): forbid silent model fallback (issue 454 M2)`.

### M1 delivery (0e82504)
Removed `ResolveModelWithFallback`; `start`/`chat` fail closed, `resume` errors carry guidance. Full `make test-q1` green on host review.

### M2 Pre-Work / Required Refinements
- The removed fallback printed the list of known models/tiers. The new `ResolveModel` error should
  still list known `provider:model:tier` specs (use `KnownModels()`) so the "ask for guidance" message
  is actionable. Add an assertion in `driver_test.go`, and commit it as part of M2.
- Add a CLI-level test (in `cmd/harnez`) that `agent start codex:missing:low "x"` exits with an error and
  creates no session in a temp `--store-dir`.
