# 687 — Add an interactive usage dashboard mode

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [usage view modes](678-make-usage-view-modes-spec-driven-and-independent-of-watch.md), [watch controls overlay](094-usage-watch-controls-overlay-and-presets.md)

---

## 1. Problem & Motivation
`harnez usage` needs a dashboard view with discoverable actions alongside its
usage panels. Users should be able to launch common CLI commands from the UI
with a mouse or keyboard.

## 2. Technical Specification / Findings
Add `--dashboard` as a usage view mode with an additional actions box. Define
its buttons and the CLI commands they invoke in `spec/`, with schema validation.
Add `--tui` for the live interactive UI, like `--watch` with mouse input
enabled; bind dashboard actions to otherwise unused hotkeys in this mode.
Keep `--watch` independently applicable, and preserve the view-mode behavior
specified in issue 678 whether or not watch rendering is enabled.

## 3. Implementation & Verification Plan
**Goal**: Provide a spec-configured usage dashboard whose actions can be
invoked by mouse or unused hotkeys under `--tui`. Done when `--dashboard`
selects the added actions box, `--tui` enables mouse interaction with the live
UI, configured commands launch correctly, and existing view modes behave
consistently with and without `--watch`. Stop and report if a command action
cannot be specified or safely dispatched.

## 4. Decisions (2026-10-02)
- Actions are spec-defined (`title`, `key`, `args`) and run the current harnez executable with
  `args`, never a shell. Keys must be unused in `spec/actions.yaml`; the sketch's `[a]` collided
  with "toggle All Usage", so Agent sessions uses `[s]`.
- `--dashboard` alone renders once. Action keys work under both `--watch` and `--tui`, because the
  Actions panel shows them; `--tui` adds mouse clicks and enables mouse reporting only when the
  view has actions.
- Ctrl-C while an action runs quits the whole dashboard, not just the action. Changing that needs
  SIGINT handling around the child; open if it bothers in practice.
- Known edge: input typed during an action without Enter can dismiss the
  "[press any key to return]" prompt immediately.
