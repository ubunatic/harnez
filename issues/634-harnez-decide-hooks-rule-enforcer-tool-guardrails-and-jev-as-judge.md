# 634 — harnez decide: hooks & rule enforcer, tool guardrails, and Jev-as-judge

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/633, internal/claude/hooks.go, docs/practices/AgenticLoop.md, docs/CLIDesign.md

---

## 1. Problem & Motivation
Agent workflows in Claude Code, Codex, and other harnesses can perform unintended or destructive operations:
- Executing destructive or irreversible terminal commands (e.g. `rm -rf`, force pushes, unverified package installations).
- Violating project-specific or file-level guidelines (e.g. touching forbidden paths like `src/generated`, adding unapproved dependencies, violating style/API rules).
- Writing code without accompanying tests (or failing to cover documented requirements).
- Running long, expensive review passes on trivial changes when a quick pre-filter would suffice.

Currently, safety hooks either rely on brittle regexes or heavy LLM invocations.
Using the `harnez decide` engine (issue 633), we can implement high-speed (~300ms), low-cost guardrails and checks as agent hooks and skills.

## 2. Technical Specification / Findings

Drawing from "10 Levels of Jev" (Levels 1, 4, 6) and the video summary:

### 1. Hook: Tool Call Guardrails (Bash & File Operations)
- Hook type: Pre-tool execution hook (e.g. Claude Code PreToolUse hook, Codex filter).
- Behavior:
  - Intercept commands before execution.
  - Query decision model:
    - `irreversible`: is this action irreversible or destructive?
    - `confidence`: confidence score.
  - Policy:
    - Irreversible with high confidence (>0.85) -> block command, inform agent why.
    - Ambiguous confidence (<0.50) -> prompt human for interactive confirmation.
    - Read-only / safe -> immediate pass-through.

### 2. Hook: Rule Enforcer on File Edits
- Hook type: Pre/Post file edit hook.
- Given the file being edited and the relevant section of `AGENTS.md` / `rules/*.md`:
  - Decision model evaluates: "Does this patch/edit violate any stated project constraints?"
  - If probability > 0.80 -> block edit with explanation of which specific rule was broken.

### 3. Skill / Reviewer: Jev as a Judge & Pre-Review Triage
- **Jev as a Judge (Access Control & Rules vs Tests)**:
  - Scans updated code and rules/doc specifications.
  - For each rule, queries whether an explicit test case covers it.
  - Outputs missing test list for the agent to implement before committing.
- **Cheap First Pass Reviewer**:
  - Before spawning a heavy subagent review or human review gate (AgenticLoop Phase 3), run 5-7 targeted yes/no evaluation questions on the diff:
    1. Does it touch security-sensitive code?
    2. Does it add external dependencies?
    3. Is there high algorithmic complexity or risk?
    4. Are there schema/database migrations?
    5. Does it follow conventional commit / code style?
  - Composite score calculated in code with explicit weights (Level 3 composite scoring).
  - Score < 0.3 -> fast-track auto-approval.
  - Score >= 0.3 -> route to full reviewer agent / human gate.

## 3. Implementation & Verification Plan
1. **Hook Integrations**:
   - Add hook script templates to `docs/templates/` or `internal/claude/` that call `harnez decide`.
   - Installable via `harnez apply` or as a reusable harnez plugin/pack.
2. **Review Triage Skill**:
   - Provide `/pre-review` or integrate into `sprint` phase 3: run `harnez decide` against `git diff`.
3. **Verification**:
   - Unit and integration tests in `test/decide_hooks_test.go`.
   - Test against sample destructive commands (`rm -rf`, `git push -f`) and safe commands (`ls`, `cargo test`).
   - Test against edits violating canary rules.
