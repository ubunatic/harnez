# 994 — issue-research skill: fan out subagent web research over candidate tickets, report, and update tracker

**Status**: Closed — implemented generic issue-research skill, research.schema.json, and validation tests (28b3aa86, 99ad1bec, ae0418a1)
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

The skill must be repository-agnostic and work for any evaluation cohort (models, libraries, CLI tools, rendering algorithms, infrastructure dependencies). Domain knowledge and project-specific evaluation goals must not be hardcoded in the skill; instead, they are driven by a per-repository configuration file: `.harnez/research.yaml`.

### 2.1 Configuration Schema: `.harnez/research.yaml`
Repositories declare their research profile, environment boundaries, and ranked goals:

```yaml
# .harnez/research.yaml
subagent_model: "luna:med"
subagent_role: "advisor"
batch_size: 3

# Target environment & boundary constraints
constraints:
  profile: "RTX 3060 12GB VRAM, 48GB host RAM, Ubuntu Linux, x86_64"
  toolchain: "Go 1.26, llama.cpp"
  disallowed:
    - "Closed-source weights or proprietary APIs without self-hosting support"
  ignored_blockers:
    - "Community claims of 'does not work on AMD/Linux' (assume driver/kernel workarounds exist)"

# Prioritized research goals (P1-P3) with brief descriptions
goals:
  - id: fit
    prio: P1
    title: "Environment & Resource Fit"
    description: "Determine exact byte sizes, memory footprints, and feasibility within configured constraints."
  - id: quality
    prio: P1
    title: "Real-world Quality & Failure Modes"
    description: "Search community forums (Reddit, GitHub, X) for real user reception, hallucinations, loops, or regressions."
  - id: baselines
    prio: P2
    title: "Community Baselines"
    description: "Identify which established tools or models reviewers compare this candidate against."
  - id: identity
    prio: P3
    title: "Upstream Identity & Licensing"
    description: "Verify official repository, license terms, author/organization, and release timeline."

# Issue automation rules
actions:
  auto_close_infeasible: true
  close_reason_template: "evaluated: {verdict_summary}"
  update_section: "2. Technical Specification / Findings"
```

### 2.2 Skill Workflow: `/issue-research`
- **Trigger / Command**: `/issue-research [filter/query]` (e.g. `/issue-research "try|trial"` or `/issue-research --status open`).
- **Phase 1: Config Discovery & Cohort Partitioning**:
  - Read `.harnez/research.yaml` for subagent model, environment constraints, and ranked goals.
  - Discover candidate tickets using `harnez find issues`.
  - Partition candidates into parallel batches of 2–4 to stay within rate limits.
- **Phase 2: Subagent Fanout**:
  - For each candidate in the batch, generate a bounded prompt file injecting:
    - Ticket problem & motivation.
    - Configured constraints and ignored blockers.
    - Ranked goals (P1 down to P3) as explicit research questions.
  - Dispatch subagents in parallel via `harnez agent start` with `HTO=0` using the configured model (`luna:med`).
- **Phase 3: Reactive Collection & Synthesis**:
  - React to subagent completion signals (zero busy-waiting).
  - Extract and validate findings against the ranked goals.
  - Author a comparative markdown report artifact summarizing findings, tradeoffs, and a clear verdict (Viable / Infeasible / Follow-up).
- **Phase 4: Issue Tracker Reconciliation**:
  - Update `§2 Technical Specification / Findings` in viable tickets with empirical data.
  - Automatically close tickets verified to be infeasible or obsolete using `harnez issues close <id> "<empirical reason>" --no-code`.

## 3. Implementation & Verification Plan

1. Author the generic skill document `skills/issue-research/SKILL.md` in `harnez`.
2. Include prompt templates for subagent dispatch, schema guidelines for empirical findings, and issue update/close conventions.
3. Register the skill in Harnez skill catalogues / `spec/` configuration as appropriate.
4. Verify by running a dry-run or simulated test over test issue tickets.

/goal Implement the generic issue-research skill in harnez with documented fanout, reporting, and issue-lifecycle reconciliation, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and write a failure report explaining why it failed.
