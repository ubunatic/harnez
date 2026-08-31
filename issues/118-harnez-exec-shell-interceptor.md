# 118 — `harnez exec`: shell execution interceptor with telemetry capture

**Status**: Closed — resolved in 27dca59
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

- [x] Wrapped command's stdout/stderr appear to the terminal/caller with
      no added buffering latency (canary-verified per `docs/other/Canary.md`
      before merging, since this is exactly the "reads back its own
      output" case the doc calls out).
- [x] Exit code matches the unwrapped command's exit code in a table
      test covering success, non-zero exit, and signal termination.
- [x] `duration_ms` and `raw_bytes` are recorded and roughly sane
      (verified against a command with known runtime/output size).
- [x] `distilled_bytes` is populated when distillation is active and
      left NULL otherwise, not zero — see "Distilled-bytes finding"
      below: `harnez exec` writes real NULL today (schema fixed to be
      nullable); populating an actual non-NULL value still needs a
      distill-side follow-up, tracked there, not blocking this AC.
- [x] Telemetry write failure (e.g. DB locked) does not block or delay
      the wrapped command's own execution or exit — telemetry is
      best-effort, never on the command's critical path.

## Notes

The "telemetry write must never block the wrapped command" criterion is
not explicit in the spec but follows from this project's zero-context-
bloat/low-overhead framing — a hung DB write turning every shell command
into a timeout risk would be a regression, not a feature. Confirm this
interpretation before implementing if it conflicts with how [[117]]
does its (synchronous) write.

## Canary result

`cmd/harnez/exec.go`'s `runExecWrapper` proxies stdio by assigning
`c.Stdin` directly (passed through as the child's underlying `*os.File`
fd, confirmed unbuffered) and by setting `c.Stdout`/`c.Stderr` to
`io.MultiWriter(callerStream, byteCounter)`. Two throwaway canary
programs (not checked in, per Canary.md's "throwaway file read back
immediately" form) verified both legs against a script that prints a
wall-clock timestamp, sleeps 0.3s, prints again, sleeps 0.3s, prints a
third time:

- Direct `*os.File` passthrough (`c.Stdout = os.Stdout`): the three
  printed timestamps came back staggered ~0.3s apart, matching the
  script's own sleeps — no batching/delay.
- `io.MultiWriter(os.Stdout, &counter)` passthrough (the actual approach
  used, needed to capture `raw_bytes` without losing real-time output):
  same staggered timing, plus an accurate combined byte count (63 bytes
  for the three timestamp lines). Go's non-`*os.File` `exec.Cmd` path
  copies via `io.Copy` as data arrives on the pipe, not on a timer or
  fixed-size batching, so no material added latency.

Conclusion: `os/exec` + `io.MultiWriter` passthrough is sufficient;
no custom unbuffered-writer plumbing needed.

## Distilled-bytes finding

`internal/distill` exposes no byte-count signal today: `distill.Distill`
returns a plain `string`, and neither `distill.go`'s wrapper
(`runDistillWrapper`) nor the `distill` package itself records or
returns a distilled-output length anywhere. `harnez exec` also has no
causal visibility into whether its own output was piped through
`harnez distill` downstream (e.g. `harnez exec --tool git -- git status
| harnez distill`) — that pipe stage runs in a separate process after
`exec` has already exited, with no channel back.

**Update (post-review, same day)**: `internal/telemetry.ToolCall.DistilledBytes`
was found to be a non-pointer `int64` backed by a `NOT NULL DEFAULT 0`
schema column, making the AC's "left NULL otherwise, not zero" wording
unimplementable as shipped. Since this is a shared-package schema
change, not `harnez exec`-specific or `internal/distill`-internals work,
it was fixed directly rather than deferred: `internal/telemetry`'s
`distilled_bytes` column is now nullable (`INTEGER CHECK (distilled_bytes
IS NULL OR distilled_bytes >= 0)`, no `NOT NULL DEFAULT 0`), and
`ToolCall.DistilledBytes` is now `*int64`. `Aggregate`'s `SUM`/`COALESCE`
needed no change (SQL `SUM` already ignores `NULL`s). `harnez exec` now
writes a genuine `nil`/SQL-`NULL` (not `0`) when no distill signal exists,
which is now representable. AC item above is satisfied for the
"left NULL, not zero" half.

What's still open, and correctly out of this ticket's scope: `harnez
exec` still cannot populate a real distilled_bytes *value* — that needs
`distill.Distill`/`runDistillWrapper` to report an output length, which
requires changing `internal/distill` itself (explicitly excluded by this
ticket's Scope: "does not change `harnez distill` itself"). Track that
as a small follow-up when a command that pipes through `harnez distill`
after `harnez exec` (or wires them together) is actually built.
