# 589 agent: recommend one native background shell per agent run

Status: Closed — merged into 581
Priority: P2
Category: docs
Related: 581

## Recommendation

When a host (e.g. Claude Code) dispatches agents with `harnez agent start|resume`, run each call
as its own native background shell of the host tool (Claude Code: Bash with
`run_in_background: true`), not with `&`, `nohup`, `( … &)` or `--detach`/`--async`.

## Why (user feedback, neus sprints 2026-09-25)

The user called the pattern "really nice, it gives clarity on what runs":

- Every agent run, SSH tunnel and long build shows up as one entry in the host's shell list, so
  the user sees at a glance what runs (e.g. "4 shells: review, dev, index build, tunnel").
- The host gets a completion notification and a readable output file; no polling.
- Stopping is one action (TaskStop); no stray processes survive.
- Detached runs were invisible to the user and were rejected.

## Proposal

- State this as the recommended dispatch pattern in the agent docs and in `harnez agent start --help`,
  next to `--detach` (which should say it hides the run from the host UI).
- Pair with 581 (dispatch discoverability).

## Pitfall seen

Ctrl+C inside an agent's own shell did not stop a child `neus index`; the agent had to kill it.
Hosts should stop the whole background shell (process group), not rely on the agent.
