# 152 — Move `agent-collector` under `usage` instead of top-level

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Cosmetic
**Category**: CLI Design
**Related**: [[153-command-tree-placement-spec-assessment]] (broader question this is the first concrete
instance of), `cmd/harnez/main.go`

## Problem

`cmd/harnez/main.go` registers 17 top-level commands on `root`
(`apply, diff, scanDocs, clean, status, usageCmd, loadStreamCmd, initCmd, assessCmd, collectorCmd,
distill, mode, release, statusline, rate, exec, stats, index`). `agent-collector` (`collectorCmd`,
main.go:316) is one of them, but its concern — collecting agent usage/quota data — is a `usage`
concept: `usageCmd` already exists as a parent with a `history` subcommand tree
(`timeline`/`fetch`/`record`/`stats`, main.go:309-310). `agent-collector` sitting at the top level
instead of under `usage` is exactly the kind of ungrouped command the top-level namespace is
accumulating too many of.

## Scope

- Move `agent-collector` to be a subcommand of `usage` (e.g. `harnez usage agent-collector`, or a
  shorter subcommand name if one reads better under the `usage` parent — decide during
  implementation).
- Update `root.AddCommand(...)` to drop `collectorCmd` from the top-level list and add it via
  `usageCmd.AddCommand(...)` instead.
- Update any docs, help text, or scripts referencing `harnez agent-collector` at the old path.
- Check `docs/README.md`/`docs/other/Spec.md` or other docs for literal `agent-collector` usage
  examples that need the new path.

## Out of Scope

- The broader command-tree/spec question — see [[153]]. This ticket is scoped to one concrete
  move, independent of whether 153 proceeds.
