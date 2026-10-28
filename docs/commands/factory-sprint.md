---
disable-model-invocation: true
---
# Factory Sprint — Multi-Tier Subagent Assembly Loop

Orchestrate a token-efficient, subscription-tiered software factory sprint: a resumable cheap searcher, single-turn architect consultations on aggregated context, fresh mechanical developers per milestone, strict locked test cases, and Quota-1 host verification.

Reference Study: `docs/studies/2026-10-27-software-factory-pre-planning-and-tiered-subagents-986.md`

## Role Matrix & Subscriptions

| Role | Default Model Pick | Lifecycle | Responsibility & Context Boundaries |
|---|---|---|---|
| **Host Orchestrator** | Main session (Antigravity / Host) | Continuous | Pure delegation; never performs exploratory repo digging; enforces Quota-1, reviews milestone commit diffs, runs `make install`, commits milestones. |
| **Searcher** | `haiku:med` | Resumable (`search-<ticket>`) | Codebase discovery; mandated to use `neus ask "<question>"` first before ripgrep/file reads. Resumed across milestones to maximize prompt caching. |
| **Architect** | `argon:low` | Single-turn advisory (`arch-<ticket>`) | High-leverage decision maker. Evaluates API shapes, JSON schemas, edge-case algorithms, and locked table tests. Strategy: feed aggregated info in one rich prompt and get a fast, decisive single-turn reply (<2m, ~150k–280k tokens). |
| **Mechanical Developer** | `haiku:med` | Fresh per milestone (`dev-<ticket>-m<N>`) | Pure translator of explicit specs into code and tests. Never invents algorithms or resolves ambiguous architecture. Reset to 0k context each milestone to stay under the 80k/100k limits and prevent instruction rot. |

---

## Circuit Breakers & Fallback Policy

- **Haiku Underperformance**: If `haiku:med` enters a loop, attempts to weaken test assertions, or fails Quota-1 verification twice on the same milestone, abort the low-tier worker and switch to **`claude:opus:low`** as developer.
- **Argon Latency / Token Spikes**: If `argon:low` takes too long (>2m) or consumes excessive tokens, switch brain/advisory queries to **`claude:opus:med`**.
- **Search Stability**: Searchers remain on `haiku:med` (with `neus ask` priority) regardless of developer fallbacks.

---

## Invariants & Guardrails

1. **One-Level Delegation**: Helpers are dispatched strictly as leaf workers (`--role advisor|developer|reviewer`); they never spawn further subagents.
2. **Zero Host Coding**: The Host Orchestrator never writes code or quick fixes directly. All implementation is performed by the dispatched developer agent.
3. **Pre-Bake Algorithms (Zero Algorithm Invention by Developer)**:
   - For complex algorithms (token position math, multiline comments, regex word boundaries, suppression parsing), the Architect (`argon:low`) must write exact struct definitions and pseudocode before the Developer touches code.
4. **Lock Test Cases (Anti-Tampering Guardrail)**:
   - In every developer prompt, pass required table-test cases verbatim with explicit instructions forbidding assertion relaxation (e.g. replacing exact matches with `strings.Contains`).
   - Host reviews test diffs (`git diff`) after each milestone to confirm assertions were preserved.
5. **Split Wiring Milestones (Tool-Driven Punch Lists)**:
   - When introducing a new linter, checker, or CLI diagnostic command:
     - **Milestone A (CLI & Rule Wiring)**: Implement the core command and rule with synthetic `t.TempDir()` fixtures.
     - **Milestone B (Repo Alignment)**: Run the newly built tool against the live repository to generate an automated punch list of violations, then hand the punch list to the developer to mechanically align.
6. **Copyable-Doc Discipline**:
   - Never edit root doc copies directly. Edit the source under `docs/other/`, `docs/practices/`, or `docs/lang/`, then run `harnez init -d .` and verify `TestRootDocCopiesMatchSources`.
7. **Quota-1 Enforcement & Golden Updates**:
   - Follow `@.harnez/rules/Quota.md`: run `make test-q1`. If CLI flags changed, regenerate golden fixtures (`go test ./cmd/<app> -run TestHelpGolden -update`) before running Quota-1.

---

## Invocation Syntax

- `/factory-sprint <ticket-numbers>` (e.g. `/factory-sprint 986`)
- `/factory-sprint "<task-description>"`

---

## Milestone Execution Loop

For each milestone in the ticket:

### Phase 1: Bounded Discovery (Resumable Searcher)
1. Resume or start `search-<ticket>` (`haiku:med`):
   ```bash
   HTO=0 harnez agent resume --name search-<ticket> -f prompt-search.md
   ```
2. Instruct the searcher to run `neus ask "<question>"` first.
3. Collect exact file paths, existing library helpers, and pattern citations.

### Phase 2: Single-Turn Advisory Spec (Architect)
1. Feed the aggregated search findings and milestone goals into a single prompt for `arch-<ticket>` (`argon:low`):
   ```bash
   HTO=0 harnez agent resume --name arch-<ticket> -f prompt-arch.md
   ```
2. Architect returns concrete Go structs, complete function logic, and locked table tests.

### Phase 3: Fresh Mechanical Translation (Developer)
1. Launch a **fresh** developer session for the milestone:
   ```bash
   HTO=0 harnez agent start --role developer --name dev-<ticket>-m<N> --model haiku:med -f prompt-dev.md
   ```
2. Pass the architect's exact code and locked test suite verbatim.
3. Developer runs local package tests (`go test -v ./...`).

### Phase 4: Host Verification & Boundary Commit
1. Host reviews diff (`git diff`) to confirm zero assertion tampering.
2. Host runs `make install` and targeted package tests.
3. Host commits milestone boundary:
   ```bash
   git commit -m "feat(<scope>): <description> (issue <ticket> M<N>)"
   ```
4. If this is the final milestone, host runs `make test-q1` and closes the ticket:
   ```bash
   harnez issues close <ticket> "<evidence>"
   ```
