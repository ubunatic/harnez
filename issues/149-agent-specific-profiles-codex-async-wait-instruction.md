# 149 — Agent-specific instruction profiles, first use case: Codex async-wait guidance

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[144-codex-subagent-model-selection-policy]] (same class of problem — a Codex-only
correction with no home in the shared instruction set — likely becomes the second use case for
this same mechanism), [[151-on-demand-doc-lookup-vs-materialized-instructions-research]] (research
ticket asking whether this profile mechanism should also apply per-repository and temporarily, and
whether materialization itself is the right delivery model for generic docs — depends on this
ticket's mechanism existing first), [[130-instruction-distribution-audit-followups]] (prior distribution audit,
scoped to *uniform* content delivered to different targets, not *differentiated* per-agent
content), [[108-subagent-dispatch-sequential-default-and-issue-number-race-guard]], `config.yaml`
`agents_md` section

## Problem

harnez's `agents_md` distribution (`config.yaml`) currently sends the same instruction content to
every harness target — `~/.claude/CLAUDE.md`, `AGENTS.md` (read by Codex, Gemini, Prime Agent via
symlink/convention). There is no mechanism for a rule, skill, or command to apply to *one* agent
only. Every instruction added to the shared sections is either global noise for agents it doesn't
apply to, or has nowhere to live at all.

A concrete case surfaced this: Codex subagents lack Claude Code's smooth async subagent-handoff
flow. In Claude Code, dispatching a subagent lets the host stay responsive; when the subagent
finishes, the host is notified and can take over (or chain the next dispatch) without the host
polling for completion. Codex has no equivalent notification path for background jobs/subagents —
self-assessment showed Codex instead falls back to writing its own polling loop to wait on a
background task, which is exactly the anti-pattern `docs/practices/AgenticLoop.md`'s Zero Zombie
Guarantee and issue [[055]] (no long sleep, use scheduled wakeups) already warn against for other
contexts.

The needed correction — "do not poll for background job completion; use \<Codex's actual async
primitive\>" — is Codex-specific. Codex doesn't have the missing capability being warned about, so
telling Claude Code or agy "don't poll" would be inert advice that only adds noise to their already
correct behavior. This is the first concrete case where a *shared* instruction set is the wrong
shape, motivating a real per-agent profile mechanism rather than another shared-section bullet.

## Scope

1. **Design an agent-specific profile mechanism** in harnez's instruction-distribution system
   (`config.yaml`/`apply.go`), parallel to the existing shared `agents_md.global`/project sections,
   that can carry rules, skills, or commands scoped to exactly one target agent (Claude Code, agy,
   Codex, Gemini, Prime Agent) without appearing in the others' generated files.
2. **First concrete content**: a Codex-only instruction correcting the background-job/subagent-wait
   behavior — identify Codex's actual supported async-wait or notification primitive (if any) and
   instruct it to use that instead of a self-written polling loop; if Codex genuinely has no
   equivalent to Claude Code's subagent-completion notification, say so explicitly and give the
   least-bad fallback (e.g., a bounded/backoff poll rather than a tight loop) rather than leaving
   the gap unaddressed.
3. **Keep this out of the shared sections.** Do not add the correction to `agents_md.global` or any
   section that reaches Claude Code/agy — verify after implementation that neither's generated
   instructions changed.
4. Document the new mechanism in `docs/CLIDesign.md` or `docs/other/Spec.md` (whichever is the
   better fit) so future agent-specific corrections (starting with [[144]]) have a defined home
   instead of being bolted onto the shared sections out of convenience.

## Out of Scope

- [[144]]'s actual model-selection policy content — this ticket only establishes the mechanism;
  144 can migrate into it as a second use case once the mechanism exists, but is not blocked on
  this ticket and should not be implemented here.
- Building a general templating/inheritance system across all five agent targets — start with the
  minimum needed to scope one instruction to one agent; generalize later if a third use case shows
  the pattern repeating.
