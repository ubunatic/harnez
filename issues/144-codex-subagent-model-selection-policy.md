# 144 — Ensure Codex uses the correct model for subagents

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], [[149-agent-specific-profiles-codex-async-wait-instruction]] (establishes the per-agent profile mechanism this policy's content likely migrates into), `AGENTS.md`, Codex subagent dispatch configuration, #479

---

## 1. Problem & Motivation

Codex subagents can inherit the host model unless a model is explicitly set.
That can assign an unsuitable model to implementation, review, or advisory
work and makes the effective model choice opaque to the user.

## 2. Technical Specification / Findings

- Define a project policy for choosing a subagent model and reasoning effort
  by task class: implementation, review, focused investigation, and simple
  mechanical work.
- The dispatcher must explicitly select the intended model when the policy
  calls for one, rather than relying on accidental inheritance.
- User-requested model overrides take precedence and must be reported when a
  subagent is dispatched.

## 3. Implementation & Verification Plan

- Document the selection policy in the canonical agent instructions or a
  referenced practice document.
- Add a dispatch helper or guardrail where supported, so implementation work
  receives the policy's coding-capable model and effort level.
- Verify spawned-agent metadata records the requested model and effort, and
  cover fallback behavior when an explicit model is unavailable.
- Keep routine tasks economical while reserving stronger coding models for
  cross-layer debugging and higher-risk implementation work.

---

## Implementation Plan

**Sequencing**: this is the second use case for [[149]]'s per-agent instruction
profile mechanism (149 says so explicitly, and 149 is still Open). Writing a
Codex-only model policy into a shared `agents_md` section before 149 lands would
put Codex-specific noise into Claude Code's and agy's instruction stacks — the
exact cost 128 is measuring. So: **do not start the content work until 149's
profile mechanism exists**; the research step below can be done now and is
useful regardless.

### Steps

1. **(Do now, independent of 149) Establish the facts.** Determine, empirically
   rather than from memory:
   - Whether Codex subagent dispatch actually inherits the host model when no
     model is named, and what the observable default is.
   - Whether Codex's dispatch surface accepts an explicit model/effort argument
     at all, and whether the chosen model is reported back in spawned-agent
     metadata.
   Record the findings in this ticket. Without them the "policy" is a
   plausible-sounding theory of the kind [[045]] warns about.
2. **Draft the policy** as a short table by task class — implementation, review,
   focused investigation, mechanical/routine — mapping each to a model tier and
   reasoning effort, plus two standing rules: (a) always name the model
   explicitly rather than relying on inheritance, (b) a user-requested override
   wins and must be stated in the handoff message. Keep it under ~10 lines;
   this is instruction-stack content.
3. **(After 149) Place it in the Codex profile** created by 149's mechanism in
   `config.yaml`, delivered via `codex_skills_target` / the Codex `AGENTS.md`
   path. Verify Claude Code's and agy's generated files are byte-identical
   before and after (149 Scope item 3's check applies here too).
4. **Verification**: dispatch one real Codex subagent per task class and confirm
   the reported model matches the policy, plus one run with an unavailable model
   name to observe the fallback. Record actual observed behaviour, not intent.
5. Cross-link: once landed, note in [[149]] that its second use case is done.

### Design decisions

- Policy as *instruction content in a per-agent profile*, not as a `harnez`
  dispatch helper. harnez does not sit in Codex's dispatch path and building a
  wrapper for it would be a new integration surface for one rule. Revisit only
  if step 1 shows Codex cannot be steered by instruction at all.
- Task-class table rather than per-ticket model choice: keeps routine work cheap
  without requiring a decision each dispatch.

### Risks / open questions

- Codex's model catalogue and dispatch API change independently of this repo; a
  hardcoded model name will rot. Prefer naming a *tier* ("the strongest
  coding-capable model available") over a specific model id where the dispatch
  surface allows it.
- If step 1 finds Codex already selects sensibly by default, the right outcome
  may be to close this ticket as a non-problem — decide after step 1, before
  writing any content.

### Scope

Small (content only) once 149 lands; Medium including the step-1 investigation.
Blocked on 149 for steps 3–5.

## Epic note (#479)

#484 supplies the fallback default model (`codex:luna:low`) when `--model` is omitted; this ticket still decides which model fits which task type.
