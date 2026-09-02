# 195 — Implement a PATH-shim wrapper to route agy's shelled-out tool calls through harnez exec/distill

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[193-research-agy-hook-surface-for-transparent-exec-distill]] (research
ticket that verified the mechanism — read its Findings section first),
`cmd/harnez/exec.go` (`newExecHookCmd`, the existing Claude `PreToolUse` hook
this generalizes), `internal/claude/apply.go`

## Problem

Issue 193 confirmed experimentally that `agy` resolves its `permissions.allow`
shell commands (e.g. `git status`) via inherited `$PATH` lookup, not hardcoded
absolute paths — a directory of shim scripts prepended to `PATH` before `agy`
starts is invoked in place of the real binary, without any change to agy's own
`~/.gemini/antigravity-cli/settings.json`. This is a real, working "quiet"
interception point, analogous to (but more general than) the existing
`alias agy="ANTIGRAVITY_AGENT=1 agy"` dotfiles pattern.

## Scope

Implement an opt-in wrapper mechanism (likely a new `harnez agy-shim` command
or a `harnez init`/`apply`-managed shim directory, TBD during design) that:

1. Generates a directory of thin shim scripts — one per command the user
   wants intercepted (start with `git`, generalize later) — each of which
   pipes the invocation through `harnez exec`/`harnez distill` semantics
   before `exec`-ing the real binary via its absolute, PATH-independent
   location (resolved once at shim-generation time, not at call time, to
   avoid the shim recursively resolving itself if placed earlier in `$PATH`
   than the real binary).
2. Provides a single alias/env wrapper (mirroring `alias agy="PATH=<shimdir>:$PATH agy"`)
   so the interception is scoped to sessions that opt in via the alias, not
   global to the user's shell — this bounds the blast radius identified in
   193's findings (a broken/hanging shim only affects `agy` invocations made
   through the wrapper alias, not every shell command system-wide).
3. Ships a clear, fast disable path — removing/commenting the alias — and
   ideally a `harnez agy-shim doctor`-style self-check that runs each shim
   against a trivial input and confirms it falls through to the real binary
   correctly, so users can verify the shim isn't silently swallowing calls
   before relying on it.
4. Does NOT modify any agy-side config file (`settings.json`, `hooks.json`,
   `mcp_config.json`) — the whole point is the alias/PATH-only interception
   path that requires zero agy-side opt-in.

## Non-goals

- Do not attempt to also implement the agy-native `hooks.json` /
  `<workspace>/.agents/hooks.json` PreToolUse mechanism found during 193's
  research — that requires editing agy's own config and is a different,
  non-"quiet" mechanism; if pursued at all it should be a separate ticket
  explicitly framed as "agy-side opt-in" (parallel to how Claude's `apply`
  wires the PreToolUse hook) rather than folded into this quiet-shim ticket.
- Do not implement generalized shimming for every possible `permissions.allow`
  command in the first pass — land `git` first (the most common
  read/log-heavy agy command in this user's `permissions.allow` list) and
  extend incrementally.

## Acceptance Criteria

1. A shim-generation command/mechanism exists and is documented.
2. At least one real command (`git`) is shimmed end-to-end and verified live
   against an actual `agy -p` invocation, with `harnez exec`/`distill`
   observably running in the middle (not just a log line — confirm the
   distilled/wrapped output actually reaches agy's tool-result path).
3. A one-line disable path is documented and tested (remove the alias, next
   `agy` invocation runs unshimmed).
4. `go test ./...` and `make install` both pass after the change.
