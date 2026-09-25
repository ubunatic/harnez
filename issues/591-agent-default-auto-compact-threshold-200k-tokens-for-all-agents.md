# 591 agent: default auto-compact threshold 200k tokens for all agents

Status: Closed — pre-prompt compaction gate: global agent.compact_threshold_tokens (200k) on all dispatch paths, context = last-turn full input tokens, compact then verify ack + drop, else error; interactive over-limit gives clear next step (52e769d, cd22fe2, dc279d5); terra review; tests pass. Runtime watchdog: 592
Priority: P2
Category: feature
Related: 590, 581, 592

## Proposal (user request, 2026-09-25)

Set the DEFAULT auto-compact threshold for all `harnez agent` sessions (Codex, Claude, others) to
200k new tokens since the last compaction. Keep it configurable per agent/model and via config,
but 200k is the default.

## Why

- In the neus sprints, `neus-dev` (luna:med) ran to 2.2M new tokens before harnez queued a
  `/compact` on `harnez agent resume`. That late, large compaction triggered 590: the new prompt
  was lost and the agent redid its previous task.
- Long sessions cost more per turn (cached context still bills) and drift: stale task context
  competes with the new prompt.
- 200k keeps sessions focused on the current sprint while preserving enough recent context.

## Enforcement, not a request (user clarification)

Telling an agent "please compact" does not help unless harnez verifies it. The threshold must be:

- **Global:** in the harnez global settings (`~/.harnez/config.yaml`), default 200k, applied to
  every agent type and every dispatch (start, resume, sprint), not per prompt.
- **Programmatic:** harnez checks the session's token count before sending each prompt. Over the
  threshold, harnez itself triggers compaction, then **verifies** it happened (compaction ack plus
  a drop in context tokens) before sending the prompt. If it cannot verify, it fails loudly or
  starts a fresh session with a handoff summary; it never sends the prompt into an uncompacted
  session silently.
- Sending a compact instruction to the agent is fine only as the mechanism, when the check above
  confirms the agent actually compacted.

## Acceptance

- Global default 200k in harnez config for all agent types; documented in `harnez agent --help`.
- Pre-prompt check enforces it on every dispatch path; covered by tests.
- Compaction is verified before the next prompt is sent (fixes the 590 race); unverified
  compaction is an error or a fresh-session fallback, visible in the session info line.
- Per-agent override possible, but the global default applies when unset.
- Apply the pre-prompt check before every dispatch path, including CLI start/resume and MCP
  spawn/resume; build on 590's wait-for-completion behavior (commits `1c4a460` and `f4cd844`).
