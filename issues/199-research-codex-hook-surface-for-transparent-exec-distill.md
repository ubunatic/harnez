# 199 — Research: does Codex CLI have a hook surface like agy's hooks.json?

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: [[193-research-agy-hook-surface-for-transparent-exec-distill]]
(prior research of the same shape for agy — reuse its methodology and
question structure rather than reinventing it), [[196-agy-native-hooks-plan-alongside-claude-hooks]]
(what got built once 193 found agy's `hooks.json`), `internal/claude/apply.go`
(existing Claude Code `PreToolUse` hook wiring this would extend to a third
agent if a Codex equivalent exists), `docs/CLIDesign.md` (apply/init split
to mirror if any config gets written)

## Problem

harnez currently wires `PreToolUse`-style interception into two agents:
Claude Code (`internal/claude/apply.go`, native since the start) and agy
(`internal/agy/hooks.go`, added in ticket 196 after 193's research found
agy's real `hooks.json` lifecycle-hook system). `harnez apply` also already
writes Codex *skills* (`~/.codex/skills/<name>/SKILL.md`, per
`docs/CLIDesign.md`), so Codex is a known, already-integrated target for
harnez — but nobody has checked whether Codex CLI has any equivalent
hook/extension surface for rewriting or intercepting shelled-out tool
calls the way agy's `hooks.json` or Claude's `settings.json` hooks do.

This ticket exists because 193's own investigation is now stale in one
specific way: the user hasn't used Codex in a while, so any settings/config
surface may have changed since it was last looked at (if it ever was —
this repo's own history shows Codex-specific tickets for status bar/usage
tracking, e.g. issues 139/141/144/149/156, but none investigating a
hook/PreToolUse-equivalent mechanism specifically).

## Task — research only, no implementation yet

Mirror 193's research shape and question structure:

1. **Is there a documented or discoverable hook/extension mechanism?**
   Check `codex --help` and subcommand `--help` output, any embedded
   help/doc strings in the Codex binary (193 found agy's `hooks.json` doc
   via `strings`-grepping the binary itself — try the same technique),
   and Codex's own config file(s) (locate them first — check the obvious
   candidates like `~/.codex/config.*` before assuming a layout).
2. **If a mechanism exists, what's its contract?** Config file location(s)
   (global vs. workspace-local), supported event names (PreToolUse-
   equivalent, PostToolUse-equivalent, etc.), matcher/filter syntax,
   handler types (command/HTTP/other), and — critically — the stdin/stdout
   JSON schema for a command-type handler (agy's turned out to support a
   `overwrite.CommandLine` rewrite field; check whether Codex's does or
   doesn't have an equivalent capability).
3. **Does Codex resolve shelled tool calls via `$PATH`?** Re-run 193's Q2/Q3
   live PATH-shim experiment (shim script logging + `exec`-ing the real
   binary, prepended to `$PATH`, run against a real non-`--help` Codex
   invocation) to check whether a PATH-shim approach (195's mechanism) would
   also work for Codex, independent of whether a native hook exists.
4. **Any MCP-equivalent extension point?** Check whether Codex has an
   MCP-server registration mechanism like `agy mcp add` (193's Q4) as a
   fallback "tell Codex" integration path if no hooks-style mechanism turns up.
5. **Blast-radius/safety notes**: same considerations 193 raised for
   agy — cwd behavior for shelled commands, `$PATH` precedence, timeout
   behavior for hook handlers if any exist, disable path if a config gets
   written.

## Non-goals

- No implementation of any Codex hook wiring, PATH-shim, or MCP
  registration in this ticket — purely investigative, matching 193's own
  scope discipline (it stayed research-only and spawned 195/196 as
  follow-ups).
- Not re-litigating the existing Codex *skills* integration
  (`~/.codex/skills/`) — that's already built and out of scope here.

## Acceptance Criteria

1. Findings section written into this ticket answering the five questions
   above (or explicitly stating "no such mechanism found" with the
   evidence for that conclusion, which is itself a valid research outcome).
2. If a real hook mechanism is found, a Recommendation section states
   whether a follow-up ticket (mirroring 195/196's split) should be filed,
   and sketches which of the two shapes (quiet PATH-shim vs. native/
   documented config) fits best — without committing to full scope itself.
3. If no mechanism is found, the ticket documents that clearly enough that
   a future session doesn't waste time re-deriving the same negative
   result.
4. No code changes in this ticket.

## Findings (2026-09-02)

Investigated on the local machine. Binary: `codex-cli 0.152.0`,
`/home/uwe/.local/bin/codex` -> symlink ->
`/home/uwe/.codex/packages/standalone/releases/0.152.0-x86_64-unknown-linux-musl/bin/codex`
(255MB, standalone Rust build, `strings`-greppable). `$CODEX_HOME` =
`~/.codex` (has `config.toml`, `plugins/`, `skills/`, `rules/`, etc.).

### Q1 — Is there a documented/discoverable hook mechanism?

**Yes — and unlike agy, this is a first-class, currently-stable, always-on
feature, not something dug out of a TUI-only doc string.** Evidence:

- `codex features list` shows `hooks    stable    true` (on by default,
  not behind a flag) and a retired `plugin_hooks  removed  false`
  (superseded by the current mechanism).
- `codex --help` exposes `--dangerously-bypass-hook-trust`: *"Run enabled
  hooks without requiring persisted hook trust for this invocation.
  DANGEROUS. Intended only for automation that already vets hook
  sources"* — proof hooks are wired into the main execution path, not a
  side experiment.
- `strings` on the binary turns up a live TUI slash command surface:
  `"view and manage lifecycle hooks"`, with a full hook-trust review UI —
  `"New hook - review required"`, `"Trusted"`, `"Modified since last
  trusted - review required"`, `"Managed hooks are always on"`, and key
  hints (`trust all` / `review hooks` / `trust` / `toggle`). Config
  sources listed in that UI: `Admin config`, `User config`, `Project
  config`, `Session flags`, `Cloud-managed config`.
- Real, installed plugin examples on this machine ship their own
  `hooks.json` (`~/.codex/.tmp/plugins/plugins/figma/hooks.json`,
  `.../replayio/hooks.json`), referenced from the plugin manifest's
  `"hooks": "./hooks.json"` field (confirmed via an embedded
  `plugin.json` schema example in the binary's strings, which also lists
  `"skills"`, `"mcpServers"`, `"apps"` as sibling manifest fields).
- No dedicated `hooks.json` exists at the user level on this machine yet
  (`find ~/.codex -maxdepth 1 -iname hooks.json` → none); user-level hooks
  live inside `~/.codex/config.toml` under a `[hooks.<name>]` table
  instead (confirmed live in Q2 below) — this is the one structural
  difference from agy, which uses a standalone `hooks.json` file.

### Q2 — Exact contract

- **Config file location**: global `~/.codex/config.toml`, `[hooks.<name>]`
  tables (verified live: wrote a scratch `CODEX_HOME` with
  ```toml
  [hooks.test-hook]
  enabled = true
  [[hooks.test-hook.PreToolUse]]
  matcher = "Bash"
  [[hooks.test-hook.PreToolUse.hooks]]
  type = "command"
  command = "echo hi"
  ```
  and ran `CODEX_HOME=<scratch> codex --strict-config doctor` — it parsed
  cleanly, i.e. `hooks.*` is a recognized top-level config.toml schema key,
  not something `--strict-config` would reject). Project-local hooks also
  exist: binary strings reference a `.codex/hooks` path candidate sibling
  to `.codex/agents` and `.codex/skills`, and plugins carry their own
  `hooks.json` (see Q1) with a `matcher`/`hooks`/`type`/`command` shape
  that is structurally identical to Claude Code's own hook JSON (not a
  coincidence — the binary also embeds a `codex_external_agent_migration`
  module with strings like `claude-code-sessions`, `.claude`,
  `settings.json`, `settings.local.json`, and a TUI action *"import setup,
  this project, and recent chats from Claude Code"* — Codex has a built-in
  Claude Code migration path that is schema-compatible with Claude's own
  hooks).
- **Event names** (from binary strings, both kebab-case wire names and
  PascalCase config names): `pre-tool-use`/`PreToolUse`,
  `permission-request`/`PermissionRequest`, `post-tool-use`/`PostToolUse`,
  `pre-compact`/`PreCompact`, `post-compact`/`PostCompact`,
  `session-start`/`SessionStart`, `session-end`/`SessionEnd`,
  `user-prompt-submit`/`UserPromptSubmit`, `subagent-start`/`SubagentStart`,
  `subagent-stop`/`SubagentStop`, `interrupt`/`Interrupt` — a near-exact
  superset of Claude Code's and agy's hook event names.
- **Matcher syntax**: a `matcher` field on each hook-group entry, same as
  Claude Code/agy (`"Bash"`, `"Write|Edit"` seen in the real plugin
  examples on disk).
- **Handler types**: strings show `EventMatcherCommandAsyncMCP
  ServerHandlerPromptManaged` and `ConfiguredHookHandler::Command`/
  `ConfiguredHookHandler::McpTool` variants — so besides `type: "command"`
  (shell exec, with `command`, `commandWindows`, `timeoutSec`, `async`,
  `statusMessage` fields per `HookHandlerConfig::Command with 6 elements`),
  Codex also supports an MCP-tool handler type
  (`HookHandlerConfig::McpTool with 5 elements`) and a `Managed`/`Prompt`
  handler class referenced in the TUI trust UI — richer than agy's
  "command-only" hook system.
- **stdin/stdout JSON schema for `PreToolUse`** (assembled from many
  `*Wire` struct names and error strings in the binary, e.g.
  `PreToolUseHookSpecificOutputWire with 5 elements`,
  `PreToolUseDecisionWire`, and a long list of hook-output-validation error
  strings): stdin carries `hookEventName`, `tool_name`, `tool_input`,
  `tool_use_id`, `session_id`, `turn_id`, `cwd`, `transcript_path`,
  `permission_mode`, `model` — again a near-exact mirror of Claude Code's
  own `PreToolUse` hook input schema. Stdout supports a `permissionDecision`
  field (`"allow"`/`"deny"`, with `"ask"` referenced but flagged as
  currently unsupported by several error strings —
  `"PreToolUse hook returned unsupported permissionDecision:ask"`), a
  `permissionDecisionReason`, and **critically: an `updatedInput` field**
  — error strings confirm the contract explicitly (`"PreToolUse hook
  returned updatedInput without permissionDecision:allow"`). This is
  Codex's equivalent of agy's `overwrite.CommandLine` — a `PreToolUse`
  hook can rewrite the tool call's input (which for a shell/`Bash` tool
  call would include the command line) before it executes, gated on
  returning `permissionDecision: "allow"`. `PostToolUse` supports
  `updatedMCPToolOutput` (not currently accepted per an error string) and
  `additionalContext`/`systemMessage`.

### Q3 — Does Codex resolve shelled tool calls via `$PATH`? Live experiment.

**Confirmed experimentally: yes.** Ran a real (non-`--help`,
non-interactive) `codex exec` invocation against this repo with a `git`
shim prepended to `$PATH`, mirroring 193's agy experiment exactly:

```sh
# shim at <scratch>/codex-shim/git:
#!/bin/sh
echo "SHIM-INVOKED args=$* pwd=$(pwd) PATH=$PATH" >> <scratch>/codex-shim.log
exec /usr/bin/git "$@"

cd /home/uwe/projects/harnez
export PATH="<scratch>/codex-shim:$PATH"
timeout 90 codex exec --sandbox read-only --skip-git-repo-check --json \
  "Run the shell command: git status. Just run it, then summarize the output in one short sentence." \
  > out.jsonl 2> err.log
```

Result: exit 0, real API round trip (`turn.completed`, ~33k input tokens
mostly cached, 89 output tokens). The model ran the final command as
`/usr/bin/zsh -lc 'git status'` from `cwd=/home/uwe/projects/harnez` (the
actual project directory Codex was launched from — unlike agy, which ran
shelled commands from its own internal scratch dir). The shim intercepted
it: the `aggregated_output` in the JSONL event includes the shim's own
stderr line (`.../codex-shim.log: Read-only file system`) *interleaved
with* real `git status` output (`On branch main / Your branch is ahead of
'origin/main' by 92 commits / nothing to commit, working tree clean`),
which matches the actual `git status` run directly afterward for
verification. The shim's `exec /usr/bin/git "$@"` fallthrough is what
produced the real output — direct proof of `$PATH`-based resolution.

An unplanned but important discovery from the same run: **Codex's own
command sandbox (`--sandbox read-only`, the default policy) blocked the
shim's `echo ... >> logfile` write** — the shim's log line never landed on
disk (`find` afterward shows only earlier, unrelated internal `git`
invocations Codex itself made for repo-trust checks, e.g. `git rev-parse
--git-dir` from the actual cwd, and one oddly from `cwd=~/.oh-my-zsh`
with a shorter `$PATH` — likely a login-shell profile probe). This means a
PATH-shim's own side effects (logging, wrapping) are subject to whatever
filesystem sandbox Codex is running under — a shim that assumes
unrestricted write access (like agy's, which had no such sandbox in its
experiment) will silently fail its logging/wrapping step under
`read-only` or a restrictive `workspace-write` policy unless its target
paths are inside the sandbox's writable roots. `exec`-ing through to the
real binary still worked regardless, since that inherits the sandboxed
process's own already-negotiated permissions.

### Q4 — MCP-equivalent extension point

Confirmed present: `codex mcp add|remove|list|get|login|logout` (`codex mcp
--help`), directly analogous to `agy mcp add`. `codex mcp add <name>
(--url <url> | -- <command>...)` registers a server into `~/.codex/config.toml`
(`codex mcp list` currently reports *"No MCP servers configured yet"* on
this machine). Like agy's MCP path, this is an explicit, "tell Codex"
opt-in — not a quiet mechanism — and, per Q1/Q2, is now the *secondary*
extension point behind Codex's native, richer `hooks` system (which,
unlike agy's, is already stable/on by default and does not itself require
telling Codex anything beyond writing the config).

### Q5 — Blast-radius / safety notes

- **cwd for shelled commands**: the actual project directory Codex was
  invoked from/`--cd` target (confirmed in Q3) — better-behaved than agy,
  which ran from its own scratch dir.
- **`$PATH` precedence**: standard inherited-process `$PATH`, shim
  prepended in this experiment took effect immediately and survived a
  `zsh -lc` (login shell) re-exec of the final command, though login-shell
  profile sourcing can reorder/extend `$PATH` around it (observed: shim
  entry stayed present but a later oh-my-zsh-influenced `$PATH` snapshot
  had different ordering) — same caveat 193 raised for agy, not unique to
  Codex.
- **Filesystem/command sandbox**: Codex wraps model-executed shell
  commands in its own sandbox (`read-only` / `workspace-write` /
  `danger-full-access`, `--sandbox` flag; default per `codex doctor` here
  was `restricted fs + restricted network`, approval `OnRequest`). This is
  a real, additional layer beyond `$PATH` resolution that agy did not
  exhibit in 193's experiment — any PATH-shim approach for Codex needs to
  either keep its own side effects (logging, wrapper state) inside the
  sandbox's writable roots or accept they'll silently no-op under
  `read-only`.
- **Hook timeout**: binary strings confirm a per-hook `timeoutSec` field
  and a `"hook timed out"` runtime error path exists, but the exact
  default numeric value was not recoverable from strings alone (unlike
  agy's documented 30s default) — would need either a live hook-trust
  config test or reading upstream OpenAI Codex docs to pin down; flagged
  here rather than guessed.
- **Disable path**: multiple concrete off-switches exist, more than agy
  had: (1) per-hook `enabled = false` in the `[hooks.<name>]` config.toml
  table; (2) the TUI `/hooks` panel's interactive toggle
  ("Turn hooks on or off. Your changes are saved automatically."); (3) a
  `disableAllHooks` key referenced in binary strings alongside
  `settings.json`/`settings.local.json` (a global kill switch, inherited
  from/compatible with the Claude Code settings shape Codex can import);
  (4) hook **trust** is itself a gate independent of `enabled` — new or
  modified hooks require explicit trust review before they run at all
  (`"New hook - review required"`), and `--dangerously-bypass-hook-trust`
  exists specifically to skip that gate for automation, implying trust
  review is the default, safer posture.

## Recommendation

**A follow-up implementation ticket is warranted, and it should mirror
195/196's split, in the *native-hook* direction rather than the
quiet-PATH-shim direction (opposite of what 193 recommended for agy).**
Reasoning:

- Unlike agy in 193, Codex's hook mechanism is not a leftover/hidden
  feature — it is `stable` by default, already exercised by real installed
  plugins on this machine, and has a documented, structured
  `PreToolUse`/`updatedInput` rewrite contract that is a closer match to
  Claude Code's own hook contract than agy's was. This makes native-hook
  wiring (a `harnez apply`/`codex`-equivalent command writing
  `[hooks.harnez]` into `~/.codex/config.toml`, or an installed harnez
  plugin shipping its own `hooks.json`) both more natural to build (schema
  is close to what `internal/claude/apply.go` already emits) and more
  robust than a PATH-shim, which Q3's finding shows is subject to Codex's
  own command sandbox silently swallowing shim side effects.
- PATH-shimming is still confirmed *technically* viable (Q3) as a
  fallback/quick-hack path, but offers no advantage over the native route
  here the way it did for agy (agy had no discoverable structured
  rewrite hook without an agy-side config edit; Codex's rewrite hook
  requires a config edit too, so the "quiet, no config change" argument
  that motivated 195 for agy does not apply to Codex).
- Suggested shape for the follow-up ticket: a `harnez codex-hooks
  apply/status/hook` command family (mirroring `cmd/harnez/agyhooks.go`'s
  shape) that writes/verifies a `[hooks.harnez]` `PreToolUse` entry (with
  a `Bash`-equivalent matcher) into `~/.codex/config.toml`, returning
  `{"permissionDecision":"allow","updatedInput":{...}}` to route the
  command through `harnez exec`/`harnez distill` — with the hook-trust
  gate and per-invocation `--dangerously-bypass-hook-trust` interaction
  explicitly scoped, since it's a Codex-specific concern `apply.go` and
  `agyhooks.go` don't have. Pin down the real hook `timeoutSec` default
  (Q5 gap) as part of that ticket's own research/design step before
  committing to a synchronous rewrite hook that could stall command
  execution.
- Do not implement in this ticket — research-only, matching 193/052's
  precedent.
