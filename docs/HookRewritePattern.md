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

**Rewrites from multiple hooks on the same matcher do not compose.** Per Claude Code's
hooks-guide ("Limitations"): when more than one `PreToolUse` hook matches the same tool
and each returns `updatedInput`, the hooks run in parallel and **the last one to finish
wins, non-deterministically** — there is no chaining, and no ordering guarantee from
array order in `settings.json`. Found the hard way in
[issues/119](../issues/119-harnez-hook-agent-hook-management.md): installing a second,
independent `PreToolUse`/`Bash` hook alongside distill's existing one meant either
rewrite could silently clobber the other depending on which process happened to exit
last. **If a new feature needs to rewrite the same matcher an existing hook already
covers, compose the rewrite logic into one hook command — do not add a second hook entry
for the same event/matcher.** (`harnez exec hook` does this: it calls
`internal/distill.RewriteBashCommand` itself, gated on the same `HARNEZ_DISTILL_AUTOPIPE`
env var `harnez distill hook` uses, rather than relying on Claude Code to run both hooks.)

**A rewritten command must stay one shell token if the original command isn't.** Claude
Code re-executes the rewritten string via its own outer `bash -c`. Splicing the original
command directly after `<feature> -- <command>` only works if `<command>` has no shell
metacharacters — a pipe, `&&`, `;`, or unbalanced quoting in the original command gets
re-interpreted by that *outer* shell instead of ever reaching the feature's own argv,
silently breaking capture (or, for `&&`/`;`, silently running part of the command outside
the wrapper entirely). Wrap the original command as one quoted argument to an inner shell
instead: `<feature> -- bash -c '<original, single-quote-escaped>'` (see
`cmd/harnez/exec.go`'s `shellQuote`, or distill's own approach of embedding the original
command inline inside one larger shell string, `internal/distill/hook.go`).

**PreToolUse rewrites run before permission evaluation; wrapper commands must be allow-listed.**
Claude Code evaluates its `permissions.allow` rules against the rewritten string returned in
`updatedInput`, not the original command. If a hook rewrites `git status` into `⚙ git status`
or `harnez exec -- git status`, rules like `Bash(git *)` will not match. The wrapper itself
(e.g. `Bash(⚙ *)`, `Bash(harnez *)`) must be explicitly present in `permissions.allow`.
Otherwise, every command requires interactive user confirmation — which cascades into complete
tool failure if interaction tools like `AskUserQuestion` are denied (such as under
`--debloat-preset aggressive`).

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
`~/.claude/settings.json` from `config.yaml`'s `hooks:` list. Installing the hook is
`apply`'s job (its existing managed-`"hooks"`-key merge in `internal/claude/apply.go`).

**Since issue 119, `apply` installs only one `PreToolUse`/`Bash` hook: `harnez exec
hook`** (see `cmd/harnez/exec.go`). It composes distill's rewrite internally (calling
`internal/distill.RewriteBashCommand` when `HARNEZ_DISTILL_AUTOPIPE` is set) before
wrapping the result for `harnez exec`'s telemetry capture — per the composability
constraint above, `harnez distill hook` is no longer separately installed by `apply`,
even though the command still exists and works standalone for direct/manual use.

**Compact Agent Alias `⚙` (Issue 270)**: `harnez apply` provisions a `⚙` symlink
in managed agent environments / PATH (`~/.claude/bin/⚙`, `~/go/bin/⚙`, etc.). Invoking
`⚙ <cmd>` (e.g. `⚙ echo hello`, `⚙ --tool test -- true`) dispatches directly to
`harnez exec` without boilerplate or token overhead in agent transcripts.

---

## Multi-Harness Hook Architectures

### 1. Claude Code (`~/.claude/settings.json`)
- **Protocol**: `PreToolUse` on matcher `Bash`.
- **Payload**: `{tool_name, tool_input: {command}}`.
- **Response**: `{"hookSpecificOutput": {"hookEventName": "PreToolUse", "updatedInput": {"command": "harnez exec --tool <tool> -- bash -c '<escaped>'"}}}`.
- **Config**: Managed via `harnez apply`.

