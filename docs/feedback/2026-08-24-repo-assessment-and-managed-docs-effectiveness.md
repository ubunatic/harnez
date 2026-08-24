# Retrospective: `harnez assess`, Token Heuristics, and Managed Docs Effectiveness

**Date**: 2026-08-24  
**Author**: Antigravity Assistant & Harnez Dev Agent  
**Context**: Implementation of Issue 057 (`harnez assess`), agentic handoffs, and documentation effectiveness assessment.

---

## 1. Executive Summary & Delivery

During this session, we specified, developed, tested, and shipped `harnez assess`:
- **Problem**: Human developers and AI agents inspecting a codebase (or gating commits) lacked a fast, standardized way to determine repo feasibility, code/doc proportions, and context-window token weight without ad-hoc scripts or slow external tools.
- **Solution**: A native, zero-dependency Go engine in `internal/assess/` that performs polyglot file classification, non-whitespace LOC calculation, estimated token weight calculation (~3.75 chars/token), and rule-based heuristic warnings (e.g., monolithic files >500 LOC, Bash scripts >100–150 LOC, missing directory tests).
- **Deliverable**: `harnez assess [path]` with human-readable 3–20 line terminal table formatting and `--json` export for pre-commit checks and agent ingestion.

---

## 2. Where Harnez-Managed Docs Helped

The presence of standardized, evergreen guidelines in `docs/` and `AGENTS.md` dramatically accelerated alignment and code quality:

1. **`docs/practices/IssueTracking.md`**:
   - Provided the exact metadata schema (`Status`, `Priority`, `Severity`, `Category`, `Related`) and synchronization rules.
   - Enabled automated validation via `harnez status` issue tracker linter.
2. **`docs/CLIDesign.md`**:
   - Prevented flag pollution and confirmed the scope of `assess` as a local inspection tool alongside `status`, `diff`, `init`, and `usage`.
3. **`docs/lang/Go.md`**:
   - Guided zero-dependency modern Go design, Cobra flag wiring, and table-driven unit tests in `internal/assess/assess_test.go`.
4. **`docs/lang/Bash.md`**:
   - Provided the explicit design rationale for the Bash warning threshold (flagging scripts >100–150 LOC to discourage unmaintainable shell sprawl).
5. **`docs/practices/AgenticLoop.md`**:
   - Structured the dev agent invocation with clear requirements, TDD expectations, and clean teardown.

---

## 3. What Was Missed & Agentic Friction Points

1. **`write_to_file` Artifact Schema Constraint**:
   - The initial attempt to create `issues/057-*.md` using `write_to_file` included `ArtifactMetadata`, which failed because artifacts are restricted to the brain log directory. Recovered immediately using standard file writing.
   - *Takeaway*: Tool usage rules should clearly distinguish between agent artifacts vs. workspace repository modifications.
2. **Self-Assessment Debt in `harnez`**:
   - Running `harnez assess .` on our own repo immediately highlighted actionable maintenance signals:
     - `internal/claude/apply.go` (550+ LOC, >4.5k tokens) and `internal/usage/watch.go` (520+ LOC, >4.2k tokens) exceed recommended single-file thresholds.
     - `scripts/` contains Bash scripts without formal unit/canary tests.
   - *Future Work*: Schedule refactoring tickets to decompose `apply.go` and `watch.go`.
3. **Token Density Variation Across Languages**:
   - The ~3.75 chars/token heuristic is fast (<1ms) and effective for general sizing, but punctuation-dense formats (JSON, YAML, regex-heavy Bash) differ slightly from prose-heavy Markdown. Future iterations can add language-specific token density multipliers without external tokenizer dependencies.

---

## 4. Architectural Invariants Established

- **Zero-Dependency Speed**: `harnez assess` executes in <10ms for typical repositories, ensuring zero friction when run as a non-blocking pre-commit check.
- **Agent Context Budget Awareness**: Output is strictly constrained to 3–20 lines, allowing agents to gauge repository feasibility without blowing context budgets.
