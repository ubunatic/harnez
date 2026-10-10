# 770 — integrate optional architect refinement step into lean-sprint

**Status**: Closed — Added optional architect refinement and refined-sprint alias; Go tests pass, changes are installed, and both skill files were verified.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `docs/commands/lean-sprint.md`, `config.yaml`, `issues/752`

---

## 1. Problem & Motivation
The current `lean-sprint` skill (`docs/commands/lean-sprint.md`) executes a two-agent flow:
`Cheap Searcher -> Focused Developer -> Diff-First Review -> Teardown/Report`.

In complex or nuanced tickets, moving directly from the cheap searcher's audit to the developer worker can leave architectural ambiguities, missing boundary definitions, or unrefined milestone structures that cause developer thrash or rework. Rather than maintaining a divergent skill, `lean-sprint` should natively integrate an optional architect refinement pass.

## 2. Technical Specification / Findings
- **Single Source of Truth**: Keep canonical skill documentation in `docs/commands/lean-sprint.md`, distributed via `harnez apply` to agent skill directories. Provide a lightweight `/refined-sprint` alias entry in `config.yaml`.
- **Optional Refinement / Architect Step (Step 1.5)**:
  - Positioned after Step 1 (Bounded Discovery with Cheap Searcher) and before Step 2 (Goal Handoff to Developer).
  - Can be explicitly requested via prose: `/lean-sprint <ticket> refined` (or `with architect`, `with sol:low`, `using opus:low`, or alias `/refined-sprint <ticket> [tier]`).
  - Supported architect tiers: Exactly one of `opus:low`, `flash38:low`, or `sol:low`.
  - Normalize vendor mentions: `opus` -> `opus:low`, `sol` -> `sol:low`, `flash38` -> `flash38:low`.
  - Pause-for-parameter enforcement: If refinement is requested without a valid tier, the host halts immediately, reports the 3 supported tiers, and asks the user to pick one before starting any agents.
  - Proactive suggestion in default mode: If running without refinement, but findings reveal complex architecture (multi-system boundaries, API/schema redesign, non-trivial component trade-offs), the host orchestrator pauses and suggests an architect pass with the 3 supported tiers for user confirmation.
  - Architect execution: Started as `--role advisor`. The architect is read-only regarding code implementation, returning structured architecture, milestone breakdowns (`M1`, `M2`...), boundary constraints, edge cases, and pre-work requirements. The host updates the ticket before developer handoff.
  - Invariant: Host orchestrator preserves zero-coding rule.

## 3. Implementation & Verification Plan
- **Goal**: `/goal Update docs/commands/lean-sprint.md, register refined-sprint alias in config.yaml, verify test suite, and run harnez apply, or stop and report when blocked on user input or denied permission.`
- **Implementation**:
  - Update `docs/commands/lean-sprint.md` with:
    - Updated description and subtitle.
    - Extended Invocation Syntax including `/refined-sprint` alias and prose syntax.
    - Step 1.5 (Architect Refinement) with pause-for-parameter pattern, proactive suggestion, and read-only advisor contract.
  - Update `config.yaml` to include `refined-sprint` bundled skill alias if desired or update `lean-sprint` description.
  - Run `go test ./...` in harnez to ensure tests (such as `agentspec_test.go`) pass.
  - Run `harnez apply` so the updated skills propagate to all agent skill directories.
