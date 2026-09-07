# 268 — harnez exec: add default 60s timeout with kill and stderr signal, overridable via --timeout flag and repo settings

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Issue 118; Issue 111; `cmd/harnez/exec.go`; `internal/resolve/resolve.go`

---

## 1. Problem & Motivation

User request (near-verbatim): "harnez exec should support a timeout, e.g.,
when a process runs >1m it should kill the call and tell the calling agent
on stderr about it. To override this behavior, a `--timeout` should be
given or set in a repo's harnez settings."

This is a proactive DX/robustness request, not a bug report — no specific
hang or incident triggered it. It surfaced in a session running
harnez-orchestrated subagent dev work on the mic-level-metering feature
(issues 262/264/265), where an agent depends on `harnez exec` (the
PreToolUse-hook-rewritten path most Bash tool calls go through — see issue
118) to eventually return control, one way or another.

**Current behavior, confirmed by reading the real implementation**
(`cmd/harnez/exec.go`, `runExecWrapper`, lines ~202–272): the wrapped
command is spawned with plain `exec.Command(args[0], args[1:]...)` (no
`context.Context`, no deadline) and the wrapper simply blocks on
`c.Run()` (line 233) until the child exits on its own. There is:

- **No timeout of any kind.** A wedged or runaway subprocess launched
  through `harnez exec` — and therefore through the PreToolUse Bash
  rewrite most agent shell calls are routed through — hangs the wrapper,
  and the calling agent, indefinitely.
- **No stderr signal** distinguishing "the process is still running and we
  gave up waiting" from "the process exited" (with any exit code) or "the
  process produced no output" (a separate, already-observed failure mode
  — see issue 111's "stricter timeout" framing for a per-agent-collector
  analogue, and issue 255's "no output" diagnostics discussion). An agent
  watching a `harnez exec`-wrapped call today cannot tell "wedged, still
  running" apart from "hung with no output, but still alive" without
  external intervention (Ctrl-C, a separate `ps`/kill).

Note: the *telemetry* write inside `runExecWrapper` already has its own
independent, unrelated bound (`opts.InsertTimeout`, default
`defaultExecInsertTimeout = 200ms`, see lines 46–51 and
`recordExecTelemetry`) — that only bounds the best-effort DB write after
the child has already exited, not the child's own runtime. This ticket is
about bounding the child process's runtime itself.

**No per-repo settings mechanism exists today.** The user's ask names "a
repo's harnez settings" as one of the two override paths; that was
verified against the actual codebase rather than assumed:

