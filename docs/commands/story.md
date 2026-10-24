# Write Session Case Study / Story

Write a candid engineering retrospective and case study documenting the session's work in `docs/studies/`.

## Purpose
A **Story** is an engineering case study that captures the reality of a development session: what was built, what worked, where mistakes or near-misses happened, code quality metrics, and velocity. It serves as primary research material for refining evergreen docs and agent harness rules.

---

## Output Location & Naming
- Path: `docs/studies/<YYYY-MM-DD>-<kebab-case-title>.md`
- Create `docs/studies/` if it does not exist and indexed studies are enabled.
- If `.harnez/repo.yaml` sets `docs.studies: false`, follow `.harnez/rules/Index.md` for the local-docs/session fallback.
- Link the document in `docs/README.md` (or `docs/index.md`) only when `.harnez/repo.yaml` enables indexed studies.

---

## Required Story Sections

Every story must include the following sections:

### 1. Header & Context
- Date, scope of work, starting state, and intended goals.

### 2. Executive Summary
- Concise overview of the session's work and final outcome.

### 3. What Worked Well
- Successful architectural choices, efficient workflows, effective subagent delegation, and automation.

### 4. Honest Post-Mortem (Failures, Bugs & Near-Misses)
- **Mandatory candor**: Explicitly describe any mistakes, false assumptions, data loss hazards, regressions, or broken steps encountered during the session.
- Document how each issue was caught (e.g. diff audit, unit tests, doctor check) and the exact fix applied.

### 5. Quality & Invariants Audit
- Structured table assessing:
  - Architecture & module separation
  - Idempotency (re-running actions produces zero spurious diffs)
  - Backward compatibility
  - Test coverage & verification results

### 6. Efficiency & Velocity Assessment
- Quantitative and qualitative evaluation of speed, turnaround time, agent efficiency, and time saved.

### 7. Key Learnings & Evergreen Upstream
- Concrete, actionable engineering rules or patterns to promote to `AGENTS.md` or evergreen docs.

### 8. File & Diff Summary
- List of key files modified, created, or deleted, and summary of git commits made.

---

## Execution Workflow for the Agent
1. **Gather Ground Truth**: Run `git status`, `git log --oneline`, and `git diff` to collect factual session history.
2. **Draft the Story**: Follow the required template structure above with full honesty.
3. **Index the Study**: Add the entry to `docs/README.md` under `docs/studies/`.
4. **Ticket Extraction Gate**: Review failures, near-misses, open questions, and recommendations.
   File every actionable follow-up with `harnez issues new` in its owning repository, link the
   story from each ticket, and list each allocated ticket number in the story. Record justified
   non-goals. Ask each allocated worker: "What ticket number was allocated, and have all
   actionable follow-ups from this study been filed and listed here?"
5. **Present Summary**: Provide a short recap to the user highlighting key lessons learned.
