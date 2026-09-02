# 193 — Research: can agy tool calls be routed through `harnez exec`/`harnez distill` without configuring agy itself?

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: [[052-headless-agent-cli-probes-for-idle-telemetry-refresh]] (prior agy
binary/CLI investigation — `agy --help`, `ANTIGRAVITY_AGENT=1` alias, local RPC
port model), `internal/usage/agy.go` (existing agy process/quota integration),
`internal/claude/apply.go` (how the Claude PreToolUse hook — `harnez exec hook`
wrapping every Bash call — is wired today), `cmd/harnez/exec.go` (`newExecHookCmd`)

## Problem

Today's `harnez exec`/`harnez distill` wrapping only works for Claude Code,
via an explicit `PreToolUse` hook entry in `~/.claude/settings.json` (matcher
`Bash` → `harnez exec hook`, see `cmd/harnez/exec.go:350-439`) that Claude
Code itself reads and honors. The user wants to know whether an equivalent
interception can be achieved for `agy` (Antigravity CLI) — and specifically
whether it can be done **without modifying agy's own config** (no opt-in
`agy` needs to know about), the same way the existing dotfiles alias
(`alias agy="ANTIGRAVITY_AGENT=1 agy"`, `$DOTFILES/shell/aliases.sh:88`)
already quietly hooks the `agy` invocation itself via an environment variable
rather than an agy-side setting.

## Research Questions

1. **Recap how hooks work in agy internally.** `strings` on the `agy` binary
   (`/home/uwe/.local/bin/agy`) shows real internal hook infrastructure:
   `jsonhook.JSONHookSpec`, `agent.PreToolHook`/`agent.PostToolHook`,
   `hooks_go_proto` (`PreInvocationHookArgs`, `PostInvocationHookArgs`,
   `SessionStartHookArgs`, `StopHookArgs`), `pretoolhooks.preToolHookConstructor`,
   `posttoolhooks.postToolHookConstructor`. Determine: is any of this
   user-configurable today (a settings.json key, a CLI flag, an env var), or
   is it entirely internal to agy's own built-in tool implementations (browser,
   file edit, etc.) with no user-facing extension point? Confirmed so far:
   `agy --help` exposes no `hooks` subcommand; `~/.gemini/antigravity-cli/settings.json`
   has no `hooks` key (only `permissions.allow` command-string patterns,
   `toolPermission`, `trustedWorkspaces`, etc.); `~/.gemini/config/mcp_config.json`
   exists but is empty on this machine — MCP server registration is the one
   real user-controllable extension surface agy documents (`agy mcp`
   subcommand), unlike a PreToolUse-style hook.
2. **Does the alias-hack pattern generalize?** The existing
   `ANTIGRAVITY_AGENT=1 agy` alias only affects how the user's interactive
   shell invokes the `agy` binary itself — it does not touch what `agy`
   shells out to internally for its own tool calls (e.g. the
   `permissions.allow` `command(...)` list — `git status`, `make build`,
   etc.). Determine whether agy invokes those permitted shell commands via a
   fresh non-interactive `sh -c`/`exec.Command` (which would NOT source
   `~/.bashrc`/aliases, per standard non-interactive-subshell semantics) or
   some other mechanism, and confirm this experimentally (not just from
   `--help`) — e.g. trace an actual `agy`-run `git status` and see whether an
   injected shell function/alias is visible to it.
3. **PATH-shimming as the "without telling agy" mechanism.** If agy resolves
   permitted commands via `$PATH` (not hardcoded absolute paths), a directory
   of shim scripts prepended to `PATH` before `agy` starts (e.g. a `git`
   wrapper that calls `harnez exec --tool agy -- git "$@"`, or a global
   `harnez distill`-piping wrapper for read-heavy commands) could intercept
   every one of agy's shelled-out tool calls without any agy-side
   configuration change — analogous to, but more general than, the existing
   env-var alias hack. Determine: does this actually work for agy's execution
   model (confirm via `strace`/experiment, not assumption)? What's the blast
   radius / safety concern (a broken or hanging shim could silently break
   every agy tool call, unlike Claude's opt-in hook which is easy to disable
   via `apply`/`settings.json`)? Is there a clean uninstall path?
4. **MCP as an alternative, non-"quiet" path.** Registering harnez as an MCP
   server agy calls explicitly does NOT satisfy the "without telling agy"
   constraint (it requires editing `mcp_config.json`), but document it as the
   supported/sanctioned alternative if research questions 1-3 conclude no
   quiet interception is safely possible — useful contrast, not the primary
   goal.

## Deliverables

1. A findings write-up (in this ticket, `## Findings` section, following the
   style of [[052-headless-agent-cli-probes-for-idle-telemetry-refresh]]) that
   directly answers research questions 1-4 with concrete evidence (binary
   strings, config file contents, and at least one live experiment — not
   `--help` output alone).
2. A go/no-go recommendation: if PATH-shimming (or another quiet mechanism)
   is confirmed safe and workable, scope a *separate* follow-up implementation
   ticket for it (do not implement in this ticket — this is research-only,
   matching 052's precedent). If no safe quiet mechanism exists, close this
   as research-complete with the MCP-registration path documented as the only
   supported alternative, same as 052 concluded for AGY headless quota
   probing.

## Acceptance Criteria

1. Findings section answers all four research questions with verified
   evidence, not speculation.
2. A clear recommendation (implement PATH-shimming in a new ticket, or close
   as not viable) is stated, with reasoning.
3. No code changes required to close this ticket — research/investigation
   only, same shape as issue 052.
