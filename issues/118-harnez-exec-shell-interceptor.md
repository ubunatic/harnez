# 118 — `harnez exec`: shell execution interceptor with telemetry capture

**Status**: Closed — resolved in 3b1e6a1
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
      below: implemented as always-0, documented deviation.
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

Additionally, `internal/telemetry.ToolCall.DistilledBytes` (issue 116)
is a non-pointer `int64` backed by a `NOT NULL DEFAULT 0` schema column
— there is no way to write SQL `NULL` into it with the current type/
schema, only `0`. So even setting aside the missing distill signal,
this ticket's "left NULL otherwise, not zero" AC text is unimplementable
against the schema issue 116 already shipped.

Decision: `harnez exec` always writes `DistilledBytes: 0` and documents
this deviation rather than reopening telemetry's schema or reaching into
distill internals (explicitly out of scope per this ticket's Scope
section — "does not change `harnez distill` itself"). Minimal follow-up
needed in a future ticket: (1) have `distill.Distill`/the wrapper report
its output length somehow, and (2) decide whether `distilled_bytes`
should become a nullable `*int64` in `internal/telemetry` to distinguish
"not distilled" from "distilled down to 0 bytes".
