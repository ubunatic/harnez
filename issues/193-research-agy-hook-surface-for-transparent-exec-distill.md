# 193 — Research: can agy tool calls be routed through `harnez exec`/`harnez distill` without configuring agy itself?

**Status**: Closed — research complete
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: [[052-headless-agent-cli-probes-for-idle-telemetry-refresh]] (prior agy
binary/CLI investigation — `agy --help`, `ANTIGRAVITY_AGENT=1` alias, local RPC
port model), [[195-path-shim-wrapper-for-agy-exec-distill-interception]] (follow-up
implementation ticket scoped from this research), `internal/usage/agy.go` (existing
agy process/quota integration), `internal/claude/apply.go` (how the Claude
PreToolUse hook — `harnez exec hook` wrapping every Bash call — is wired today),
`cmd/harnez/exec.go` (`newExecHookCmd`)

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

## Findings (2026-09-02)

Investigated on the local machine, binary at `/home/uwe/.local/bin/agy` (Go
binary, 209MB, unstripped-enough to `strings`-grep embedded docs). Build on
052's already-established facts (binary location, `ANTIGRAVITY_AGENT=1 agy`
alias, `agy --help` subcommand list) rather than re-deriving them.

### Q1 — Is agy's internal hook infra user-configurable?

**Yes — this overturns the "no user-facing extension point" hypothesis in the
ticket's problem statement.** `agy --help` and every subcommand's `--help`
(`agy mcp --help`, `agy install --help`, `agy plugin --help`, `agy plugin
import --help`, `agy plugin validate --help`) confirm there is no `hooks`
*subcommand* — but `strings -n 5 /home/uwe/.local/bin/agy` turns up a complete,
verbatim embedded Markdown doc (evidently shipped as in-product help content,
likely surfaced by a `/hooks` slash command inside an interactive session —
one of the strings literally reads `"a bug where the `/hooks` command wrote
configurations to `~/.gemini/antigravity-cli/hooks.json` instead of the shared
`~/.gemini/config/hooks.json`"`) titled **"Lifecycle Hooks (`hooks.json`)"**.
This is agy's real, fully-specified hook system:

- Config file locations: global/shared `~/.gemini/config/hooks.json`,
  or workspace-local `<workspace>/.agents/hooks.json` (neither exists on this
  machine today — confirmed via `find ~ -iname hooks.json`, zero hits).
- Named hook objects support `enabled`, `PreToolUse`, `PostToolUse`,
  `PreInvocation`, `PostInvocation`, `Stop` — a near-exact structural mirror
  of Claude Code's own hook event names.
- `PreToolUse`/`PostToolUse` handlers are grouped under a `matcher` regex
  (e.g. `"matcher": "run_command"`, `"browser_.*"`, or `"*"`/`""` for all
  tools) matching a lowercased tool-step-type name.
- Handler `type` is currently always `"command"` (shell exec via `sh -c` /
  `cmd /c`, `timeout` default 30s, cwd = the directory containing
  `hooks.json`). Doc explicitly states: *"Only `type: "command"` is currently
  supported (no HTTP or prompt hooks yet)"* and *"Hooks run synchronously and
  block the agent loop (no async execution)."*
- **`PreToolUse` contract is the interesting one for exec/distill purposes**:
  stdin gets `{"toolCall": {"name": "run_command", "args": {"CommandLine":
  "npm test"}}, "stepIdx": N, ...}`; stdout must return `{"decision":
  "allow"|"deny"|"ask"|"force_ask", "reason": "...", "permissionOverrides":
  [...], "overwrite": {"CommandLine": "ls -la"}}`. The `overwrite` field lets
  a hook **rewrite the command line before it executes** — i.e. agy's native
  hook mechanism could, in principle, route every shelled command through
  `harnez exec`/`distill` by rewriting `CommandLine` to a wrapped form. This
  is strictly more capable than PATH-shimming (structured JSON in/out,
  official support, documented contract) but requires writing agy's own
  `hooks.json` — which fails this ticket's explicit "without telling agy"
  constraint, same as MCP registration. Documented here for completeness and
  as the best non-quiet alternative (better than MCP — see Q4).

### Q2/Q3 — Does agy resolve `permissions.allow` shell commands via `$PATH`? Live PATH-shim experiment.

**Confirmed experimentally: yes, `$PATH` lookup, not hardcoded/absolute
paths.** Built a real shim and ran it against a live (non-`--help`) `agy -p`
invocation:

```sh
# shim at <scratch>/agy-shim/git:
#!/bin/sh
echo "SHIM-INVOKED args=$* pwd=$(pwd) PATH=$PATH" >> <scratch>/agy-shim.log
exec /usr/bin/git "$@"

cd /home/uwe/projects/harnez
export PATH="<scratch>/agy-shim:$PATH"
timeout 60 env ANTIGRAVITY_AGENT=1 agy -p \
  "Run the shell command: git status. Just run it, then summarize the output in one short sentence."
```

