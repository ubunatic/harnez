# 741 — Host runs harnez agent models before starting an explicitly named model like luna:med

**Status**: Closed — fixed: policy says start named models directly
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics / Bug
**Related**: 449 (model aliases), 454 (explicit luna selection fails closed)

---

/goal When the user names a model spec (e.g. "start a luna:med agent"), the host runs the matching `harnez agent start --model luna:med ...` (or `harnez_spawn_agent`) directly, with no `harnez agent models` lookup first; verified by guidance/tests, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
The user says "start a luna:med agent". The host first runs `harnez agent models` to look up the
spec, then starts the agent. The lookup is a wasted tool call: the spec is already explicit, and
`harnez agent start` already rejects unknown specs (454, fail closed).

## 2. Findings
- `internal/agentpolicy/policy.go:21` (managed Subagent Policy) says a requested model is a Harnez
  agent model "(see `harnez agent models`)". Hosts likely read that as "run it first".
- Several skills (`docs/commands/Sprinter.md`, `LeanSprinter.md`, `sprint.md`, `ReverseSprinter.md`)
  point to `harnez agent models` as a menu. That is correct only when the user has *not* named a model.

## 3. Implementation & Verification Plan
- Reword the policy: dispatch an explicitly named spec directly; consult `harnez agent models` only
  to choose a model the user did not name, or after `start` rejects the spec.
- Check the skill docs above for the same ambiguity; sync root doc copies.
- Verify: policy text test, and a live check that "start a luna:med agent" yields one start command.
