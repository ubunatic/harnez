# Focus

Re-orient and ground the agent strictly on the host repository's product, summarize the top 2–3 open priorities with concrete expected outcomes, and align with the user on the next goal.

Triggers: the start of a session, `/focus`, or when the user asks what to work on next.

## Invocation Syntax

- `/focus` — re-ground on the current repository (`.`).

## Workflow

When invoked, the agent executes the following steps immediately without drifting into auxiliary explorations:

### 1. Re-ground on Host Product
- Acknowledge the active repository (`basename $(git rev-parse --show-toplevel)`) and its core software applications, tools, and code as the sole product of this session.
- Drop all tangential discussions, unsolicited cross-repository proposals, and secondary distractions.
- Affirm that external repositories are read-only references or bug-reporting targets, never implementation venues.

### 2. Identify Top Priorities
Extract the top 2–3 key open items, issues, or epics by examining:
1. **Immediate Session Context**: Any task or milestone currently in flight or paused.
2. **Active Issue Tracker**: Run `harnez find issues -a status:active` to discover high-priority (`P0`/`P1`), in-progress, or blocking tickets.
3. **Product Roadmap**: Consult `docs/Roadmap.md` if present for milestone sequencing.

Select the 2–3 most impactful, actionable items for the product's immediate progress.

### 3. Project Concrete Outcomes
For each selected priority, articulate a crisp, 1-line outcome projection:
- What user capability is unlocked?
- What defect, performance bottleneck, or reliability risk is resolved?
- How does it advance the product toward its next release or milestone?

### 4. Interactive Goal Prompt
Format the output cleanly and ask the user to select or state the next `/goal`:

```markdown
# 🎯 Repository Focus: <Repo Name>
**Primary Product**: <Summary of core deliverable / tool / app>

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
