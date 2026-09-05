# 145 — Add an orchestrator-session skill and command

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[144-codex-subagent-model-selection-policy]], [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], [[039-agentic-loop-practices-and-sprint-command]], [[064-fresh-handoff-workflow-skill-and-friction-reporting]]

---

## 1. Problem & Motivation

A session that is meant to coordinate delegated work needs an explicit,
reliable operating mode. The host should remain available in the user chat,
spawn subagents when the user requests delegation, and avoid blocking on
subagent completion unless an immediate integration step requires it.

Today those expectations are distributed across instructions and workflow
documents, making the host role easy to lose during a long session.

## 2. Technical Specification / Findings

- Provide a skill and/or command that explicitly activates orchestrator mode
  for the current session.
- The activated mode must state that the host remains responsive, delegates
  concrete user-requested subtasks, reports handoffs, and manages subagent
  lifecycle hygiene.
- It must not cause speculative delegation: user intent and normal task scope
  still govern when subagents are spawned.
- Define how the mode interacts with existing sprint and fresh-sprint
  workflows, including their non-blocking handoff rule.

## 3. Implementation & Verification Plan

- Choose a command, skill, or paired design consistent with existing runtime
  mode conventions.
- Add concise activation instructions and examples for user-requested
  delegation, status reporting, integration, and teardown.
- Test installation/discovery and confirm activation changes the session
  guidance without overwriting unrelated instructions.
- Exercise a controlled handoff to verify the host stays responsive and
  reports the selected subagent model and task scope.

---

## Implementation Plan

Current state: the host-role expectations already exist, but scattered — the
non-blocking-handoff and no-worktree rules live in `docs/templates/AGENTS.md`'s
"Background Tasks & Process Hygiene" section (and this repo's `CLAUDE.md`),
subagent lifecycle hygiene lives in `docs/practices/AgenticLoop.md` Phase 4, and
delegation shape lives in `commands/sprint.md` Phase 1. Nothing *activates* the
role for a session that is not running a full sprint. That is the gap.

### Steps

1. **New `commands/orchestrator.md`** (~30 lines, same shape as
   `commands/mode.md`): Invocation Syntax, Behavior, and an explicit
   "Not this" section. Content:
   - Host stays in the user chat and remains responsive; it does not block on a
     subagent unless the user asks it to wait or the next user-visible step
     genuinely cannot proceed (restating the AGENTS.md rule, one line, linking
     it rather than re-explaining).
   - On each handoff, report: what was delegated, to which agent type/model, and
     the scope boundary. (This is where [[144]]'s "report the selected model"
     requirement is satisfied for Claude Code.)
   - Delegate only what the user asked to delegate — no speculative fan-out.
   - Teardown: list and kill background tasks/subagents before ending
     (`AgenticLoop.md` Phase 4, linked not copied).
   - Relationship to `/sprint` and `/fresh-sprint`: orchestrator mode is the
     *ambient* host posture; `/sprint` is a bounded 5-phase workflow that
     assumes it. Running `/sprint` implies orchestrator mode; orchestrator mode
     does not start a sprint.
2. **`config.yaml` `skills:`** — add an `orchestrator` entry with
   `file: commands/orchestrator.md` and a trigger-shaped description ("use when
   the user asks to hand work to a subagent, coordinate delegated work, or run
   this session as a host/orchestrator"), following the `tool-feedback-protocol`
   entry's description style.
3. **Cross-reference, do not duplicate**: add one line to
   `docs/practices/AgenticLoop.md` §5 (Role Taxonomy) pointing the Host
   Orchestrator row at `/orchestrator`. Do not restate the rules there.
4. **Verify**: `harnez apply` to a scratch HOME + `harnez diff` clean; confirm
   the skill appears in Claude Code's skill list; extend
   `internal/claude/claudeskills_test.go` with the new entry if it asserts on
   the skill set. `go test ./...`.
5. **Exercise once**: run a real delegated task with the mode active and confirm
   the host reports the handoff and stays responsive.

### Design decisions

- Skill only, no separate slash command implementation needed — a `skills:`
  entry with a `file:` is already invocable as `/orchestrator`, the same way
  `mode`, `sprint`, and `fresh-sprint` are. One surface, not two.
- The skill *links* to the existing AGENTS.md/AgenticLoop rules rather than
  copying them. Copying would create the duplicated-instruction drift 128 is
  auditing, and the two copies would diverge on the next edit.
- No enforcement mechanism (no hook, no `harnez` subcommand). This is a posture,
  and a posture that only holds while a directive is in context; a persistence
  mechanism like `harnez mode`'s AGENTS.md rewrite is out of scope unless the
  role is observed to be lost mid-session in practice.

### Risks / open questions

- Overlap with `/sprint`: if the two are both active the guidance must not
  conflict. Step 1's explicit "sprint implies orchestrator, not vice versa" line
  is the mitigation — verify the sprint doc does not contradict it.
- Skill-trigger noise: "delegate"/"subagent" are common words; keep the
  description's trigger list concrete.

### Scope

Small (one new command doc, one config entry, one cross-reference, one test).
