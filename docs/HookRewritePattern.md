# Hook Rewrite Pattern — Two-Stage Command Interception

How harnez features hook into an agent's tool-use lifecycle without owning process
execution themselves. Established by `harnez distill hook`; reused by the planned
`harnez exec hook` (see [issues/118](../issues/118-harnez-exec-shell-interceptor.md),
[issues/119](../issues/119-harnez-hook-agent-hook-management.md)).

## The constraint

A Claude Code `PreToolUse` hook is not a process wrapper. Claude Code invokes the hook's
`command:` binary and feeds it JSON on stdin describing the tool call about to run
(`{tool_name, tool_input: {command}}`). The hook does not get to spawn or supervise that
command — it can only respond with a JSON envelope telling Claude Code to substitute a
different command, which Claude Code's own Bash tool then executes:

```json
{"hookSpecificOutput": {"hookEventName": "PreToolUse", "updatedInput": {"command": "..."}}}
```

This means a hook has no access to the eventual exit code, duration, or output bytes —
those only exist after Claude Code runs the (possibly rewritten) command, by which point
the hook process has already exited.

## The pattern: rewrite now, capture later

Split the feature into two stages, each a separate CLI entry point with its own stdin
contract:

1. **`<feature> hook`** — the PreToolUse handshake only. Reads the JSON tool-call
   payload, decides whether to rewrite, and emits the `updatedInput` envelope. No side
   effects beyond the rewrite; it does not run anything itself.
2. **`<feature>` (no verb, or a wrapper subcommand)** — the command the rewrite points
   at. This is what Claude Code's Bash tool actually executes, so it's the one with
   access to the real subprocess: it can proxy stdio, measure wall-clock time, capture
   exit code, and record whatever telemetry the feature needs.

This is why `<feature> hook` and plain `<feature>` deliberately have incompatible stdin
contracts (JSON hook payload vs. raw content/passthrough) rather than one command
auto-detecting which mode it's in — format-sniffing to merge them would be implicit
magic this codebase avoids elsewhere (see explicit-flag conventions in `docs/lang/Go.md`
and `docs/CLIDesign.md`'s apply/init split).

## Reference implementation: `harnez distill`

`cmd/harnez/distill.go`:

- `newDistillHookCmd` / `runDistillHook` — the PreToolUse stage. No-ops unless
  `HARNEZ_DISTILL_AUTOPIPE=1`; otherwise rewrites known-noisy commands
  (`go test`, `git status`, …) to pipe through `harnez distill`.
- `runDistillWrapper` — the execution stage (`harnez distill -- <command>`), invoked
  as the rewritten command; runs the child, distills its combined output, preserves
  the exit code.

The hook entry itself is declarative config, not code: `apply` writes it into
`~/.claude/settings.json` from `config.yaml:90` (`command: "harnez distill hook"`).
Installing the hook is `apply`'s job (its existing managed-`"hooks"`-key merge in
`internal/claude/apply.go`); the hook only becomes active per-project/per-user once
the opt-in env var is set — `apply` can ship it globally as a no-op.

## Applying it to a new feature

1. Add the feature's rewrite logic under `<feature> hook`, matching `runDistillHook`'s
   shape (decode payload → decide → encode `hookSpecificOutput`, or no-op).
2. Point the rewrite at whatever subcommand does the real work — usually the feature's
   own wrapper form, not a third command.
3. Wire the hook's `command:` string into `config.yaml` and let `apply`'s existing
   hooks-merge install it — do not add a new top-level installer command (see
   [issues/119](../issues/119-harnez-hook-agent-hook-management.md)'s 2026-08-31
   decision for the footgun this avoids).
4. If the wrapper stage's own telemetry/side-effect write can fail or block, make it
   best-effort and non-blocking relative to the wrapped command's exit — the hook
   protocol gives no way to retry or recover after the fact.
