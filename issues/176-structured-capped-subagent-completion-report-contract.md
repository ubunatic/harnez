# 176 — Structured, Capped Subagent Completion-Report Contract

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[142-disable-rate-feedback-and-measure-overhead]] (same underlying concern — token
overhead of harness feedback mechanisms — but scoped to `harnez rate`'s per-tool-call instruction
injection, not this ticket's target),
[[066-native-go-command-output-distillation]] and
[[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]] (distill: rewrites *Bash tool-call*
output via a PreToolUse hook — does not and cannot reach this ticket's target, since a subagent's
final chat message is not a tool call), `docs/practices/AgenticLoop.md` (5-phase sprint workflow,
subagent dispatch), `docs/studies/2026-09-01-usage-watch-startup-splash-and-usage-flag-redesign.md`
§3 (where this gap was first surfaced)

---

## 1. Problem & Motivation

When a subagent (dispatched via the `Agent` tool) finishes, only its final free-form chat message
reaches the orchestrator, via a `task-notification`'s `<result>` block — the full tool-call
transcript stays in a local file the orchestrator is explicitly told never to read. That final
message currently has no length cap or structure: it is shaped entirely by however the dispatch
prompt phrased "report back," and in practice runs to multiple paragraphs (file lists, mechanism
explanations, verification narratives) that land in the orchestrator's context verbatim.

The orchestrator then has to manually write a *second*, shorter summary of that report for the
user, since a subagent's raw report is never shown to the user directly. So the current path pays
the full cost of an uncapped report at the first hop (into orchestrator context) and only
compresses at the second hop (orchestrator to user) — the expensive part is not the part that's
actually cut.

Neither existing harness token-efficiency mechanism reaches this:

- `harnez rate` scores individual tool calls, not a subagent's closing message.
- `harnez distill`'s autopipe hooks rewrite *Bash tool-call* output within one agent's own session
  (issues 066/070) — a subagent's final report is not a tool call at all, so distill has no
  attachment point for it.

This was first identified while auditing a session's context-cost drivers (see the related study
doc §3) as the single largest recurring contributor that session, ahead of both existing
mechanisms' scope.

## 2. Proposed Direction

1. **A documented, capped report contract** for what a dispatch prompt asks a subagent to return,
   replacing open-ended "report back: ..." phrasing. Fixed fields only, no narrative prose:
   - Files changed (path:line-range)
   - Commit hashes
   - Test result (pass/fail counts only, not full test output)
   - Spec/doc changes (if any)
   - Friction (only if genuinely non-trivial — omit otherwise, per this project's existing
     "calibrated friction reporting" convention)
   - Target: roughly a 15–20 line ceiling.
2. **Narrative detail moves to the ticket file, not chat.** This project already treats
   `issues/*.md` as the durable, git-tracked home for implementation notes and verification
   findings (Invariant 4, `docs/practices/AgenticLoop.md`: "In-Repository Single Source of Truth").
   Instructing subagents to write detailed findings there instead of in their chat-facing report
   keeps the detail available without double-paying for it in orchestrator context.
3. Document this as a new short practice doc (e.g. `docs/practices/SubagentReporting.md`),
   referenced from `AgenticLoop.md` and the `sprint`/`fresh-sprint` skills, so every dispatch
   prompt across this project's workflows can cite one canonical convention instead of each
   dispatch prompt inventing its own report phrasing.

## 3. Open Questions / Follow-Up Scope (not required for this ticket)

- Whether a `harnez report` command (mirroring `rate`/`stats`'s storage) should eventually make
  completion signals queryable centrally instead of living only in chat scrollback — noted as a
  possible follow-up, not a prerequisite for the doc/prompt-convention fix above.
- This contract cannot be mechanically enforced (a subagent is still a model following prompt
  instructions, not a validated schema) — treat it as a strong convention with citable text, not a
  hard guarantee, same as this project's other prompt-level conventions.

## 4. Verification Plan

- Write `docs/practices/SubagentReporting.md` with the capped-field contract from §2.
- Reference it from `docs/practices/AgenticLoop.md`'s Phase 1/Phase 2 dispatch guidance and from
  the `sprint`/`fresh-sprint` skill definitions.
- No code changes required for the initial doc-convention version; validate by dispatching a real
  subagent under the new convention and confirming its `<result>` stays within the target line
  ceiling while the ticket file still carries full detail.
