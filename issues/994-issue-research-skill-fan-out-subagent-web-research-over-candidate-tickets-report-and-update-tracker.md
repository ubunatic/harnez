# 994 — issue-research skill: fan out subagent web research over candidate tickets, report, and update tracker

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [960](issues/960-research-skill-iterative-web-search-fanout-where-a-reviewer-seeds-the-next-round-user-set-round-count.md), [981](issues/981-epic-workflow-skills-and-skill-installation.md)

---

## 1. Problem & Motivation

Projects frequently accumulate exploratory, evaluation, or "try" tickets (e.g. evaluating open-source models, libraries, dependencies, tools, or runtimes). Assessing them manually or one-by-one inside a main chat session clutters context and slows down work.

An agentic pattern proven effective in live sessions (`lmcoder` issues 231, 234, 235, 125, 212, 233) runs a fanout workflow:
1. Identify a cohort of candidate research tickets.
2. Formulate explicit evaluation questions and machine/environment constraints (e.g. RAM, GPU, OS, license).
3. Fan out disposable, low-cost subagents (`harnez agent start --name ... --model luna:med --role advisor`) to search the web in parallel.
4. Synthesize findings into a comparative report matrix.
5. Directly update the issue tracker: enriching viable candidate specifications and closing infeasible dead-ends with empirical reasons.

Packaging this workflow as a generic Harnez skill (`issue-research`) makes it reusable across any repository without domain-specific hardcoding.

## 2. Technical Specification / Findings

The skill must be repository-agnostic and work for any evaluation cohort (models, libraries, CLI tools, infrastructure dependencies):

- **Trigger / Command**: `/issue-research [filter/query]` (e.g. `/issue-research "try|trial"` or `/issue-research --status open`).
- **Phase 1: Discovery & Cohort Partitioning**:
  - Discover candidate tickets using `harnez find issues`.
  - Allow the user or orchestrator to specify evaluation constraints (e.g. target hardware, license restrictions, required platforms).
  - Partition candidates into parallel batches of 2–4 to stay within rate limits.
- **Phase 2: Subagent Fanout**:
  - Generate structured prompt briefs for each candidate covering:
    - Upstream identity (repo, author, license, release date, scale).
    - Community reception & real-world quality reports (forums, issues, discussions).
    - Common baselines used by reviewers.
    - Resource & environmental fit against specified target constraints.
  - Dispatch subagents via `harnez agent start` with low-cost researcher models (e.g. `luna:med` or configured cheap advisor role).
- **Phase 3: Reactive Collection & Synthesis**:
  - React to subagent completion signals (no busy-waiting).
  - Author a comparative markdown report artifact summarizing findings, tradeoffs, and a clear verdict (Viable / Infeasible / Follow-up).
- **Phase 4: Issue Tracker Reconciliation**:
  - Update `§2 Technical Specification / Findings` in viable tickets.
  - Automatically close tickets verified to be infeasible or obsolete using `harnez issues close <id> "<empirical reason>" --no-code`.

## 3. Implementation & Verification Plan

1. Author the generic skill document `skills/issue-research/SKILL.md` in `harnez`.
2. Include prompt templates for subagent dispatch, schema guidelines for empirical findings, and issue update/close conventions.
3. Register the skill in Harnez skill catalogues / `spec/` configuration as appropriate.
4. Verify by running a dry-run or simulated test over test issue tickets.

/goal Implement the generic issue-research skill in harnez with documented fanout, reporting, and issue-lifecycle reconciliation, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and write a failure report explaining why it failed.
