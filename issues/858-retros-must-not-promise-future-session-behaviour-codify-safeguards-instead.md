# 858 — Retros must not promise future-session behaviour; codify safeguards instead

**Status**: Closed — Retros now name durable safeguards and forbid future-session promises; package tests and full suite pass.
**Priority**: P2 (Medium) — false promises hide that nothing was fixed
**Severity**: Minor
**Category**: Workflow
**Related**: docs/AgenticLoop.md (Phase 5, "Ephemeral-Only Retrospectives"), docs/commands/evergreen.md
("tell the user what you would do differently"), docs/commands/Wrap.md

---

/goal Harnez guidance (AgenticLoop Phase 5, the evergreen and wrap commands, and the shared output rules)
tells agents not to promise behaviour for later sessions ("in future I will", "next time I won't"), limits
any promise to this session or the next steps, and turns each lesson into a durable safeguard (test,
command, make target, hook, check script, or at least a prompt/rule line) or a ticket for one; verified by
one retro run whose report contains no cross-session promise; or stop and report when blocked on a user
decision.

## Problem (user, 2026-10-22)

After a retro, agents say "in future I'll …" or "next time I won't …". Sessions are ephemeral, and even
compaction in the same session can drop instructions and the agent's own promises, so these promises
cannot be kept. They also hide that no guardrail was added.

Example (loom /evergreen, 2026-10-22): "In future I'll time a wait helper once before relying on it" and
"Next time I'll require 'failed before the fix'". Only a prompt line or a check would make either stick.

## Guidance to add

- Allowed scopes: "in this session I will …", "as a next step I will …".
- For a lasting lesson, name the safeguard instead of a promise, in order of preference:
  non-LLM checks (test, lint, make target, script, hook, CLI check) > rule/prompt line > ticket to build one.
- Rephrase the evergreen prompt "tell the user what you would do differently" so it asks for the
  safeguard (done or proposed), not a resolution.

- Wording: say "promise" or "plan", not "commit"/"commitment", which readers take for git commits.

## Open question

- Can a cheap check (e.g. a harnez output lint or hook) flag phrases like "in the future I will" /
  "next time I" in final reports?
