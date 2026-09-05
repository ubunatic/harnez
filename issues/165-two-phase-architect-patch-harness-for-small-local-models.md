# 165 — Two-Phase Architect/Patch Execution Harness for Small Local Models

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[149-agent-specific-profiles-codex-async-wait-instruction]] (per-agent instruction
profile mechanism — this ticket's read-only-plan/isolated-patch pattern is exactly the kind of
content that has no home in a shared instruction set and would need its own profile, e.g. a
"small local model" profile distinct from Claude Code/Codex/Gemini/Prime Agent),
[[151-on-demand-doc-lookup-vs-materialized-instructions-research]] (research ticket on whether
materialized vs. on-demand instruction delivery is the right model — directly relevant since a
27B-class local model's context/instruction-following budget is the motivating constraint here),
`commands/sprint.md`, `docs/practices/AgenticLoop.md`

---

## 1. Problem & Motivation

External ticket received (source: pasted YAML ticket content, not yet triaged against harnez's
own priority/severity conventions — original fields were `type: feature`, `priority: high`,
`target: agentic-loop`; re-scored below per harnez's own schema since "high" in the source ticket's
own scheme doesn't map directly onto ours):

Local ~27B-parameter models frequently fail when asked to plan, call tools, and emit multi-file
diffs all within a single prompt turn — combining architecture/planning reasoning with concrete
multi-file patch generation in one pass exceeds what these smaller models can reliably hold
together, unlike frontier-class models the current sprint workflow (`docs/practices/AgenticLoop.md`,
`commands/sprint.md`) is implicitly tuned for.

The user flagged that this needs to be coordinated with the existing agent-profile work
([[149-agent-specific-profiles-codex-async-wait-instruction]]) rather than designed as a standalone
mechanism — profiles were introduced precisely because different agents (there: Codex vs. Claude
Code) need different instruction shapes, and "small local model" is plausibly another such profile
rather than a new bolt-on system.

## 2. Requirements & Acceptance Criteria (from source ticket, unedited)

- [ ] Introduce a two-phase execution pattern in `commands/` and `skills/sprint.md`:
  - **Phase 1 (Architect/Plan)**: strictly read-only analysis emitting structured verification
    criteria and invariant edge-cases (e.g. UTF-8 multi-byte tests).
  - **Phase 2 (Isolated Patch)**: single-file diff generation driven by the Phase 1 plan.
- [ ] Integrate an automated validation loop: run `go test ./...` after patch application and pipe
  stdout directly to the retry prompt if exit code is non-zero.
- [ ] Ensure non-destructive fallbacks if diff parsing fails.

## 3. Open Questions / Scope Note

- This repo's sprint workflow already has a real read-only/write-phase separation (Advisor vs. Dev
  Worker roles in `docs/practices/AgenticLoop.md` §3) built for frontier-model multi-agent
  orchestration. Before building a parallel small-model-specific harness, assess how much of that
  existing separation can be reused vs. how much is genuinely different for a single small local
  model driving its own turns (no multi-agent orchestration available).
  - **Single-file-diff constraint**: `commands/`/`skills/sprint.md` currently assume an agent
    capable of coherent multi-file edits per task; a harness constrained to one-file-diffs-at-a-time
    is a meaningfully different shape, not just a stricter prompt.
- Depends on [[149]]'s profile mechanism existing (or at least having a clear design) before this
  can be scoped as "add a small-local-model profile" rather than a bespoke command surface.
- No implementation should start until this is prioritized relative to [[149]]/[[151]]; filed here
  for tracking only.

---

## Implementation Plan

**Deliberately short — this ticket is filed for tracking only** (§3: "No
implementation should start until this is prioritized relative to [[149]]/[[151]]"),
and both of those are still Open. Building a small-local-model harness now would
mean inventing a second, parallel profile mechanism that 149 is about to define,
and committing to materialized per-model instructions while 151 is still asking
whether materialization is the right delivery model at all.

The one step that is useful now and does not front-run either dependency:

1. **Decide whether this is a profile or a workflow.** Answer §3's open question
   on paper before any code: how much of `docs/practices/AgenticLoop.md` §3's
   existing Advisor (read-only) / Dev Worker (write) role split can a *single*
   small model reuse by running the two roles as consecutive turns in one
   session, versus what genuinely needs new machinery. The specific thing that
   is *not* covered by the existing split is the one-file-diff-at-a-time
   constraint plus the automated test-output-into-retry-prompt loop — those are
   an execution harness, not an instruction profile, and conflating the two is
   the main design risk here.
2. If the answer is "mostly reusable", this collapses to a
   `small-local-model` entry in [[149]]'s profile mechanism plus a stricter
   variant of `commands/sprint.md` — Small/Medium.
3. If the answer is "the test-retry loop must be mechanized", that is a separate
   `harnez`-side runner ticket and should be filed as such rather than expanded
   here — Large, and it needs its own justification against actually available
   local-model tooling.

**Recommended action**: leave Open/blocked, revisit after 149 lands. Re-score
priority then; P3 looks right while no local-model workflow is in active use.

### Scope

Not scoped — blocked on [[149]] and [[151]]. Step 1 above (a written decision,
no code) is Small and can be done at any time.
