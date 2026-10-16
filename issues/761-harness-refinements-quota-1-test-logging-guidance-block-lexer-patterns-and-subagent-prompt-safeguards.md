# 761 — Harness refinements: Quota-1 test logging guidance, block lexer patterns, and subagent prompt safeguards

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**:

---

## 1. Problem & Motivation
Learnings from rapid multi-model lean sprints on Loom (2026-10-09 session covering issues 306–311) highlighted several areas where Harnez agent prompts, rules, and guidance can be hardened to reduce harness friction and prevent subtle subagent rule breaches:

1. **Quota-1 Logging / `tail` Traps**: Under Quota-1 rules, developers are instructed to capture test output to a file and grep for `--- FAIL`. In practice, worker models occasionally pipe `make test-q1` directly into `tail`, masking intermediate failure lines and exit codes, or causing secondary quota-lockouts when trying to inspect results.
2. **Block Lexer & Tag Parser Conventions**: As formats like `.tui` evolve with custom block syntax (`[box]`, `[table]`, `[image]`), having standard agentic guidance on shared `key=value` lexer decomposition and functional option hooks in Go prevents repetitive parser boilerplate across client repos.
3. **Subagent Prompt Safeguards on Editing Tools**: Workers occasionally resort to heredocs or shell `sed` for quick fixes when encountering strict file-editing constraints, risking syntax corruption. Developer prompts should reinforce native structured patch usage.

## 2. Technical Specification / Findings
- Audit `.harnez/prompts/developer.md` and `rules/Quota.md`:
  - Strengthen Quota-1 command recipes to explicitly warn against piping to `tail`, `head`, or subshells that lose `$?`.
  - Provide a standardized test execution pattern:
    ```bash
    make test-q1 > /tmp/test.log 2>&1
    grep -E -- "--- FAIL|FAIL:" /tmp/test.log || true
    ```
- Document the functional render hook pattern in Harnez architectural guidance for heavy dependencies (e.g. image decoders or syntax engines) to prevent root package dependency bloat.
- Review developer prompt constraints regarding shell editing workarounds (`sed`, heredocs).

## 3. Implementation & Verification Plan
1. Update `.harnez/prompts/developer.md` and `.harnez/rules/Quota.md` with explicit test execution and logging guardrails.
2. Verify updated rules against `harnez check` and test on a synthetic sprint.
3. Document patterns in Harnez developer docs.

**/goal** Refine Harnez developer prompt guidelines, Quota-1 test logging recipes, and subagent editing safeguards based on Loom sprint findings, or stop and report if blocked.
