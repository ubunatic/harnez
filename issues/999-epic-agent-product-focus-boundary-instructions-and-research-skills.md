# 999 — Epic: Agent product focus, boundary instructions, and research skills

**Status**: Closed — completed all phases: 996 repo-scoped product focus policy, 998 /focus skill, 994 /issue-research skill, epic orchestrator clarification, and evergreen learnings
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Epic
**Related**: 994, 996, 998, 981

---

/goal Complete the child tickets below with /lean-sprint runs; each closes only when its own acceptance holds, and any shortfall gets its own ticket. Stop and report when a worker loops, an architectural blocker cannot be solved, or a ticket needs an owner decision. Circuit breaker: on no progress or persistent loop, clear the goal and write a failure report here.

## 1. Problem & Thematic Affinity

Recent sessions identified two critical workflow friction points across agent operations:
1. **Context & Boundary Drift**: Agents frequently experience tangent drift during long sessions or cross-repo discussions, offering unauthorized implementation in external repositories instead of treating the host repository's applications as their sole product.
2. **Backlog Evaluation Clutter**: Evaluating exploratory "try" or candidate research tickets clutters main conversational context and burns expensive tokens without a standardized parallel fanout mechanism.

This Epic unifies the newly requested skills and governing instructions into a coherent, phased delivery: establishing the foundational boundary instructions first, building the `/focus` interactive re-grounding skill second, and delivering the autonomous `/issue-research` subagent fanout skill third.

---

## 2. Phases & Delivery DAG

```text
Phase 1: Boundary Policy & Instructions (Issue 996)
   │
   ▼
Phase 2: Product Focus Skill /focus (Issue 998)
   │
   ▼
Phase 3: Autonomous Issue Research Skill /issue-research (Issue 994)
   │
   ▼
Phase 4: Synthesis, Documentation & Hygiene (/evergreen, /wrap)
```

### Phase 1: Core Repo-Scoped Product Invariant (Issue 996)
- **Goal**: Inject standard instruction across Harnez templates (`docs/AgenticLoop.md`, system prompt conventions, generated `AGENTS.md`/`CLAUDE.md`).
- **Rule**: Host repository applications are the product; never volunteer cross-repo implementation; only user-mandated external work is allowed.

### Phase 2: Interactive Focus Skill `/focus` (Issue 998)
- **Goal**: Provide the user with an immediate command (`/focus`) to reset agent attention back to the host product, summarize top 2–3 priorities, and project expected outcomes.
- **Depends on**: Phase 1 policy definitions.

### Phase 3: Autonomous Subagent Web Research `/issue-research` (Issue 994)
- **Goal**: Fan out low-cost subagents over candidate evaluation tickets driven by `.harnez/research.yaml` environment constraints and prioritized goals.
- **Depends on**: Independent delivery (can run in parallel or following Phase 2).

---

## 3. Tasks

| # | Ticket | Pri | What | Sprint Group | Searcher → Architect → Developer | Reviewer | Status |
|---|---|---|---|---|---|---|---|
| 1 | 996 | P1 | Agent instruction: repo-scoped focus and product primacy | 1 (Policy) | cheap → — → lean | smart | Closed |
| 2 | 998 | P2 | `/focus` skill: re-ground agent on host product and top priorities | 2 (Skills) | cheap → smart → lean | smart | Closed |
| 3 | 994 | P2 | `/issue-research` skill: fan out subagent web research over tickets | 3 (Research) | cheap → smart → lean | smart | Closed |
| 4 | — | — | Run `/evergreen` learnings and `/wrap` session close | 4 (Wrap) | — | — | Closed |

---

## 4. Out of Scope & Open Decisions

- **Out of Scope**:
  - Modifying code or filing PRs in sibling/external repositories during research runs.
  - Interactive multi-turn subagent research loops (handled separately in #960).
- **Decisions**:
  - Research profile schema lives in `.harnez/research.yaml` with JSON schema validation.
  - `/focus` skill lives in `skills/focus/SKILL.md` and installs via `harnez apply`.
