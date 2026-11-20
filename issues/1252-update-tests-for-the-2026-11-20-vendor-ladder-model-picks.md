# 1252 — Update tests for the 2026-11-20 vendor-ladder model picks

**Status**: In Progress
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Bug
**Related**: 992, 1243, 1242, 1229

---

## 1. Problem & Motivation

Branch `models/subscription-ladders` changes `spec/agent.yaml` to the owner's 2026-11-20 model
ladders (one pick per vendor per role trait, OpenAI with or without a subscription) and records
the evidence in `docs/Models.md`. The suite has not run on it: `make test-q1` fails inside the
taia sandbox before any test starts (`bwrap: setting up uid map: Permission denied`). Several
tests pin the old picks and default efforts and will fail until updated.

## 2. Technical Specification / Findings

Spec changes on the branch:

- Default effort `med` for sol, haiku, sonnet, opus, fable, hydrogen, neon; `high` for argon;
  `low` for terra and astra. Names `gpt-6.2-sol`, `gpt-6.2-astra` (Astra unverified).
- Role picks: see the `harnez:model-picks` block in `docs/commands/epic.md`. `luna:high` follows
  each OpenAI pick above `luna:med` as the no-subscription fallback; plan gating is 992.
- Argon roles `reviewer, advisor` (developer removed); flash38 out of all picks, replaced by
  `hydrogen:low`; flash37 kept as `disabled: true`.
- `compact_thresholds`: `codex:gpt-6.1-terra` and `codex:gpt-6.2-sol` at 200000,
  `claude:haiku:xhigh` at 80000 added.
- Guidance prose fixed for YAML: a ` #` in a plain scalar had cut the Neon text; colon-space
  rewritten.

Test change already on the branch (unrun): `internal/subagent/agentspec_test.go` no longer pins
guidance prose; it checks every model has `use` text, every `model:tier` named in guidance
resolves via `ResolveModel`, and `guidance_source_url` is https. Checked by hand against the
spec: all references resolve.

Tests expected to fail and need new expected values:

- `internal/subagent/driver_test.go:57` (default tiers for opus, haiku, sonnet)
- `internal/subagent/purpose_test.go:11-15`, `internal/subagent/fallback_test.go`
- `cmd/harnez/agent_role_test.go:103-105`, `cmd/harnez/agent_picks_test.go`,
  `internal/pickdocs/pickdocs_test.go:21`
- `cmd/harnez/agent_resume_test.go:1126`, `cmd/harnez/agent_cost_test.go` (`gpt-6.1-sol`,
  `gpt-6-astra` names)
- `internal/subagent/agentspec_test.go` developer-role loop over argon, `agy_test.go:209,241`
  (neon and argon default tiers)

## 3. Implementation & Verification Plan

Resume point: on `models/subscription-ladders`, run `make test-q1` outside the sandbox (or in a
session with user namespaces), update the failing expectations to the new spec values without
pinning prose, rerun, then review and merge through a PR. Out of scope: cost recalibration and
the `luna:high` plan gate (992).

## 4. Progress (2026-11-20)

Quota-1 suspended by the owner for this session; tests ran with `go test` in the taia sandbox.
Method: failing test names on `main` vs. the branch for `./cmd/harnez`, `./internal/subagent`
and the other fast failing packages; the sandbox alone fails 147 tests in the first two on
`main` (runner-mode host-only verbs, agymeter paths, socket path length, telemetry broker).

- The branch added 23 failures; all are fixed by updating expected spec values. Cycle tests in
  `role_policy_test.go` now build their own cycle instead of relying on the shipped `tui` pick;
  `TestCodexArgsUseModelThresholds` compares against `CompactThreshold`; the `--picks` test checks
  the chain head instead of an account-dependent resolution.
- No branch-only failures remain in the compared packages; the root package passes.
- Not compared: `internal/server` and `internal/usage` hit the 100 s timeout in the sandbox.
- Sessions recorded on `gpt-6.1-sol` count as a removed model after the rename and cannot
  resume. Owner (2026-11-20): accepted; old sessions will not be resumed.

## 5. Owner decision: tests derive expectations from the spec (2026-11-20)

Updating literal values only moves the problem: the next catalog update breaks the same tests
again. Tests must read expected values from the loaded spec (default tier, chain head, model
name, compaction threshold, cost) or assert properties and ranges (resolves, tier is one of the
model's tiers, threshold positive, chain has one entry per vendor), never copy spec literals.
Keep literals only in tests that build their own synthetic spec or policy.

Targets: the tests touched in `157da616` (`driver_test.go`, `agy_test.go`, `compact_test.go`,
`agentspec_test.go`, `purpose_test.go`, `agent_role_test.go`, `agent_fallback_test.go`,
`agent_test.go`, `agent_picks_test.go`, `agent_resume_test.go`, `agent_cost_test.go`,
`modelcatalog/load_external_test.go`). Check: change a default tier and a pick in a scratch
copy of `spec/agent.yaml`; no test outside spec-validation tests may fail.

Done (2026-11-20): the target tests read expectations from the shipped spec (helpers in
`internal/subagent/specexpect_test.go`) or loop over all shipped models, roles and costly
entries; process tests receive their expected values from the parent. Check passed: with Neon's
default tier changed and developer `smart` reordered in `spec/agent.yaml`, `internal/subagent`,
`modelcatalog`, `cmd/harnez` and the root package showed no failures beyond the sandbox baseline.
The spec-wide resolution test found 1253.

Remaining: one full `make test-q1` on the host, then merge the PR.
