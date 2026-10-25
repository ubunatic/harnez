# Write Session Case Study / Story

Write a candid engineering retrospective and case study documenting the session's work in `docs/studies/`.

## Purpose
A **Story** is an engineering case study that captures the reality of a development session: what was built, what worked, where mistakes or near-misses happened, code quality metrics, and velocity. It serves as primary research material for refining evergreen docs and agent harness rules.

---

## Follow-Up Story (bare `/story`)

Invoked without arguments, `/story` writes a follow-up to the previous overall story and covers everything
developed since then, not only the current session.

1. **Find the previous story.** Search `docs/studies/` (newest first, by date prefix and git history) for the last
   overall story about the agentic flow and project progress. Skip sub-studies by individual agents on a single
   concern, test or benchmark reports, and per-ticket sprint reports. Name the chosen story and its date or commit;
   if none qualifies, say so and use the repository's first commit as the baseline.
2. **Compare deliveries.** From git and the tracker since that baseline, list the features delivered, bugs fixed,
   tickets closed and opened, and releases, compared with what the previous story listed as open or planned.
3. **Measure.** Count commits (code vs. `issues/`/`docs/`), new features, closed tickets, and lines added/removed
   outside `issues/` (`git diff --shortstat <base>..HEAD -- . ':!issues'`); say which part is tests, docs or
   generated files. Give an honest quality verdict: improved, stable or diverged, with evidence (test runs,
   regressions caught live, lint state, reverts).
4. **Assess the agentic flow.** Rate how smooth the flow was compared with the previous story: owner corrections,
   retries and loops, live checks that caught what tests missed, wasted cycles, host rule slips. The expectation is
   rising smoothness as the harness and companion tools mature; say plainly when it did not rise and why.
5. Write it with the required sections below, linking the previous story and the sub-studies it summarizes.

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
