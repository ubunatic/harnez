# Write Overall Follow-Up Project Story

Write the overall follow-up project story in `docs/studies/`, covering deliveries since the previous story,
code metrics, quality, and agentic flow.

## Purpose
A **Story** tracks overall project progress across sessions and serves as primary research material for refining evergreen docs and agent harness rules.

---

## Follow-Up Story (bare `/story`)

Invoked without arguments, `/story` writes a follow-up to the previous overall story and covers everything
developed since then, not only the current session.

1. **Find the previous story.** Take the newest `docs/studies/` file with `kind: story` front matter. For files
   without `kind`, judge from title and content: the last overall story about the agentic flow and project
   progress. Skip sub-studies by individual agents on a single
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
6. **Second-vendor review.** A `kind: story` story gets a dedicated read-only review
   (`harnez agent start --role reviewer`) by a model from a different vendor than the writer, at a high tier: not a
   low-cost tier and not the most expensive one; pick from the current lineup. The reviewer checks numbers,
   delivered tickets, post-mortem honesty and verdicts against git and the tracker. A low-cost helper may gather
   raw facts (counts, ticket states) first; verdicts stay with the reviewer. Apply the accepted corrections and
   note the review (reviewer model, findings fixed) in the story's header.

## Shared Study Rules

Follow the `study` skill for the shared output location and naming, front matter, indexing and fallback
rules, and the **Ticket Extraction Gate**. Use `kind: story`. List allocated follow-up ticket numbers
in the story document.

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
3. **Index the Study**: Follow the shared indexing rule in the `study` skill.
4. **Ticket Extraction Gate**: Follow the shared gate in the `study` skill.
5. **Present Summary**: Provide a short recap to the user highlighting key lessons learned.
