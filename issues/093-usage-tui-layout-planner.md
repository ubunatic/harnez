# 093 — Usage TUI layout planner and sizing policy

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: issues/083-usage-tui-self-hiding-auto-discovery.md, issues/084-aggregate-quota-window-box-assessment.md, issues/088-load-panel-ram-vram-gtt-memory.md, issues/089-load-panel-combine-gpu-vram-gtt-row.md

---

## 1. Problem & Motivation

`harnez usage` has accumulated enough panels and modes that the layout rules now feel mostly correct but brittle. Summary/watch output handles normal terminals, hidden-panel hints, and compact mode, but observed frames show layout friction at width/height boundaries and when flags interact with compact startup state.

Observed commands:

- `~/go/bin/harnez usage --summary`
- `~/go/bin/harnez usage --summary --proc`
- `COLUMNS=80 LINES=24 ~/go/bin/harnez usage --summary --proc`
- `stty cols 100 rows 24` then `~/go/bin/harnez usage --watch --compact`
- `stty cols 100 rows 24` then `~/go/bin/harnez usage --watch --compact --proc`
- `stty cols 80 rows 18` then `~/go/bin/harnez usage --watch --proc`
- `stty cols 120 rows 18` then `~/go/bin/harnez usage --watch --compact --proc`
- Interactive toggles in compact watch: `A`, `p`, `l`, `a`, `q`

## 2. Findings

- At 80 columns with `--summary --proc`, quota rows fit only by dropping some time labels, while load rows truncate values such as `24...` and `7%...`. That is acceptable as a fallback, but the policy is implicit and panel-specific.
- At 80x18 with `--watch --proc`, the UI hides two panels as "terminal too short"; the hint is useful, but it only reports a count and does not identify which visible-by-default panels were omitted.
- `--watch --compact --proc` starts with hidden state `[C] [G] [O] [H] [P]`, so the process panel remains hidden even though `--proc` explicitly requested it.
- Compact mode gives `All Usage` more width than `Load`, causing load temperature rows to truncate at 100 columns while all-usage has spare horizontal space.
- Toggling `A`, `p`, `l`, and `a` works, but the packing result depends on special-case state: all-usage can become full-width, processes can be manually added, and hidden-panel hints then switch between explicit panel letters and generic "N panel(s) hidden" height overflow messages.

## 3. Suggested Direction

Add a small layout planner / sizing policy for usage panels rather than a broad TUI rewrite. Each panel should declare:

- minimum width, preferred width, and whether it can stretch;
- minimum height and optional collapsed height;
- priority when terminal height is constrained;
- whether a CLI flag makes the panel requested even in compact mode;
- a short hidden reason/label for footer hints.

The renderer can then plan rows/columns once, allocate spare width deliberately, and make compact/all-usage behavior a policy decision instead of a cluster of local special cases.

## 4. Acceptance Criteria

- `--watch --compact --proc` shows the process panel or clearly documents/renames the flag interaction.
- At 80-column summary/watch widths, truncation is deliberate and consistent across agent, all-usage, process, history, and load panels.
- Height overflow hints identify omitted panel keys when possible, not only the number of hidden panels.
- Compact mode distributes width so `All Usage` does not starve `Load` when both are visible.
- Layout tests or golden frame checks cover at least 80x18, 80x24, 100x24, and 120x18 with `--summary`, `--summary --proc`, `--watch --compact`, and `--watch --compact --proc`.

## 5. Verification Notes

This is a review-only ticket. No Go code was changed.

The assessment used bounded watch runs with `timeout` and an interactive PTY session exited with `q`; no watch process was left running.
