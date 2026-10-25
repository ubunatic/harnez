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

### M1: Dev & Advisor Design Review & User Challenge (Done)
- **Harness /goal adoption**: Slash commands cannot be executed directly by agents in chat UIs; the agent will set an internal durable goal in context and advise the user if a harness-level `/goal` command is desired.
- **Grouping heuristics**: Tickets must be grouped by thematic affinity (subsystem/component/session context), never pure blind priority concatenation. If affinity is ambiguous, propose and confirm.
- **Prefix & argument flexibility**: Support both leading modifiers (`file new /epic`, `run new /epic`, `next /epic`) and trailing subcommands (`/epic file`, `/epic run`, `/epic next`).
- **Autonomous safety boundaries**: Automode makes implementation decisions autonomously, but stops for user confirmation on irreversible destructive operations (deletions, breaking changes, canary bypasses).

### M2: Update Specification & Skill Definitions
- Update canonical documentation in `docs/commands/epic.md`.
- Update standard skill definitions (e.g. `.gemini/skills/epic/SKILL.md` and related cross-harness skill templates).
- Implement invocation dispatch rules:
  - Bare `/epic` without args (check open epics -> prompt or auto-group -> auto-goal -> run).
  - Modifier prefixes (`file new`, `run new`, `next`).
  - Automated ticket grouping logic based on priority, tags, and session context.

### M2 Pre-Work (host, 2026-10-25, from plan turn d918)

- Source is `docs/commands/epic.md` plus its `config.yaml` description only. The `.gemini/skills/...` path above
  is only the filing agent's name for it (owner): the skill must reach every target harness. `make install` and
  `harnez apply` render those copies from the source; check each rendered copy (Claude, Codex, agy) contains
  the new rules, and report any harness that does not get the epic skill.
- Open epics: `harnez find -d . issues -a status:active`, then keep `Category: Epic` (no category filter exists).
- Keep `/epic "<idea>"`, `/epic <n>`, `/epic <n> no-web` unchanged. Dispatch section at most ~25 lines; delegate,
  don't restate /issue or /lean-sprint.

M2 delivered (8b2b6b64): dispatch section in `docs/commands/epic.md`, new `config.yaml` description; rendered
copies in `~/.claude`, `~/.codex` and `~/.gemini` (agy) skills contain it; `go test ./internal/claude` and
`make lint` green. Host reviewed the diff. Open: M3 owner run of bare `/epic`.

### M3: Verification & Dry-Run
- Test bare `/epic` with open epics present (verify prompt).
- Test bare `/epic` with no open epics (verify grouping proposal and auto-run).
- Test modifier phrases (`file new /epic`, `next /epic`).
- Ensure compatibility across supported harnesses.
