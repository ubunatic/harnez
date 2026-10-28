# 998 — focus skill (/focus): re-ground agent on host repo product, summarize top 2-3 open priorities, and prompt for goal

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [996](issues/996-agent-instruction-repo-scoped-focus-never-offer-work-in-external-repos-prioritize-host-repo-product.md), [981](issues/981-epic-workflow-skills-and-skill-installation.md)

---

## 1. Problem & Motivation

During long sessions, multi-turn troubleshooting, or cross-repo discussions (e.g. filing tickets in sibling tools or researching external dependencies), agents often suffer from context dilution or "tangent drift." When the conversation meanders away from the host repository's core deliverables, users need a fast, standardized way to reset the agent's attention back to the host repository, its applications, and its highest-priority goals.

A dedicated `/focus` slash command and skill will allow the user to immediately pull the agent back onto the repository's primary track.

## 2. Technical Specification / Findings

Define the `/focus` skill in `skills/focus/SKILL.md` (and register the `/focus` slash command across supported agent harnesses):

### Workflow & Behavior on `/focus`:
1. **Re-ground on Host Product**:
   - Re-orient the agent: the active repository and its software applications are the agent's product.
   - Discard tangents, external repo proposals, and secondary distractions.
2. **Identify Top Priorities**:
   - Extract the main open task based on:
     a) The immediate conversation thread (what work was actively underway or paused), or
     b) The repository tracker / roadmap (`harnez find issues -a status:active`, `docs/Roadmap.md`).
   - Select the top 2–3 key open items, issues, or epics.
3. **Outcome Projection**:
   - For each of the 2–3 items, concisely describe the concrete expected outcome of completing it (what functionality is unlocked or improved for the product).
4. **Interactive Goal Alignment**:
   - Ask the user to choose or define the next clear `/goal` to pursue.

### Skill Output Format:
```markdown
# 🎯 Repository Focus: <Repo Name>
**Primary Product**: <Summary of core app/deliverable>

### Active Priorities & Expected Outcomes
1. **#<ID> — <Title>**
   - *Outcome*: <Concrete product capability or quality fix>
2. **#<ID> — <Title>**
   - *Outcome*: <Concrete product capability or quality fix>
3. **#<ID> — <Title>**
   - *Outcome*: <Concrete product capability or quality fix>

---
**Next Step**: Which goal would you like to tackle next? (Define with `/goal <statement>`)
```

## 3. Implementation & Verification Plan

1. Author `skills/focus/SKILL.md` in `harnez`.
2. Document slash command usage in `docs/AgenticLoop.md` and relevant command reference docs.
3. Verify invocation in a session to confirm it cleanly extracts top issues, articulates expected outcomes, and solicits the next goal.

/goal Implement the /focus skill and documentation in harnez to re-ground agents on host repo priorities and prompt for the next goal, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and write a failure report explaining why it failed.
