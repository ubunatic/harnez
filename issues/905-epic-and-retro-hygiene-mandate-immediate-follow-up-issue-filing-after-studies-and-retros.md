# 905 — epic and retro hygiene: mandate immediate follow-up issue filing after studies and retros

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Workflow
**Related**: 904

---

## 1. Problem & Motivation

Engineering studies and retrospectives (conducted under `/epic`, `/lean-sprint`, `/sprint`, or `/story`) are where the richest empirical failure modes, near-misses, edge cases, and design friction are surfaced. However, an agentic loop failure mode was observed during the Neus Next Wave epic:

During a 25-query cross-repository evaluation study and subsequent retrospective, three critical engine bugs were identified and analyzed in detail in `docs/studies/`:
1. Semicolon sentence splitting causing false citation validation failures.
2. Incomplete negative presence recognition rejecting truthful absence statements.
3. Misleading zero-vector repository diagnostics reporting healthy embedding endpoints as unreachable.

Despite being thoroughly analyzed in the study's post-mortem, these findings were initially left as passive prose in the markdown document. The epic was nearly closed without filing corresponding issue tickets in the repository until a human user intervened:
> *"This looks like that these are issues not just in the harnez tool, but also here. Did we file issues for that? Do we need to repair them?"*

When studies and retrospectives do not mandate immediate issue extraction:
- Empirical findings become orphaned in documentation archives.
- High-severity engine bugs and workflow friction go untracked and unrepaired.
- The repository backlog loses sync with discovered technical reality.

## 2. Technical Specification & Policy

We need to establish a strict protocol and invariant across Harnez agentic workflows, skills, and documentation:

### 2.1 The "Study & Retro Ticket Extraction" Invariant

Whenever an agent authors or updates an engineering study (`docs/studies/*.md`) or conducts an epic/sprint retrospective:
1. **Mandatory Extraction Pass**:
   The agent MUST systematically review the study's "Failure Modes", "Near-Misses", "Open Questions", and "Recommendations" sections.
2. **Immediate Ticket Filing**:
   Every actionable bug, false positive/negative, usability pitfall, or architectural requirement identified MUST be filed immediately using `harnez issues new` in the affected repository.
3. **Cross-Repo Routing**:
   If the failure mode occurred in an external dependency, consumer, or sibling repository (e.g. engine bugs in `neus`, CLI/skill bugs in `harnez`, model serving bugs in `lmcoder`), the issue MUST be filed directly in the appropriate repository rather than grouped as an untracked comment.
4. **Reference Linking**:
   The filed issue MUST reference the originating study (with relative file links), and the study's post-mortem table MUST be updated to link directly to the newly allocated issue numbers.
5. **Phase Completion Gate**:
   A study or retrospective phase is NOT complete, and an epic cannot transition to closed, until all extracted issues are either:
   - Filed and committed as open tickets in the tracker, or
   - Explicitly listed as deliberate non-goals with written justification.

### 2.2 Skill & Documentation Updates

1. **`skills/epic/SKILL.md` & `skills/story/SKILL.md`**:
   - Add an explicit step to the Study/Retro phase: **Ticket Extraction & Backlog Sync Gate**.
   - Provide a template prompt/checklist for subagents:
     *"Have all failure modes, bugs, and required follow-ups from this study been filed as issues via `harnez issues new`? List each allocated ticket number."*
2. **`docs/AgenticLoop.md`**:
   - Update Phase 5 (Retro / Hygiene) to specify that retrospectives produce two deliverables:
     - The retrospective / case study document.
     - Concrete, tracked issues for every discovered bug or systemic friction.
3. **`docs/IssueTracking.md`**:
   - Document the lifecycle transition from study findings to tracked issues.

## 3. Implementation & Verification Plan

```markdown
/goal Formalize the Study & Retro Ticket Extraction invariant across harnez docs (AgenticLoop, IssueTracking) and skills (epic, story), requiring immediate issue filing for failure modes before closing studies or epics, or stop and report when blocked on a user decision or denied permission.
```

- **Tasks**:
  1. Update `docs/AgenticLoop.md` to define the Study/Retro Ticket Extraction Invariant in Phase 5 and Invariants summary.
  2. Update `docs/IssueTracking.md` with guidelines on converting study post-mortems into tickets.
  3. Update `skills/epic/SKILL.md` (and reference templates in `issues/904`) to add the Ticket Extraction Gate to epic workflows.
  4. Verify with linter / docs checks and commit.
