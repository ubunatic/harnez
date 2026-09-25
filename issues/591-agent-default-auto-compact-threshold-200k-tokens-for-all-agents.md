# 591 agent: default auto-compact threshold 200k tokens for all agents

Status: Open
Priority: P2
Category: feature
Related: 590, 581

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

## Acceptance

- Default threshold 200k for every agent type; documented in `harnez agent --help` and agent docs.
- Override via config (and optionally a flag), shown in the session info line.
- Compaction completes before the next prompt is sent (depends on 590).
