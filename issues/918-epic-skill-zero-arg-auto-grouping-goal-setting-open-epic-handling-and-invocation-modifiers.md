# 918 — epic skill zero-arg auto-grouping, goal-setting, open epic handling and invocation modifiers

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 900 (epic skill), 904 (epic skill evaluation), docs/commands/epic.md

---

/goal Enhance the `epic` skill (`docs/commands/epic.md` and installed skill definitions) to support zero-argument smart invocations, conversational modifier prefixes, automatic ticket grouping, autonomous goal setting, and open-epic disambiguation; preceded by a dev/advisor review milestone that analyzes and challenges the initial design before locking it in; or stop and report on unresolved architectural trade-offs. Circuit breaker: on a loop, unresolvable harness limitation, or no progress, clear the goal and record a failure report here.

## 1. Motivation & Context

Currently, `/epic` expects explicit arguments (`/epic "<idea>"`, `/epic <n>`, or `/epic <n> no-web`). When invoked with no arguments (`/epic`), the workflow halts or requires interactive prompting.

The desired user experience allows flexible, natural invocations where the orchestrator takes initiative while maintaining safety rails:
1. **Zero-arg `/epic`**:
   - Inspects repository status and active backlog.
   - Intelligently groups currently discussed or high-value/high-priority tickets into a cohesive epic.
   - Evaluates whether prior-art web search and code refinement are necessary.
   - Launches execution in automode, making implementation choices autonomously.
   - Adopts the epic as an active `/goal` without requiring manual user `/goal` invocation (subject to harness capabilities).
   - **Open epic collision check**: If one or more open epics already exist, proactively asks the user whether to run an existing open epic or group a new one.
2. **Invocation Modifiers / Prefixes**:
   - `file new /epic`: Draft and file the epic ticket only; do not start execution.
   - `run new /epic`: Draft, file, and immediately execute.
   - `next /epic`: Identify and execute the next logical epic based on context, priority, or ticket number.
3. **Design Review & Challenge Gate**:
   - Dev and advisor must review this design and challenge edge cases, failure modes, and ergonomics before committing changes.

---

## 2. Milestones

### M1: Dev & Advisor Design Review & User Challenge
- Review parameterization syntax and natural-language prefix variations (`file new /epic`, `run new /epic`, `next /epic`, bare `/epic`).
- Investigate feasibility and mechanism of programmatic `/goal` adoption across harnesses (Antigravity slash command vs harness state vs goal prompt convention).
- Analyze risks and define guardrails:
  - Ticket grouping heuristics (how to avoid grouping unrelated high-prio bugs into an incoherent epic).
  - Autonomous decision boundaries (when should automode ask vs decide).
  - Open epic disambiguation prompts (interactive TTY vs script/headless environments).
- Formulate concrete challenges and alternative proposals for user alignment.

### M2: Update Specification & Skill Definitions
- Update canonical documentation in `docs/commands/epic.md`.
- Update standard skill definitions (e.g. `.gemini/skills/epic/SKILL.md` and related cross-harness skill templates).
- Implement invocation dispatch rules:
  - Bare `/epic` without args (check open epics -> prompt or auto-group -> auto-goal -> run).
  - Modifier prefixes (`file new`, `run new`, `next`).
  - Automated ticket grouping logic based on priority, tags, and session context.

### M3: Verification & Dry-Run
- Test bare `/epic` with open epics present (verify prompt).
- Test bare `/epic` with no open epics (verify grouping proposal and auto-run).
- Test modifier phrases (`file new /epic`, `next /epic`).
- Ensure compatibility across supported harnesses.