- `config.yaml` at the harnez repo root (loaded via
  `internal/claude/config.go`'s `LoadConfig`/`LoadConfigEmbedded`) is
  **harnez's own project config**, embedded into the `harnez` binary and
  consumed by `harnez apply`/`init` to manage `~/.claude` — it is not a
  per-target-repo settings file that other projects using harnez would
  each carry their own copy of.
- `~/.config/harnez/local.yaml` (`internal/usage/localconfig.go`,
  `LocalConfig`/`LoadLocalConfig`) is an existing **user-local**,
  machine-specific override file (issue 109) — not per-repo, and not
  committed/shared with a project.
- No `.harnez.yaml`/`.harnez.toml` or similar per-project-repo dotfile
  resolution exists anywhere in `internal/` or `cmd/harnez/` today (grepped
  for `RepoConfig`, `per-repo`, `.harnez.` — no hits besides the
  `~/.harnez/sessions` state dir in `internal/resolve/resolve.go`).

So "a repo's harnez settings" is a **new mechanism** this ticket needs to
either introduce (a small per-repo settings file, e.g. `.harnez.yaml` at
the target repo's root, distinct from harnez's own embedded
`config.yaml`) or explicitly scope down during design if a lighter-weight
existing seam (e.g. an env var a project's own `.envrc`/CI config sets) is
judged sufficient instead. This ambiguity should be resolved in Phase 1
design, not assumed away.

## 2. Scope

**In scope:**
- Bound `runExecWrapper`'s child-process runtime with a real deadline
  (`context.Context` + `exec.CommandContext`, or an equivalent
  timer+`Process.Kill()` path) — default **60s** when nothing overrides
  it.
- On timeout: kill the child process (process group, to catch children the
  wrapped command itself spawns — mirror whatever signal/kill convention
  the codebase already uses elsewhere for subprocess teardown, e.g.
  `internal/usage/miclive.go`'s zero-zombie handling, if applicable) and
  write a clear, unambiguous message to stderr identifying this as a
  **timeout kill** (not a normal exit, not a spawn failure) before
  returning — the message must let a calling agent programmatically or
  visually distinguish "killed on timeout" from "process exited with a
  non-zero code" or "process hung with no output but still alive."
- Preserve the existing telemetry contract: a timeout-killed call should
  still record a `tool_calls` row (best-effort, same bounded-write
  discipline already in `recordExecTelemetry`) with a way to distinguish
  it from a normal exit — decide during design whether this reuses
  `call_type`/`note` (compare to issue 226's
  `ExpectedFailureCallType`/`ExpectFailure` convention already in this
  file) or adds a new field.
- `--timeout` CLI flag on `harnez exec` (duration-parseable, e.g. `30s`,
  `2m`) that overrides the default for that one invocation.
- A repo-level settings override, resolved per the design question raised
  in §1 above — name and locate the actual file/mechanism as part of the
  implementation, don't guess the path in this ticket.
- Precedence order once both exist: `--timeout` flag > repo setting >
  built-in 60s default.
- Tests: default-timeout kill-and-stderr-message behavior, `--timeout`
  override, repo-setting override, and flag-over-setting precedence.

**Out of scope:**
- Changing the unrelated `defaultExecInsertTimeout` (200ms telemetry-write
  bound) — already correct and unrelated to this ticket.
- Building a general-purpose per-repo settings system beyond what this one
  timeout setting needs, unless the design phase finds the minimal version
  not worth a bespoke one-off (judgment call for whoever picks this up).
- Retroactively adding timeouts to other long-running harnez subsystems
  (issue 111's per-agent collector pipelines, issue 255's collector
  retries) — related in spirit, tracked separately.

## 3. Acceptance Criteria

- [ ] `harnez exec --tool <t> -- <long-running command>` with no
      `--timeout` given and no repo setting configured kills the child
      after 60s and exits non-zero, with a stderr message clearly stating
      the call was killed due to timeout.
- [ ] `harnez exec --tool <t> --timeout 2s -- sleep 10` is killed at ~2s,
      not 60s.
- [ ] The repo-level override (once its mechanism is named/built) changes
      the effective default without requiring `--timeout` on every call.
- [ ] `--timeout` on the command line wins over the repo setting when both
      are present.
- [ ] A command that finishes well within the timeout is unaffected —
      no behavior change, no added latency, existing telemetry/distill
      paths untouched.
- [ ] The telemetry row for a timeout-killed call is distinguishable from
      a normal exit's row (exact mechanism decided during implementation).
- [ ] `go test ./...` passes; new tests assert on the specific stderr
      timeout message text and on exit-code/kill behavior, not just "the
      command returns."

## 4. Verification Guidance

- Unit test `runExecWrapper` (or its context-aware successor) with a
  short injected timeout against a command that ignores/outlives it (e.g.
  `sleep 5` with `--timeout 200ms` in test), asserting: (a) the wrapper
  returns well before the child's natural exit time, (b) the stderr
  writer received the timeout message, (c) the child process is actually
  gone afterward (no zombie/orphan — check via `/proc/<pid>` or process
  group signal fan-out in the test, consistent with this project's
  "Zero Zombie Guarantee" — see `docs/practices/AgenticLoop.md` Invariant
  3).
- Live check: run `harnez exec --tool test -- sleep 90` in a real shell
  and confirm it is killed at ~60s with the expected stderr message,
  since this changes real subprocess/PreToolUse-hook-path behavior that
  unit tests with injected short timeouts could still miss (per
  `docs/practices/AgenticLoop.md`'s live-verification guidance for
  hook/env-resolution-adjacent features).
