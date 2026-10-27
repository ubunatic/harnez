---
kind: study
---

# Software factory pre-planning and tiered subagent allocation (Issue 986)

Date: 2026-10-27. Host: Antigravity (Gemini 3.8 Flash). Architect: `agy:argon:low` (`refine-986`, `arch-986`).
Target: [Issue #986](file:///home/uwe/projects/harnez/issues/986-per-project-machine-readable-glossary-and-harnez-assess-glossary-wording-check.md) (per-project machine-readable glossary and wording check).

## 1. Context & Motivation

Agentic workflows often fall into one of two traps:
1. **Monolithic sprawl**: A single high-end model reads the entire codebase, invents architectures on the fly, writes code, debugs, and runs into context exhaustion or massive token bills.
2. **Rigid scripted sprints**: Fixed multi-stage frameworks that enforce boilerplate turns even when the technical unknowns have already been resolved.

For Issue #986, we explore an experimental **software factory pattern**:
- Orchestrate specialized tiers across available subscriptions (`haiku:med`, `argon:low`, Gemini host).
- Minimize cumulative token consumption while strictly preserving code quality.
- Execute a structured **pre-planning phase** (Ticket Refinement -> Factory Design -> Adversarial Plan Review) before writing code.
- Serve as the blueprint to codify into a reusable skill (`/software-factory`).

---

## 2. Factory Role Specialization & Subscription Economics

| Role | Model | Lifecycle | Responsibility & Token Guardrails |
|---|---|---|---|
| **Host Orchestrator** | Antigravity (Host) | Continuous | Pure delegation; never ingests raw code/doc files; runs background turns (`HTO=0`), commits verified milestone diffs, enforces [Quota.md](file:///home/uwe/projects/harnez/.harnez/rules/Quota.md). |
| **Searcher** | `haiku:med` | Resumable | Codebase discovery; mandated to use `neus ask "<question>"` first before ripgrep/file reads. Resumed across queries to leverage prompt caching. |
| **Architect / Decisions** | `argon:low` | Short, focused turns | High-leverage decision maker. Evaluates API shapes, JSON schemas, edge-case math, and reviews execution plans. Strategy: feed aggregated info in one rich prompt and get a fast, decisive single-turn reply (<1m, ~50k tokens). |
| **Mechanical Developer** | `haiku:med` | Fresh per milestone | Pure translator of explicit specs into Go/YAML and tests. Never invents algorithms or resolves ambiguous architecture. 100k context limit (80k auto-compact); reset to 0k context each milestone to prevent prompt rot. |

### Circuit Breakers & Fallback Policy (Exit Criteria)
- **Haiku underperformance**: If `haiku:med` struggles (loops, breaks tests, weakens assertions, or fails Quota-1 twice), abort the low-tier worker and switch to **`claude:opus:low`** as developer.
- **Argon token/latency spikes**: If `argon:low` takes too long (>2m) or consumes excessive tokens, switch brain/advisory queries to **`claude:opus:med`**.
- **Search stability**: Searchers remain on `haiku:med` (with `neus ask`) regardless of developer fallbacks.

---

## 3. The Pre-Planning Protocol

Before initiating coding on Issue #986, the factory completed two pre-flight advisory gates:

### Step 1: Upstream Ticket Refinement (`refine-986`)
- Model: `agy:argon` (turn: 5m35s, 1.0M tokens incl. cached).
- Resolved §3 Discovery: Settled scope on domain vocabulary + ubiquitous language (`term`, `definition`, `synonyms`, `code_names`, `forbidden`). Excluded general prose style linting and AST refactoring.
- Resolved §4 Prior Art: Evaluated Vale, `textlint`/`prh`, and SKOS/DDD. Chose a native concept-centric YAML schema (`spec/schemas/glossary.schema.json` + `spec/glossary.yaml`) with Vale-style substitution mechanics.
- Produced initial Milestones M1–M4. Committed directly to the issue tracker (`e7566893`).

### Step 2: Adversarial Plan Review (`arch-986`)
- Model: `argon:low` (turn: 51s, 50k tokens).
- Question: *Does it make sense to completely skip a mid/high-tier developer in this experiment, relying solely on `haiku:med` guided by `argon:low`?*
- **Verdict**: **Yes**, because conceptual ambiguity was eliminated in Step 1. `haiku:med` functions with high fidelity when restricted to mechanical implementation of unambiguous specs.
- **Identified Failure Modes for `haiku:med`**:
  1. *Token position math*: Off-by-one errors in `go/scanner` multi-line strings/comments and Markdown fence tracking.
  2. *Regex boundaries*: Go regexp `\b` treating `_` as word characters (breaking `CONTEXT_SIZE` or code tokens), plus non-deterministic map iteration in deduplication.
  3. *Assertion weakening*: Relaxing test assertions to pass `make test-q1` when edge cases fail.
  4. *Copyable-doc drift*: Editing `docs/Spec.md` directly instead of `docs/other/Spec.md`.
  5. *Context fatigue*: Degradation if one session spans multiple milestones.

---

## 4. Guardrails & Milestone Adjustments Adopted

Following the architect's review, the factory incorporated four structural safeguards:

1. **Pre-Bake Algorithms Before Implementation**:
   For complex scanner or boundary logic (M2), `argon:low` writes the exact Go struct definitions and 15-line pseudocode before `haiku:med` starts coding.
2. **Split Milestone 3 into M3a and M3b**:
   - **M3a (CLI Wiring)**: Wire `harnez assess --glossary [--json]` and `internal/lint` with synthetic `t.TempDir()` fixtures.
   - **M3b (Repo Alignment)**: Run `harnez assess --glossary` against the live repository to generate an automated punch list of violations, then assign `haiku:med` to mechanically align them. Prevents overloading the developer with simultaneous CLI wiring and multi-file refactoring.
3. **Lock Test Cases (Anti-Tampering)**:
   Table test cases are embedded verbatim in developer prompts with explicit instructions forbidding assertion weakening. Host verifies test diffs at each commit.
4. **Session Lifecycle Policy**:
   - Resumable Searcher & Architect: retain cache, small state.
   - Fresh Developer per milestone: clean working memory (<25k tokens), eliminating compaction friction.

---

## 5. Next Steps for Skill Codification (`/software-factory`)

This live run on Issue #986 will measure the real cost, turn counts, and failure rates of this multi-tier pipeline. Upon completion, the following steps will be abstracted into a generic skill:
1. Automated role assignment based on task typology (Searcher -> Architect Gate -> Fresh Mechanical Dev -> Host Review).
2. Standard prompt templates for pre-flight adversarial plan review.
3. Automated test-assertion diff checking to catch assertion-relaxation anti-patterns.