Result: agy ran `git status` (allowlisted via `command(git status)` in
`~/.gemini/antigravity-cli/settings.json`, `toolPermission: "always-proceed"`
— no interactive approval prompt), the shim log recorded the invocation
(`SHIM-INVOKED args=status ...`), and the model's final text response
reflected the *real* git's output, confirming the shim's `exec /usr/bin/git`
fallthrough worked. Total round trip: non-interactive, ~1 API call, no hang,
exit 0.

Two additional facts surfaced by the same experiment, neither previously
documented:

1. **agy prepends its own bin dir to `$PATH` at the front**, ahead of the
   inherited process `PATH`: the shim log's captured `PATH=` value showed
   `/home/uwe/.gemini/antigravity-cli/bin:<our shim dir>:<rest of inherited
   PATH>`. `~/.gemini/antigravity-cli/bin/` currently holds only `agentapi`
   and `webm_encoder` — no `git`, so it doesn't shadow our shim, but it means
   agy's own bin dir wins any future naming collision, and a shim strategy
   must not assume it occupies the front of `PATH`.
2. **agy executes shelled tool commands with `cwd` set to
   `~/.gemini/antigravity-cli/scratch`**, not the directory `agy` itself was
   launched from — `git status` failed with "not part of a Git repository"
   even though `agy` was launched from `/home/uwe/projects/harnez` (a
   `trustedWorkspaces` entry). This is orthogonal to the shim question but
   relevant to any follow-up implementation: a shim that assumes it inherits
   the user's working directory will be wrong; the real command's cwd is
   agy's own scratch dir unless the session is bound to a project/workspace
   some other way (not investigated further here — out of scope for this
   ticket).

**Safety / blast-radius assessment (Q3):** PATH-shimming is viable and, if
scoped correctly, no riskier than the existing `ANTIGRAVITY_AGENT=1 agy` alias
pattern:

- **Bounded blast radius**: if the shim directory is prepended to `PATH` only
  via a wrapper alias (`alias agy="PATH=<shimdir>:$PATH agy"`), the
  interception — and any failure mode — is scoped to processes launched
  through that alias, not the user's entire shell/`PATH`. This matches 193's
  framing concern ("no easy disable path unlike Claude's `apply`-managed
  hook") — in practice the disable path is exactly as easy as the existing
  alias: comment it out / remove it. It is *not* centrally managed by
  `harnez apply` (no drift-detection, no `harnez status` linter coverage)
  unless a future ticket adds that.
- **Failure mode**: a shim that hangs or exits non-zero unexpectedly would
  break every agy invocation of that specific command for any session running
  the alias — e.g. every `git status`/`git diff` call agy makes would fail or
  stall (up to whatever timeout wraps the outer `agy` call; agy's own
  `PreToolUse`-equivalent hook timeout of 30s does *not* apply here since this
  is a plain `$PATH` shim, not agy's hook system — a hanging shim blocks until
  the process is killed or agy's own tool-call timeout, if any, fires).
  Mitigation: keep shim scripts trivial (log + `exec` immediately, no
  buffering/blocking I/O) and test them standalone before wiring into the
  alias.
- **No collision risk observed today** with agy's own prepended bin dir
  (empty of relevant commands), but any future agy update that adds
  commands there should be re-checked.

### Q4 — MCP as the alternative

Confirmed unchanged from the ticket's premise: `agy mcp add/remove/list/
enable/disable` is the one CLI-documented extension subcommand, and
registering harnez there requires editing `~/.gemini/config/mcp_config.json`
(currently empty on this machine) — an explicit agy-side opt-in, not quiet.
Given the Q1 finding above, **`hooks.json`'s `PreToolUse` contract is a
better-fitting non-quiet alternative than MCP** for this specific goal (exec
wrapping/distillation), since it offers a structured `overwrite` field
purpose-built for rewriting a command line before execution, whereas MCP
would require exposing harnez's exec/distill functionality as a tool agy
explicitly calls (a different shape of integration, more like giving agy a
new capability than wrapping an existing one). Both remain "tell agy" paths,
unlike PATH-shimming.

### Recommendation

**Go.** PATH-shimming is confirmed technically viable and reasonably safe
when scoped through an opt-in alias (matching the existing
`ANTIGRAVITY_AGENT=1 agy` alias pattern's risk profile). Filed
[[195-path-shim-wrapper-for-agy-exec-distill-interception]] to scope the
actual implementation (shim generation, `git` as the first wrapped command,
disable-path documentation, self-check) — not implemented in this ticket per
its research-only scope, matching issue 052's precedent. Closing 193 as
research-complete.
