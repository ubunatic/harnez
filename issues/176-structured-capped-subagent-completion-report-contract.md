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

---

## 5. Implementation Plan

Doc + prompt-convention change only, per §4. No Go code. The work is small; the value depends
entirely on the convention being cited from every dispatch site, so the plan is mostly about
wiring the references, not about writing the doc.

### Step 1 — Write `docs/practices/SubagentReporting.md`

Keep it under ~60 lines — a practice doc that itself blows the context budget would be
self-refuting. Content:

1. The contract from §2.1 as a literal, copy-pasteable block a dispatch prompt can paste verbatim
   (fields: files changed as `path:line-range`, commit hashes, test pass/fail counts, spec/doc
   changes, friction-only-if-nontrivial; 15-20 line ceiling).
2. The §2.2 rule stated as a directive: detailed findings go in the ticket file, chat carries the
   pointer. Cite `docs/practices/AgenticLoop.md` Invariant 4.
3. One "before/after" example — a real multi-paragraph report next to its capped form. This is the
   part that actually transfers; the field list alone will not.
4. An explicit non-goal, per §3.2: this is a convention, not a validated schema. Say so, so future
   readers do not go looking for enforcement that does not exist.

### Step 2 — Register it as a copyable practice doc

Add an entry to `config.yaml`'s `docs.practices` block (alongside `agentic-loop`,
`issue-tracking`, `concise-mode` at ~line 465-490):

```
    subagent-reporting:
      name: "Subagent Reporting Contract"
      ref: "@docs/SubagentReporting.md"
      hint: "capped 15-20 line completion report, fixed fields, detail goes to the ticket file"
      source: docs/practices/SubagentReporting.md
      target: ~/.claude/docs/SubagentReporting.md
      local: ./docs/SubagentReporting.md
      default: false
```

`default: false` — this is a workflow convention, not something every project should inherit
unasked, matching how `concise-mode` is registered.

### Step 3 — Wire the references (the step that makes it real)

1. `docs/practices/AgenticLoop.md`: add one line to Phase 1's and Phase 2's mechanics pointing at
   the new doc for report shape. Do not restate the contract there — a second copy will drift.
2. `commands/sprint.md`: amend the dispatch instructions (lines ~22 and ~40, advisor and reviewer
   spawns) to cite the contract.
3. `commands/fresh-sprint.md`: same at the subagent-spawn step (~line 21). Note §4 of that file
   already has "Calibrated Friction Reporting" — the new doc's friction field must be phrased to
   agree with it, not duplicate or contradict it. Check that wording before writing Step 1.3.
4. `config.yaml`'s `commands:` entries for `sprint` and `fresh-sprint` mirror these files — confirm
   whether the command content is inlined in `config.yaml` or sourced from `commands/*.md`, and
   update whichever is authoritative. Getting this wrong means `harnez apply` silently reverts the
   edit.

### Step 4 — Validate

Per §4: dispatch one real subagent under the new convention on an actual ticket, and check the
returned `<result>` block against the 15-20 line ceiling. Record the observed line count in this
ticket. One trial is enough to tell whether the phrasing lands; it is not a statistical claim.

### Key Decisions / Tradeoffs

- **Doc-only first, no `harnez report` command.** §3.1 raises it as a follow-up; keep it there. A
  storage-backed report command solves queryability, which is not the problem this ticket names —
  the problem is first-hop context cost, and a command does not reduce that.
- **Single source, referenced everywhere** rather than the contract text pasted into each skill.
  Costs an indirection at dispatch time; saves three copies drifting apart.
- **Accepting unenforceability.** A subagent can ignore the cap. The realistic win is moving the
  median report from ~5 paragraphs to ~15 lines, not eliminating the tail.

### Risks / Open Questions

- The real failure mode is that dispatch prompts written ad hoc (not via `/sprint`) never cite the
  doc, so the convention only binds the scripted paths. Mitigation would be putting a one-line
  version directly in `AGENTS.md` — but that pays a permanent system-prompt cost to fix an
  occasional one. Worth an explicit decision when this lands; recommend not doing it initially.
- Step 3.4 is the actual trap: if `config.yaml` inlines command content, editing `commands/*.md`
  alone does nothing after the next `apply`. Verify with `harnez diff` before committing.

### Scope

**Small** — one ~60-line doc, one config entry, four reference edits, one live validation run.