### 2. Google Antigravity (`~/.harnez/shims/bash` & `hooks.json`)
- **Shell Command Interception (Guarded `bash` PATH shim)**:
  - See [issues/195](../issues/195-agy-path-shim-vs-native-hooks-options-and-tradeoffs.md) and [issues/271](../issues/271-decommission-agy-hooks-pretooluse-interception-in-favor-of-guarded-bash-path-shim.md).
  - **Clean UI & Zero Overwrite Artifacts**: Antigravity's PreToolUse `overwrite.CommandLine` hook mechanism leaks wrapper plumbing (e.g. `Bash(⚙ ...)` or `Bash(harnez exec ...)`) into user-facing chat traces. The quiet PATH shim intercepts `run_command` transparently while preserving native, clean commands in the UI (e.g. `Bash(git status)`).
  - **Recursion Guard**: Uses `HARNEZ_INTERCEPTED=1` to ensure nested subshells (e.g. `bash script.sh` inside an agent command) execute directly via `/bin/bash` without recursive wrapping.
  - **Shim Script** (`~/.harnez/shims/bash`, mode `0755`):
    ```sh
    #!/bin/sh
    if test "$HARNEZ_INTERCEPTED" = "1"
    then exec /bin/bash "$@"
    fi
    export HARNEZ_INTERCEPTED=1
    exec harnez exec -- /bin/bash "$@"
    ```
  - **Activation**: Provisioned by `harnez apply` at `~/.harnez/shims/bash`. Enabled in AGY environments via `PATH="$HOME/.harnez/shims:$PATH"`.

- **Client-Native Tool Observation (`hooks.json` PreToolUse Observer)**:
  - See [issues/373](../issues/373-bake-antigravity-internal-tool-observation-hook-into-harnez-apply-and-hooks-management.md) and canary at `scripts/canary-agy-tool-hook.sh`.
  - **The Distinction**: Issue 271 decommissioned `hooks.json` for *command rewriting* because `overwrite.CommandLine` polluted the UI. However, client-internal RPC tools (`schedule`, `generate_image`, `ask_question`, `view_file`, `replace_file_content`, etc.) never invoke bash and are completely invisible to PATH shims.
  - **Passive Observation**: A read-only `PreToolUse` hook with `matcher: "*"` returns `{"decision": "allow"}` without rewriting commands. This introduces **zero UI artifacts** while seamlessly capturing all host-internal tool calls into `tool_catalog.sqlite` for `harnez stats`.

---

## Telemetry & Agent Attribution

When the wrapped command executes under `harnez exec`, it resolves session and agent identity:
- **Claude Code**: `CLAUDE_CODE_SESSION_ID` → `agent_id: "claude"`.
- **Google Antigravity**: `ANTIGRAVITY_CONVERSATION_ID`, `ANTIGRAVITY_AGENT=1` → `agent_id: "agy"` (see [issues/266](../issues/266-support-antigravity-session-and-agent-id-resolution-in-telemetry.md)).
- **Codex**: `CODEX_SESSION_ID` → `agent_id: "codex"`.
- **Fallback**: PPID-keyed sliding window lockfile (`~/.harnez/sessions/`).

---

## Applying it to a new feature

1. Add the feature's rewrite logic under `<feature> hook`, matching `runDistillHook`'s
   shape (decode payload → decide → encode `hookSpecificOutput` / `overwrite`, or no-op).
2. Point the rewrite at whatever subcommand does the real work — usually the feature's
   own wrapper form, not a third command.
3. Wire the hook's `command:` string into `config.yaml` / `hooks.json` and let `apply`'s
   existing hooks-merge install it — do not add a new top-level installer command.
4. If the wrapper stage's own telemetry/side-effect write can fail or block, make it
   best-effort and non-blocking relative to the wrapped command's exit — the hook
   protocol gives no way to retry or recover after the fact.

