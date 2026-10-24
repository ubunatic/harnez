# 904 — Epic skill evaluation: lessons, failure modes, and architectural improvements from Neus Next Wave

**Status**: Closed — Delivered prose phased DAG, sibling decoupling, code/design typology, and phase gates in docs/commands/epic.md; Recommendation 5 remains in 915 (65aac99d).
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Evaluation
**Related**: 900 (epic skill), 902 (first live run), 897 (neus integration), neus:061 (parent epic), neus:063 (retrospective study)

---

## 1. Problem & Motivation

Harnez recently shipped the initial `/epic` skill ([Issue 900](900-epic-skill-plan-refine-and-run-epics-with-optional-web-prior-art-fanout.md), `docs/commands/epic.md`), modeling an epic as a single ticket with a task table, optional web prior-art fanout, and `/lean-sprint` execution.

During the execution of the **Neus Next Wave Program** ([`neus:061`](/home/uwe/projects/neus/issues/061-epic-neus-next-wave-turnkey-adoption-external-remotes-temporal-reasoning-and-auto-indexing.md)), we conducted an end-to-end live testbed of large-scale agentic epic coordination spanning:
- 8 child tickets (`neus` 055, 056, 060, 058, 059, 057, 062, 063)
- 4 sequenced phases
- Multi-provider fleet (`codex:sol:med`, `claude:haiku:med`, `codex:luna:low`, `claude:opus:med`)
- 26 Next Wave commits (60 commits spanning the quality resurgence track)
- Cross-repository integration boundaries with `harnez`

This ticket reports:
1. What was done and how it compares against the current `epic` skill.
2. What worked well.
3. What failed, broke, or created friction.
4. Concrete architectural recommendations to elevate `/epic` into a robust, multi-phase standard.

---

## 2. Execution Analysis: What Was Done vs Current `/epic` Skill

The current `/epic` skill (`docs/commands/epic.md`) assumes a flat task table within a single repository, sequential `/lean-sprint` executions, and uniform execution expectations across all tasks. 

In Neus Next Wave, we operated with several critical governance patterns that extend beyond the current skill:

1. **Phased Track Sequencing (DAG)**:
   - Rather than a flat task list, tickets were organized into 4 strict phases:
     - *Phase 1: Turnkey Adoption & Presentation* (055 README/Website proof cards, 056 `neus install` bootstrap).
     - *Phase 2: Intelligent Query Routing* (060 intent classification with GBNF and `--kind` overrides).
     - *Phase 3: External Repositories & Temporal Git History* (058 remote git indexing, 059 git temporal reasoning).
     - *Phase 4 (Positioned Last): Continuous Auto-Indexing & Governance* (057 daemon design, 062 Harnez integration epic plan, 063 retrospective).
2. **Infrastructure-Last Principle**:
   - Core standalone features and query routing were completed *before* continuous background services and consumer harnesses were designed. This prevented premature API coupling and saved significant rework.
3. **Sibling Decoupling Pattern**:
   - When the user requested `harnez` integration, attempting to mix `harnez` code changes directly into the `neus` operational epic would have violated single-workspace write discipline.
   - We decoupled Harnez integration into its own dedicated follow-up epic (anchoring `harnez #897`) and kept a lightweight planning bridge ticket (`neus #062`) in the current epic.
4. **Dual Delivery Typology (Executable Code vs Architectural Design)**:
   - We explicitly categorized child tickets into:
     - *Code Deliverables* (055, 056, 060): Required `make test-q1`, binary installation, and runtime verification.
     - *Design/Planning Deliverables* (058, 059, 057, 062, 063): Required deep empirical studies in `docs/studies/`, risk models, and decomposed follow-up backlogs without pretending unverified runtime code was delivered.

---

## 3. What Worked Well

- **Zero Zombie Guarantee & Clean Tree Hygiene**:
  - Every subagent (`dev-055`, `dev-056`, `dev-060`, `dev-058`, `dev-059`, `dev-057`, `dev-062`, `dev-063`) ran to completion and terminated cleanly. No orphan processes or locked SQLite handles.
- **Orchestrator Sanity Gates**:
  - The orchestrator executed live CLI sanity checks between phases (e.g. running `neus install --json`, probing `neus ask` intent classification), catching behavioral regressions before Phase $N+1$ began.
