# Retrospective: Subagent Handoff Responsiveness and Load Memory Follow-Through

**Date**: 2026-08-28
**Author**: Codex (Pair Programming with User)
**Context**: Issues [[088-load-panel-ram-vram-gtt-memory]], [[089-load-panel-combine-gpu-vram-gtt-row]], and [[090-remote-load-panel-uses-local-metrics]]

## What Happened

This session started as a cost question about adding system RAM plus AMD VRAM/GTT to the `[L] Load`
panel. The key technical decision held: keep telemetry on kernel-standard procfs/sysfs reads only.
On the reference host, amdgpu exposes `mem_info_vram_*` and `mem_info_gtt_*`, so the feature could
be implemented without any `rocm-smi`, `nvidia-smi`, SDK, or other vendor-tool polling.

The implementation was correctly filed as issue `088`, delegated to a dev subagent, reviewed by the
host, adjusted after visual inspection, verified with `make test`, `make install`, and
`harnez usage --summary`, then committed. A follow-up UI decision became issue `089`: combine the
GPU memory presentation into one compact row shaped like:

```text
gpu vram/gtt  [██░░][█░░░] 9.7/19.6G 63%
```

One additional design gap surfaced while discussing remote hosts: remote mode currently fetches
remote agent usage via SSH, but the Load panel still reads local `/proc` and `/sys`. That is now
tracked as issue `090`.

## Orchestration Failure

After dispatching the dev subagent for issue `088`, the host immediately called a blocking wait.
That made the main chat less responsive even though the user's intent was only to hand off the task.
The user corrected the process expectation:

> When I say to hand a task to a subagent, it never means that you should block the main chat.

The rule is now captured in `AGENTS.md`, `docs/templates/AGENTS.md`,
`docs/practices/AgenticLoop.md`, `docs/AgenticLoop.md`, `commands/sprint.md`, and
`commands/fresh-sprint.md`: the host remains the responsive orchestrator. Handoff means dispatch
and stay available, not wait by default.

## Decisions

- Local Load memory telemetry remains cheap because it is plain Go file reads from `/proc` and
  `/sys`, not shelling out to `cat` or a vendor utility.
- VRAM and GTT should be visually distinguishable; summing them alone risks implying they are one
  interchangeable pool.
- The current preferred compact row still includes a combined used/total/percent, but makes the
  VRAM/GTT split visible via adjacent bars.
- Remote load telemetry should ride with the existing SSH `harnez usage --json` collection path, or
  another explicitly designed remote payload, rather than opening a 1 Hz SSH polling loop.

## What To Do Differently

- After spawning a subagent, report the handoff and return control to the user unless they asked to
  wait.
- If the child result arrives asynchronously, treat it as a notification to review/integrate when
  the host is not handling a newer user question.
- When UI output is the feature, run a real render (`harnez usage --summary` or a captured watch
  frame) before committing; the two-row GPU memory format passed tests but visual inspection showed
  the normal-width row clipped the GTT detail.
