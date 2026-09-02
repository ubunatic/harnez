# 200 — Implement native Codex hooks wiring (harnez codex-hooks)

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[199-research-codex-hook-surface-for-transparent-exec-distill]]
(research ticket this implements — read its Findings/Recommendation
sections first), [[196-agy-native-hooks-plan-alongside-claude-hooks]] and
`internal/agy/hooks.go` + `cmd/harnez/agyhooks.go` (the analogous agy
implementation this should mirror in shape — merge/preserve-unrelated-keys
semantics, `apply`/`status`/`hook` subcommand split),
`internal/claude/apply.go` (original PreToolUse hook wiring pattern both
196 and this ticket descend from), `docs/CLIDesign.md` (apply/init
separation — check before deciding where this installs from)

## Problem

Ticket 199 found that Codex CLI (unlike agy) ships a stable, always-on
native hooks system: `~/.codex/config.toml`'s `[hooks.<name>]` tables
support a `PreToolUse` handler whose stdout can include an `updatedInput`
field (gated on `permissionDecision: "allow"`) to rewrite a tool call's
input before execution — Codex's equivalent of agy's
`overwrite.CommandLine`, and structurally closer to Claude Code's own
`PreToolUse` hook contract than agy's was. 199 recommended building this
as a native-hook integration rather than a PATH-shim, since Codex's own
command sandbox (`read-only`/`workspace-write`) can silently swallow a
PATH-shim's side effects, and a config write is required either way (so
PATH-shimming's "no config change" advantage, which motivated 195 for
agy, doesn't apply here).

harnez now has three agent surfaces it talks to (Claude Code, agy, Codex)
but only two have `PreToolUse` routing into `harnez exec`/`distill`. This
ticket closes that gap for Codex.

## Scope

Implement `harnez codex-hooks apply|status|hook`, mirroring
`cmd/harnez/agyhooks.go`'s shape and `internal/agy/hooks.go`'s
merge/status/remove semantics:

1. **`internal/codex` package** (new, parallel to `internal/agy`):
   - `HooksPath(home string) string` → `~/.codex/config.toml` (per 199's
     Findings — TOML, not JSON, so this needs a TOML encoder/merge, not
     `internal/jsonc`; check what TOML library (if any) is already a
     dependency before adding one).
   - `BuildHooksDoc()`/`Apply()`/`Status()`/`Remove()` following the same
     "only touch the harnez-owned `[hooks.harnez]` table, preserve every
     other top-level key and named hook" contract `internal/agy` uses —
     this is a TOML analog of `mergeHooksDoc`, not a straight port.
   - `PreToolUse` matcher/hooks entry per 199's Q2 schema: `matcher =
     "Bash"` (or the shell-tool equivalent — confirm exact tool name
     Codex uses for shell execution before hardcoding "Bash"), `type =
     "command"`, `command = "harnez codex-hooks hook"`.
2. **`cmd/harnez/codexhooks.go`** (new, parallel to
   `cmd/harnez/agyhooks.go`):
   - `apply` — writes/merges `~/.codex/config.toml`'s `[hooks.harnez]`
     table.
   - `status` — installed/drifted report.
   - `hook` — PreToolUse handshake: read Codex's documented stdin schema
     (199's Q2: `hookEventName`, `tool_name`, `tool_input`, `tool_use_id`,
     `session_id`, `turn_id`, `cwd`, `transcript_path`, `permission_mode`,
     `model`), and for a non-empty, not-already-routed shell command,
     write `{"permissionDecision":"allow","updatedInput":{...}}` pointing
     the command at `harnez exec --tool <cmd> -- bash -c '<original>'` —
     same `bash -c`-wrapping rationale `harnez exec hook`/`harnez
     agy-hooks hook` already use (shell metacharacters must survive as one
     argument). Reuse `alreadyRoutedThroughExec`/`shellQuote` from
     `cmd/harnez/exec.go` rather than duplicating them.
3. **Hook-trust interaction** (Codex-specific, not present for Claude/agy):
   199's Q5 found hook trust is a gate independent of `enabled` — new or
   modified hooks require explicit trust review
   (`--dangerously-bypass-hook-trust` exists to skip it for automation).
   Document in this ticket's own design notes (or a linked doc) how
   `harnez codex-hooks apply` should communicate this to the user (e.g. a
   printed note that Codex will prompt for trust review on first use,
   rather than silently assuming the hook is active immediately after
   `apply`).
4. **`timeoutSec` default**: 199 flagged that the real default hook
   timeout value wasn't recoverable from binary strings alone. Pin this
   down (upstream Codex docs, or an experiment writing an explicit
   `timeoutSec` and observing behavior) before deciding whether
   `BuildHooksDoc()` should set an explicit value or omit the field and
   rely on Codex's own default.
5. **Wire into `cmd/harnez/main.go`**'s root command, same as `agy-hooks`.

## Non-goals

- No PATH-shim fallback for Codex — 199's recommendation explicitly
  prefers the native route; do not also implement a `codex`-targeted
  PATH-shim in this ticket (a separate ticket if ever warranted).
- No automatic wiring into the global `harnez apply`/`init` — land as its
  own opt-in command group first, matching how 196 landed `agy-hooks`
  standalone rather than folding into `apply`.
- Not attempting `PostToolUse`/telemetry-parity wiring in v1 — same call
  196 made for agy: `harnez exec` already records the `tool_calls` row
  once the rewritten command runs, so a second `PostToolUse` hook would be
  redundant.

## Acceptance Criteria

1. `internal/codex` package exists with `Apply`/`Status`/`Remove` covering
   create, idempotent-reapply, drift detection, and preserve-unrelated-
   content semantics (mirroring `internal/agy/hooks_test.go`'s coverage).
2. `harnez codex-hooks apply` writes a well-formed `~/.codex/config.toml`
   `[hooks.harnez]` `PreToolUse` entry, verified against a real
   `CODEX_HOME`-scoped `codex --strict-config doctor` run (per 199's Q2
   verification method) so the config is confirmed schema-valid, not just
   internally self-consistent.
3. `harnez codex-hooks hook` correctly rewrites a real shell command
   (including one with shell metacharacters, e.g. `&&`) into a
   `harnez exec`-routed form, and passes through an already-routed or
   empty command unchanged — mirroring `cmd/harnez/agyhooks_test.go`'s
   coverage.
4. Hook-trust interaction is documented (see Scope point 3) so a user
   running `harnez codex-hooks apply` isn't surprised when Codex prompts
   for trust review.
5. `go test ./...` and `make install` pass.
