# Review Changes & Verify Completion

Triggered by `/review [issue-number|issue-path]`.

## Workflow

1. **Review Current Changes**:
   - Inspect the working tree diff and status: `git status`, `git diff`, and `git diff --staged` (or `git diff main...HEAD`).
   - Identify modified files, scope of changes, and any unexpected or uncommitted files.
   - Use range-bounded reads or targeted grep for context discipline on large changes.

2. **Verify Issue Completion (if issue provided)**:
   - If a ticket number or path is specified (or in active context):
     - Read the ticket (`issues/<n>-*.md` or `harnez issues show -d <repo> <n> -I`).
     - Check all acceptance criteria, milestones, and requirements against the implementation.
     - Confirm test coverage verifies the expected behavior.
     - Check documentation updates (evergreen docs, `docs/README.md`) and tracker synchronization (`issues/*.md` status, `issues/README.md` index).

3. **Check Code Quality**:
   - **Assertion Rigor**: Verify unit and integration tests are meaningful and test real failure modes.
   - **Cleanliness & Minimalism**: Remove dead code, redundant comments, leftover debug prints, and temporary scratch files.
   - **Invariants & CLI Design**: Ensure architectural rules and command boundaries (e.g., `apply` vs `init`) are respected.
   - **Root Cause vs. Symptom**: Verify defensive checks fix the actual source rather than masking underlying errors.
   - **Build & Tests**: Run `make test-q1` or `go test ./...` to ensure clean compilation and passing test suite.

4. **Do Small Fixes Directly**:
   - Apply minor fixes directly in the working tree (e.g. typos, simple doc/comment corrections, small 3–4 line fixes, missing nil checks, trivial lint issues).
   - Re-verify tests after making changes to keep the build green.

5. **File Bigger Issues as New Issues (see `/issue` skill)**:
   - Do not bloat or stall the current review with out-of-scope refactorings, large architectural gaps, or new feature requests.
   - File follow-up issues using the `/issue` skill conventions:
     - Search existing issues for duplicates: `harnez find -d <repo> issues "<search terms>" -I`.
     - Reserve and draft the ticket: `harnez issues new -d <repo> "<title>"`.
     - Fill in problem, scope, acceptance criteria, and verification targets.
     - Open and commit: `harnez issues open -d <repo> <n> --commit "docs(issues): file <n>, <summary>"`.

6. **Summary & Handoff**:
   - Present a concise, structured summary:
     - **Issue Status**: Complete / Incomplete / N/A (with breakdown).
     - **Quality Findings**: Notes on code hygiene, invariants, and test rigor.
     - **Fixes Applied**: Direct minor edits made during review.
     - **Follow-Up Tickets Filed**: New issue numbers and titles (if any).
   - Name every ticket or milestone at least once with a short label ("498 (Claude resume)"), not a bare number (`@docs/AgenticLoop.md` §4, Status reports).
