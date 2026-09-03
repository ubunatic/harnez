# 207 — Determine real Codex `timeoutSec` default for PreToolUse hooks

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: [[200-codex-native-hooks-preTooluse-wiring]] (`BuildHooksDoc`
in `internal/codex/hooks.go` intentionally omits `timeoutSec`), [[199-research-codex-hook-surface-for-transparent-exec-distill]]
(confirmed the field exists but couldn't recover its default value from
binary strings)

## Problem

Codex's `[hooks.<name>]` PreToolUse entries support a per-hook
`timeoutSec` field, but its actual default numeric value wasn't
recoverable from binary strings alone (199's Q5). `BuildHooksDoc()`
currently omits the field entirely, letting Codex apply its own built-in
default — which is fine as long as that default is reasonable for
`harnez exec` calls, but nobody has confirmed what it actually is.

If Codex's real default is too short, slower `harnez exec` invocations
(e.g. distill pipelines) could hit spurious "hook timed out" failures
that would look like harnez bugs.

## Scope

- Check upstream Codex docs/changelog for a documented `timeoutSec`
  default.
- Failing that, run a live experiment: install a hook with a
  deliberately slow command and binary-search/observe when Codex times
  it out.
- Decide whether `BuildHooksDoc()` should pin an explicit `timeoutSec`
  value once the real default is known (only if the default turns out to
  be too aggressive for realistic `harnez exec` runtimes).

## Non-goals

- No config change unless the discovered default is actually a problem.
