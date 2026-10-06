# 736 — harnez-advisor skill is stale and pulls agents into heavy session-reuse flows when they only need a reviewer or a quick answer

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/commands/HarnezAdvisor.md` (skill source), `config.yaml` `skills:` and `debloat:`, issues/317 (split per-harness guidance into resources), `docs/commands/issue.md` (refers to "harnez-advisor lifecycle guidance"), `spec/agent.yaml` (roles `advisor`, `reviewer`)

---

## 1. Problem & Motivation

The owner reports that agents find the `harnez-advisor` skill and then try to spawn an agent and
run a complicated flow, when all they needed was a reviewer or a quick answer from another
model. The owner is unsure whether the skill is still up to date.

## 2. Technical Specification / Findings

Checked 2026-10-06 (uncertainties are marked):

- The skill can be loaded by agents on their own: the installed `SKILL.md` has no
  `disable-model-invocation`, and `~/.claude/settings.json` `skillOverrides` has no entry for it.
  Its description ("Reuse compatible advisor sessions across supported agent tools using prose
  requests") matches any "ask an advisor" intent.
- The body is mostly about session reuse: matching agent family, model, effort, project,
  permissions; compaction of stale advisors; token evidence labels. That pushes a quick question
  into a lifecycle procedure.
- Likely stale (last changed 2026-09-10/11): it describes each provider CLI's native resume
  (`codex exec -c model_reasoning_effort=...`, Prime unsupported) and says a future
  `harnez advisor` command does not exist yet. It does not mention `harnez agent start/resume`
  with `--role advisor|reviewer` and `--model provider:model:tier`, nor the MCP tools
  (`harnez_spawn_agent`, ...), which are how agents dispatch helpers today. Verify against
  `harnez agent --help` and `spec/agent.yaml` before changing anything.
- Not checked: which agents and sessions triggered it (telemetry or transcripts may show it).

Possible directions (for the implementer to decide, not fixed):

- Lead with the simple path: a quick answer or review = one `harnez agent start --role
  reviewer|advisor` call (or the native subagent), wait, read the reply, rate, delete. Session
  reuse and compaction only when the user asks for reuse.
- Narrow the description so it matches reuse requests only, or make the skill user-invocable-only
  via `debloat:` if agents keep misusing it.
- Fold or drop the per-provider CLI notes that `harnez agent` already covers (overlaps 317).

## 3. Implementation & Verification Plan

/goal The harnez-advisor skill matches current `harnez agent` usage, and an agent that needs a
reviewer or a quick answer takes the one-call path instead of a session-reuse procedure; verified
by reading the installed skill and by a dry run of two prompts ("get a quick second opinion on
X", "reuse my advisor from earlier"); or stop and report when blocked on an owner decision or a
denied permission.

Check the live skill, `harnez agent` help and recent commits first; this ticket may be stale.
