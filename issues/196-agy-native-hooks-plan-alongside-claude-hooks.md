# 196 — Plan: use real agy hooks (`hooks.json`) alongside Claude Code hooks

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research / Feature
**Related**: [[193-research-agy-hook-surface-for-transparent-exec-distill]] (research
ticket that discovered agy's `hooks.json` lifecycle-hook system — read its
Findings section first), [[195-path-shim-wrapper-for-agy-exec-distill-interception]]
(the quiet PATH-shim alternative this ticket explicitly does NOT overlap with),
`internal/claude/apply.go` (existing Claude Code `PreToolUse`/hook wiring this
should mirror in spirit)

## Problem

Issue 193's research found that `agy` (Antigravity CLI) has a real, documented
hook system — `hooks.json` (global at `~/.gemini/config/hooks.json`, or
workspace-local at `<workspace>/.agents/hooks.json`) — supporting
`PreToolUse`/`PostToolUse`/`PreInvocation`/`PostInvocation`/`Stop` events, a
`matcher` regex on tool/step type, and a `command`-type handler whose stdout
can return `{"decision": "allow"|"deny"|"ask"|"force_ask", "overwrite":
{"CommandLine": "..."}}`. This is structurally the same shape as Claude Code's
own `PreToolUse` hook that `harnez apply` already wires up for `harnez
exec`/`harnez distill` — but for agy it currently goes unused because 193 was
scoped to the "without telling agy" quiet-interception question (which 195
now covers via PATH-shimming instead).

Using agy's *native* hook surface is a different, complementary path: it
requires an explicit, agy-side config write (not quiet), but in exchange gets
a real, documented, forward-compatible mechanism — including the
`overwrite.CommandLine` rewrite capability, which is more general than a
PATH shim (works for any tool call agy dispatches, not just ones that
happen to resolve through `$PATH`, and doesn't depend on agy's PATH-lookup
behavior remaining unchanged in future releases).

## Task — plan only, no implementation yet

This ticket is scoped to producing a plan, not code. Write the plan as a new
section in this ticket (or a linked `docs/studies/` doc if it grows large)
covering:

1. **Where the config lives**: decide global (`~/.gemini/config/hooks.json`,
   parallel to how `harnez apply` manages global `~/.claude` state) vs.
   per-workspace (`<workspace>/.agents/hooks.json`, parallel to `harnez init`
   writing project-local state) — mirror the existing `apply`/`init` split
   documented in `docs/CLIDesign.md` rather than inventing a third convention.
2. **What harnez would manage**: likely a `PreToolUse` hook (matcher on
   shell/`run_command` steps) whose handler shells out to the same
   `harnez exec`/`harnez distill` semantics the Claude Code hook already
   uses, returning `overwrite.CommandLine` to route the call. Confirm via
   193's findings whether `PostToolUse` is also worth wiring (e.g. for
   telemetry/rating capture parity with Claude Code's PostToolUse-driven
   `harnez rate` prompting, if any exists).
3. **Drift detection / `harnez status` coverage**: unlike the PATH-shim
   (195), a real config file write CAN be linted the same way `harnez apply`
   already checks `~/.claude/settings.json` for drift — plan how
   `harnez status`/`harnez apply --check` would detect a missing or
   hand-edited `hooks.json` entry.
4. **Relationship to 195**: state plainly whether both mechanisms would ever
   run simultaneously for the same user (e.g. native hooks for `PostToolUse`
   telemetry, PATH-shim for the read-only `git` fast path) or whether they're
   mutually exclusive alternatives the user picks one of. Do not let this
   ticket silently duplicate 195's scope.
5. **Command surface**: sketch the likely new subcommand(s) (e.g. `harnez
   agy-hooks apply`/`agy-hooks status`, or folding into existing `apply`/
   `init` with an `--agy` flag — check `docs/CLIDesign.md`'s apply/init
   separation before assuming the latter fits) without committing to exact
   flag names yet.

## Acceptance Criteria

1. Plan is written into this ticket (or a linked doc) covering the five
   points above.
2. Plan explicitly states the relationship to ticket 195 (complementary,
   overlapping, or superseding) so a future implementer doesn't have to
   re-derive it.
3. No code changes in this ticket — implementation gets its own follow-up
   ticket(s) once the plan is reviewed.
