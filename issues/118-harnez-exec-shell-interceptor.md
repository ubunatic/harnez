# 118 — `harnez exec`: shell execution interceptor with telemetry capture

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[116-tool-telemetry-schema-and-storage-layer]], [[119-harnez-hook-agent-hook-management]], [[121-multi-repo-session-and-ticket-id-resolution]]

## Problem

Shell/CLI tool executions aren't explicitly rated by the agent the way
internal tools are — the spec wants these transparently intercepted and
logged. `harnez exec` wraps an arbitrary command, proxies it unbuffered,
preserves its exit code, and records execution metrics.

## Scope

Command signature:

```
harnez exec --tool <tool_name> [--ticket <ticket_id>] -- <command...>
```

- Spawns `<command...>` as a subprocess; proxies stdin/stdout/stderr
  unbuffered (no line-buffering delay — verify with a canary against a
  command that streams slowly, e.g. `yes | head`, before assuming Go's
  `os/exec` pipe defaults are sufficient).
- Preserves and re-exits with the child's native exit code, including
  signal-terminated cases.
- Captures: wall-clock `duration_ms`, raw stdout+stderr byte count
  (`raw_bytes`), and — when the wrapped command's output was piped
  through `harnez distill` — the distilled byte count (`distilled_bytes`).
  This ticket does not change `harnez distill` itself; it only reads
  whatever byte-count signal distill already exposes or can be made to
  expose with a minimal addition.
- Writes one row via [[116]] with `call_type = 'shell'`.
- Also add `harnez exec hook` (mirroring `harnez distill hook`,
  `cmd/harnez/distill.go`'s `newDistillHookCmd`/`runDistillHook`): a
  stdin-driven endpoint that reads an agent's tool-use hook payload and
  rewrites it to route through `harnez exec --tool ... -- <command>`.
  This is the target [[119]]'s `apply`-managed hook entries actually
  invoke — the direct `harnez exec --tool ... -- <command>` form above
  is what the *rewritten* command runs as (and what manual/scripted use
  calls directly); the `hook` verb itself never runs the child process
  or writes telemetry. See `docs/HookRewritePattern.md` for the general
  shape and why the two stages can't be merged into one command.

## Acceptance Criteria

- [ ] Wrapped command's stdout/stderr appear to the terminal/caller with
      no added buffering latency (canary-verified per `docs/other/Canary.md`
      before merging, since this is exactly the "reads back its own
      output" case the doc calls out).
- [ ] Exit code matches the unwrapped command's exit code in a table
      test covering success, non-zero exit, and signal termination.
- [ ] `duration_ms` and `raw_bytes` are recorded and roughly sane
      (verified against a command with known runtime/output size).
- [ ] `distilled_bytes` is populated when distillation is active and
      left NULL otherwise, not zero (spec's schema marks it nullable).
- [ ] Telemetry write failure (e.g. DB locked) does not block or delay
      the wrapped command's own execution or exit — telemetry is
      best-effort, never on the command's critical path.

## Notes

The "telemetry write must never block the wrapped command" criterion is
not explicit in the spec but follows from this project's zero-context-
bloat/low-overhead framing — a hung DB write turning every shell command
into a timeout risk would be a regression, not a feature. Confirm this
interpretation before implementing if it conflicts with how [[117]]
does its (synchronous) write.
