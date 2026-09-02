# 197 — "Agent shell enhancements": user-invoked aliases/shell funcs feature

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[193-research-agy-hook-surface-for-transparent-exec-distill]]
(the PATH-shim experiment that incidentally proved out this mechanism — the
live test smuggled in a `gss` git alias via the shim dir),
[[195-path-shim-wrapper-for-agy-exec-distill-interception]] (the agy-facing
use of the same shim-dir mechanism — this ticket is the human-facing sibling,
not a duplicate), `docs/CLIDesign.md` (apply/init scope split — check before
deciding where this installs to)

## Problem

While building and live-testing the PATH-shim used for ticket 193's research,
a `git` shim script directory was used to smuggle in short aliases (e.g.
`gss` for a common git status variant) alongside the pass-through shim logic.
That was incidental to 193's research goal, but it demonstrated a genuinely
useful, separate feature: harnez could manage a directory of small shell
aliases/functions as a first-class, opt-in convenience layer — **not**
primarily for agent tool-call interception (that's 195's job), but for the
**user themselves**, typed directly at the prompt via Claude Code's `!
<command>` passthrough (see this project's own session guidance: "suggest
`! <command>` for interactive commands the user should run themselves").

## Scope

Add a "agent shell enhancements" feature: a harnez-managed directory of
short, memorable shell aliases/functions (starting with a small git-focused
set, e.g. `gss`, generalizing as more are requested) that:

1. Installs via the existing `apply`/`init` mechanics (check
   `docs/CLIDesign.md` for which one owns shell-rc-sourced content — likely
   `apply` if these are meant to be global/personal, `init` if meant to be
   project-scoped; do not blur the two, per this project's load-bearing
   apply/init separation).
2. Is sourced from the user's shell rc (a single `source
   ~/.claude/shell/agent-aliases.sh`-style line, managed/idempotent the same
   way `harnez apply` already manages other dotfile insertions — check
   existing apply drift-detection patterns before inventing a new one).
3. Ships a small, curated starter set (document the initial list; `gss` from
   193's live test is a natural first entry — confirm its exact definition
   with the user rather than guessing) plus a documented, low-friction way to
   add more without editing harnez's Go source (a YAML/text list under
   `spec/` or a dotfiles-managed file, consistent with this repo's existing
   "spec, not hardcoded Go strings" convention used by `spec/actions.yaml`
   etc.).
4. Is explicitly framed (in docs and `--help` text) as a convenience for
   **interactive human use via `! <command>`**, not as the agent-interception
   mechanism — keep the docs clearly distinct from 195's agent-facing framing
   so future readers don't conflate the two despite the shared shim-dir
   origin story.

## Non-goals

- This is not the agy/Claude tool-call interception mechanism (195/196) —
  no wiring into `PreToolUse` hooks or PATH-shimming for agent-invoked
  commands here. If overlap turns out to be desirable (e.g. sharing a shim
  directory), that's a follow-up integration ticket, not this one.
- Not attempting a full alias-management framework (no per-alias enable/
  disable UI, no templating) in the first pass — a flat curated list is
  enough to start.

## Acceptance Criteria

1. A documented starter set of aliases/functions exists in a spec/config
   file (not hardcoded Go string literals).
2. `apply`/`init` (whichever is chosen — document the choice and why)
   installs the sourcing line idempotently, matching existing dotfile-drift
   conventions.
3. At least one alias (`gss` or equivalent) is verified working end-to-end
   in an interactive shell after install.
4. Docs updated to clearly distinguish this (human-typed, `! <command>`
   convenience) from ticket 195's agent-interception PATH-shim, despite
   sharing the shim-dir mechanism's origin.
5. `go test ./...` and `make install` pass after the change.