- **Strict Single-Test Quota Boundary (`make test-q1`)**:
  - Developers ran `HTO=0 make test-q1` exactly once per code modification, keeping test turnaround fast and compliant.
- **Multi-Provider Velocity**:
  - Using `codex:sol:med` for complex Go architecture and test suites delivered rapid turnaround (~2–7 minutes per ticket), while `haiku:med` and `luna` excelled at read-only discovery.

---

## 4. What Did Not Work & Failure Modes Encountered

1. **Semicolon Citation Splitting (`internal/ask/ask.go`)**:
   - The citation validator split sentences on `;`. When LLMs emitted lists using semicolons without `[n]` on each clause, validation failed with exit code 4.
2. **Negative Assertion Citation Rejection**:
   - When a queried concept was absent and the model truthfully reported *"The retrieved sources do not describe X"*, the validator rejected the answer as an ungrounded factual statement because negative statements have no passage to cite.
3. **Misleading "Endpoint Unreachable" Warning**:
   - Querying a repository with 0 embedded vectors caused `SearchModeRerank` to set `keywordOnly = true`, causing the CLI to falsely report that the embedding endpoint was unreachable.
4. **Git Author Anonymity**:
   - Subagents share the host's Git config identity. Git history alone cannot prove which model or provider authored which commit; attribution requires inspecting ticket closure notes or study dossiers.
5. **Tracker Flatness**:
   - The Harnez tracker treats tickets as an unsequenced flat list. Phase ordering and dependency gates currently depend 100% on orchestrator memory and discipline.

---

## 5. Architectural Recommendations for Harnez `/epic` Skill

Based on the Next Wave empirical retrospective ([`neus:063`](/home/uwe/projects/neus/docs/studies/2026-10-24-issue-063-epic-workflow-retrospective-and-harnez-epic-skill-spec.md)), we recommend extending `docs/commands/epic.md` with:

### Recommendation 1: Phased Track Structure (DAG) in Epic Schema
Replace the simple flat task table with an explicit phased structure:
```yaml
phases:
  - id: phase-1-adoption
    exit_gate: runtime-accepted
    children: [055, 056]
  - id: phase-2-routing
    depends_on: [phase-1-adoption]
    exit_gate: runtime-accepted
    children: [060]
  - id: phase-3-history
    depends_on: [phase-2-routing]
    exit_gate: design-reviewed
    children: [058, 059]
  - id: phase-4-continuous
    depends_on: [phase-3-history]
    exit_gate: design-reviewed
    children: [057, 062, 063]
```

### Recommendation 2: Sibling Decoupling Protocol
Add an explicit rule to `/epic`:
> *An operational epic must be strictly single-repository. When work touches a sibling repository, the epic must not edit both codebases. Instead, create a lightweight planning/bridge ticket in the current repo and file an anchor ticket or follow-up epic in the sibling repo.*

### Recommendation 3: Explicit Delivery Typology (`type: code` vs `type: design`)
Every subticket must declare its delivery class:
- `code`: Must pass `make test-q1`, `make install`, and runtime canary check before closure.
- `design` / `discovery`: Must produce a study in `docs/studies/` and a decomposed implementation backlog; must explicitly disclaim runtime delivery.

### Recommendation 4: Phase Exit Gate Checklist
The `/epic` skill must enforce that Phase $N+1$ cannot start until Phase $N$ satisfies:
1. All declared child tickets are closed with commit hashes referencing external changes.
2. An orchestrator sanity check (CLI probe or canary test) succeeds.
3. Working tree is clean on the default branch.

### Recommendation 5: Provenance in Ticket Closure
Update `harnez issues close` to accept `--model <spec>` and `--session <id>`, recording execution metadata in the generated tracker index so multi-provider attribution is durable.

---

## 6. Implementation & Verification Plan

```markdown
/goal Review this evaluation dossier with Harnez maintainers, integrate the phased DAG, sibling decoupling rule, and delivery typology into docs/commands/epic.md, or stop and report when blocked on a user decision or denied permission.
```

- **Primary Research Artifact**: [`docs/studies/2026-10-24-issue-063-epic-workflow-retrospective-and-harnez-epic-skill-spec.md`](/home/uwe/projects/neus/docs/studies/2026-10-24-issue-063-epic-workflow-retrospective-and-harnez-epic-skill-spec.md).
- **Assigned Model**: `sol:med` / Harnez maintainer.
