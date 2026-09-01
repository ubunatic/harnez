# 153 — Assess whether command-tree placement should be spec-driven

**Status**: Open
**Priority**: P3 (Low)
**Severity**: N/A (assessment)
**Category**: CLI Design / Research
**Related**: [[152-move-agent-collector-under-usage-command]] (first concrete instance of a
misplaced command this ticket generalizes from), `docs/other/Spec.md` (existing `spec/` YAML +
JSON Schema convention, e.g. `spec/actions.yaml`, `spec/colors.yaml`), `docs/CLIDesign.md`,
`cmd/harnez/main.go`

> **Note**: this is explicitly an assessment ticket, not a committed feature. It may conclude
> "not worth it, too speculative" and get dropped — that is an acceptable outcome. Do not build a
> generalized command-placement engine as a foregone conclusion.

## Problem

`cmd/harnez/main.go` has grown to 17 top-level commands (`apply, diff, scan-docs, clean, status,
usage, load-stream, init, assess, agent-collector, distill, mode, release, statusline, rate, exec,
stats, index`), several of which share overlapping concerns (e.g. `agent-collector` vs. `usage`,
see [[152]]) and ended up at the top level or under a given parent mostly because that was the
obvious/convenient choice at the moment each was added — not because of a deliberate structural
rule. As more commands get added, this pattern will likely repeat: new commands default to
top-level, second-level verb groupings (`usage history timeline`, etc.) accumulate ad hoc, and
there's no single place that states the intended tree shape or a rule for "does this belong at the
top level or nested."

## What to assess

1. **Is the current tree actually a problem**, or does it just look busy? Inventory the 17
   top-level commands and their subcommand depth; identify how many are genuinely standalone verbs
   vs. how many share a concern with an existing parent (like [[152]]'s case) and could nest without
   losing clarity.
2. **General principle**: prefer grouping related commands under a shared parent (second-level
   verbs) over adding more flat top-level commands, so the top-level namespace stays small and
   scannable. Assess whether this project already has enough natural groupings (`usage`, `history`,
   others) to reorganize around, or whether some commands are genuinely orthogonal and belong at
   the top level.
3. **Should the command tree be declared in `spec/` (a new `spec/commands.yaml` alongside
   `actions.yaml`/`colors.yaml`, per `docs/other/Spec.md`'s existing convention) rather than only
   living as `cobra.Command{}` construction + `AddCommand()` calls in `cmd/harnez/main.go`?**
   - Scope such a spec to structure only — which command nests under which parent — not flags.
     Flags stay attached to their own command definitions in code as today; only placement
     (top-level vs. nested, and under which parent) would be spec-driven.
   - Weigh benefit (a single reviewable source of truth for the tree shape, easier to reason about
     "where should X go" when adding a new command, consistency check similar to
     `spec/actions.yaml`'s no-stale-identifiers rule) against cost (indirection between the spec and
     the actual `cobra.Command` wiring, another thing to keep in sync, and whether Cobra's
     construction style even fits being spec-generated cleanly).
4. **Explicitly consider dropping this ticket.** If the assessment concludes the tree isn't a real
   problem yet, or that spec-driving placement adds more indirection than it saves for a CLI this
   size, say so and close the ticket rather than building the mechanism anyway.

## Deliverable

A short written assessment (in this ticket or a linked note) covering: current tree inventory,
whether reorganization is warranted now, and a recommendation on whether a `spec/commands.yaml`-style
mechanism is worth building — or whether ad hoc placement + occasional manual moves (like [[152]])
is sufficient at this project's current scale.

## Out of Scope

- Implementing a spec-driven command-tree mechanism — this ticket only assesses whether to.
- Reorganizing any specific command's placement other than [[152]]'s `agent-collector` move, which
  is filed separately and not blocked on this ticket.
