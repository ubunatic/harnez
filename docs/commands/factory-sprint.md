# Factory Sprint — Multi-Tier Subagent Assembly Loop

Orchestrate a token-efficient, subscription-tiered software factory sprint: a resumable cheap searcher, single-turn architect consultations on aggregated context, fresh mechanical developers per milestone, strict locked test cases, and Quota-1 host verification. Works across any repository using `harnez` with optional discovery via `neus` and static analysis via `gokay`.

Reference Practice: `@docs/AgenticLoop.md`

## Role Matrix & Subscriptions

| Role | Default Model Pick | Lifecycle | Responsibility & Context Boundaries |
|---|---|---|---|
| **Host Orchestrator** | Main session (Antigravity / Host) | Continuous | Pure delegation; never performs exploratory repo digging; enforces test budget and Quota-1, reviews milestone commit diffs, runs repo build/install verification, commits milestones. |
| **Searcher** | `--role advisor --model cheap` (Haiku med effort) | Resumable (`search-<ticket>`) | Codebase discovery; searches with `neus ask "<question>"` first before ripgrep/file reads when `neus` is available. Resumed across milestones to maximize prompt caching. |
| **Architect** | `--role advisor --model smart` (Argon low effort) | Single-turn advisory (`arch-<ticket>`) | High-leverage decision maker. Evaluates API shapes, data schemas, edge-case algorithms, and locked table/unit tests. Strategy: feed aggregated info in one rich prompt and get a fast, decisive single-turn reply (<2m, ~150k–280k tokens). |
| **Mechanical Developer** | `--role developer --model cheap` (Haiku med effort) | Fresh per milestone (`dev-<ticket>-m<N>`) | Pure translator of explicit specs into code and tests in the repository's target language. Never invents algorithms or resolves ambiguous architecture. Reset to clean context each milestone to stay under the 80k/100k limits and prevent instruction rot. |

---

## Circuit Breakers & Fallback Policy

- **Cheap Worker Underperformance**: If the cheap worker enters a loop, attempts to weaken test assertions, or fails Quota-1 verification twice on the same milestone, abort the low-tier worker and switch to **`claude:opus` (low effort)** as developer.
- **Smart Advisor Latency / Token Spikes**: If smart advisor takes too long (>2m) or consumes excessive tokens, switch brain/advisory queries to **`claude:opus` (med effort)**.
- **Search Stability**: Searchers remain on cheap tier (with `neus ask` priority) regardless of developer fallbacks.

---

## Invariants & Guardrails

1. **One-Level Delegation**: Helpers are dispatched strictly as leaf workers (`--role advisor|developer|reviewer`); they never spawn further subagents.
2. **Zero Host Coding**: The Host Orchestrator never writes code or quick fixes directly. All implementation is performed by the dispatched developer agent.
3. **Pre-Bake Algorithms (Zero Algorithm Invention by Developer)**:
   - For complex algorithms (token position math, multiline comments, regex word boundaries, suppression parsing, serialization), the Architect (`--role advisor --model smart`) must write exact type definitions, interfaces, and pseudocode before the Developer touches code.
4. **Lock Test Cases (Anti-Tampering Guardrail)**:
   - In every developer prompt, pass required table/unit test cases verbatim with explicit instructions forbidding assertion relaxation (e.g. replacing exact matches with `contains` or deleting edge-case tests).
   - Host reviews test diffs (`git diff`) after each milestone to confirm assertions were preserved.
5. **Split Wiring Milestones (Tool-Driven Punch Lists)**:
   - When introducing a new linter, checker, diagnostic, or CLI tool:
     - **Milestone A (Core Engine & Test Fixtures)**: Implement the core command/module and rules with synthetic fixtures and isolated unit tests.
     - **Milestone B (Repo Alignment)**: Run the newly built tool against the live repository to generate an automated punch list of violations, then hand the punch list to the developer to mechanically align until 0 violations remain.
6. **Repository Guidelines & Hygiene**:
   - Adhere to the host repository's conventions (`AGENTS.md`, `CONTRIBUTING.md`). If the project maintains copyable, template, or generated docs, edit canonical sources rather than derived copies.
7. **Quota-1 Enforcement & Quality Audits**:
   - Follow `@.harnez/rules/Quota.md`: run `make test-q1` when available, or run `harnez exec --quota-1 -- <repo-test-command>`.
   - In Go repositories, run `gokay check --all` and `gokay assess` after substantial code additions.
   - If CLI help or command flags changed in projects with golden tests, update golden fixtures before running Quota-1.

---

## Invocation Syntax

- `/factory-sprint <ticket-numbers>` (e.g. `/factory-sprint 986` or `/factory-sprint 042`)
- `/factory-sprint "<task-description>"`

---

## Milestone Execution Loop

For each milestone in the ticket:

### Phase 1: Bounded Discovery (Resumable Searcher)
1. Resume or start `search-<ticket>` (`--role advisor --model cheap`):
   ```bash
   HTO=0 harnez agent resume --name search-<ticket> -f prompt-search.md
   ```
2. Instruct the searcher to run `neus ask "<question>"` first when `neus` is available, followed by targeted repo reads.
3. Collect exact file paths, existing library helpers, and pattern citations.

### Phase 2: Single-Turn Advisory Spec (Architect)
1. Feed the aggregated search findings and milestone goals into a single prompt for `arch-<ticket>` (`--role advisor --model smart`):
   ```bash
   HTO=0 harnez agent resume --name arch-<ticket> -f prompt-arch.md
   ```
2. Architect returns concrete types, function signatures, complete logic, and locked table/unit tests.

### Phase 3: Fresh Mechanical Translation (Developer)
1. Launch a **fresh** developer session for the milestone:
   ```bash
   HTO=0 harnez agent start --role developer --name dev-<ticket>-m<N> --model cheap -f prompt-dev.md
   ```
2. Pass the architect's exact code and locked test suite verbatim.
3. Developer runs local package/module tests (`<repo-test-command>`).

### Phase 4: Host Verification & Boundary Commit
1. Host reviews diff (`git diff`) to confirm zero assertion tampering.
2. Host runs local build/install and targeted module tests.
3. Host commits milestone boundary:
   ```bash
   git commit -m "feat(<scope>): <description> (issue <ticket> M<N>)"
   ```
4. If this is the final milestone, host runs full repository Quota-1 test suite (`make test-q1` or `harnez exec --quota-1 -- <repo-test-command>`) and closes the ticket:
   ```bash
   harnez issues close <ticket> "<evidence>"
   ```

### Phase 5: Teardown, Process Hygiene & Retrospective
1. **Process & Subagent Hygiene**:
   - Terminate and delete all remaining helper sessions (`search-<ticket>`, `arch-<ticket>`, and milestone developers) via `harnez agent delete --name <session>` to enforce Invariant 3 (Zero Zombie Guarantee).
2. **Factory Assembly Retrospective**:
   - Record a concise sprint study note (in `docs/studies/` when present, otherwise in the closed ticket notes) capturing:
     - Delivery statistics: turns per milestone, token footprint, and prompt cache hit efficiency.
     - Circuit breaker assessment: whether cheap workers implemented the locked specs cleanly or required model escalation.
     - Friction points, tooling edge cases, or runtime verification surprises.
3. **Evergreen Learnings & Session Wrap**:
   - Extract any follow-up tickets discovered during the sprint.
   - Run `/evergreen` to capture enduring architecture or review learnings, then `/wrap` to conclude the session.
