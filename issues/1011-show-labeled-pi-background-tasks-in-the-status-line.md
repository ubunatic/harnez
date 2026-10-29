# 1011 — Show labeled Pi background tasks in the status line

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1009, 1003; `docs/PiAgent.md`; `f72ec68f`, `5c5ab8ba`, `5ac6cb19`, `6bb791e6`

---

## 1. Problem & Motivation

The Pi background-task status currently shows only `Tasks: <num>`, so the user cannot tell which tasks are running at a glance.

## 2. Goal

/goal Show the running task count and compact labeled entries in the Pi status line, verify the behavior, and stop and report if blocked by a user decision or denied permission. On a loop, hard architectural or OS blocker, or lack of progress, clear the goal and record a failure report.

Use the user-approved format, for example:

```text
Tasks (3): ¹🔨build, ²🧪tests, ³👤backup
```

The calling agent supplies a single-word label when creating a task; manual `/task` jobs use the fixed 👤 emoji. Bound the display for narrow terminals or many tasks while retaining the total, for example `Tasks (7): ¹🔨build, ²🧪tests, ³👤backup, +4`. Preserve task execution, completion notifications, and existing workflows.

## 3. Live-Code Findings (2026-10-29)

- `.pi/extensions/background-tasks.ts`: `updateStatus` filters `status === "running"`, emits `Tasks: N` through `ctx.ui.setStatus("background-tasks", text)`, and clears it with `undefined` at zero. It runs on runner `changed` events and session start. Both `background_task` and `/task` currently pass only command and cwd to `runner.start`; the tool schema accepts only `command`.
- `.pi/lib/background-task-runner.ts`: `BackgroundTask` has no display metadata. `start(command, cwd)` allocates increasing numeric IDs per runner; finished-task eviction does not renumber surviving tasks. Result/cancel tools and commands resolve the same canonical positive safe-integer IDs. An unconfirmed cancellation remains `running` until settlement.
- `test/pi/background-tasks.test.ts` exercises command-only tool calls and direct runner starts, numeric ID validation, retention, cancellation, shutdown and notifications; it explicitly asserts `Tasks: 1`. Its fake UI captures status strings but has no terminal-width/resize model. The TypeBox test stub does not prove real Pi schema acceptance.
- `spec/pi-background-tasks.yaml` and `spec/schemas/pi-background-tasks.schema.json` currently specify output, retention and stop bounds only. The runtime parser rejects unknown/missing keys and requires JSON-compatible YAML; schema changes alone are insufficient. `pi_background_spec_test.go` validates the embedded spec.
- `f72ec68f` hardened lifecycle behavior (1003). `5c5ab8ba` added event-bus coordination for duplicate extension copies; `5ac6cb19` installs the runner/library/spec globally; `6bb791e6` documents that installation (1009). Extend the shared payload and preserve duplicate-load protection rather than adding a second runner or installer.

## 4. Decision and Implementation Constraints

- **Approved intent:** count plus compact ID/emoji/word entries, agent-supplied single-word labels, fixed 👤 for manual tasks, and bounded display retaining the total. The example does not establish a fixed three-entry limit, a universal emoji vocabulary, or who selects agent-task emoji.
- **Compatibility decision:** introduce additive display metadata; keep command-only tool calls and `start(command, cwd)` valid. Instruct new agent calls to supply a label, but use a safe, documented fallback when legacy callers omit it. Keep `/task <command>` syntax and shell command semantics intact. Choose and document manual/legacy label derivation without executing or rewriting the command; no new required manual arguments.
- Validate/sanitize labels before display: one word, bounded length, no newlines, control characters or terminal escape injection. Define treatment of empty, whitespace-only, multi-word and overlong inputs. Exact field names, fallback word, length bounds and agent-emoji selection remain implementation choices to document, not additional user approvals recorded here.
- Store metadata on the task before its initial `changed` event, so the first status is correct. Display only running tasks, in stable task-ID order. Superscript the actual decimal task ID digit by digit (including 10 and above); do not substitute the entry's current position. APIs and `/task-result`/`/task-cancel` continue accepting ordinary numeric IDs; labels are not identifiers and need not be unique.
- Keep the running total independent of finished-task retention and visible entries. Recompute on start/settlement, keep unconfirmed cancellations visible, and clear at zero. Preserve output capture, process-group cancellation, exactly-once natural completion/failure follow-ups, cancellation/shutdown notification rules and timer cleanup.
- Put new display bounds/default labels/emoji policy values in the spec with matching schema, loader and fixture updates; retain JSON-compatible YAML and strict validation. Do not duplicate spec defaults in TypeScript. Update `docs/PiAgent.md` with the chosen caller contract and fallback behavior during implementation.
- Bound using terminal display cells, not JS string length; avoid splitting surrogate pairs, grapheme clusters, emoji sequences or IDs. Reserve the count and overflow suffix first; `+K` counts omitted running entries. When no entry fits, fall back to the count alone. If even the count cannot fit, document the physical limitation instead of promising impossible visibility.
- **Feasibility check before rendering work:** the current `setStatus` call supplies a string and no allocated footer width. Verify the installed Pi API's width/resize facilities and its handling of other footer statuses; terminal columns alone may overestimate available space. Use a supported integration and deterministic formatter tests; distinguish the extension's width budget from any later Pi truncation. Record the chosen budget, resize behavior, and limitations without replacing the whole footer merely to render this status.

## 5. Implementation Verification (Not Performed by This Refinement)

- Extend automated tests for the approved three-task example, supplied labels and legacy command-only calls, manual 👤 tasks, label validation/fallbacks, duplicate labels and unchanged command execution. Retain lifecycle assertions while updating the old count-only expectation.
- Cover count/entry updates after success, failure, confirmed cancellation, unconfirmed cancellation and eventual settlement; clear at zero/shutdown. Cover nonconsecutive IDs after completion/eviction, 9→10 and larger IDs, stable ordering, and unchanged numeric result/cancel lookup.
- Test many tasks, exact width boundaries, no-entry/count-only fallback, correct `+K`, long labels, Unicode/emoji cell widths and resize behavior. Add an appropriate `.ansi` mockup under `docs/data/` per `docs/AgenticLoop.md` before TUI implementation.
- Keep schema/parser validation and spec fixtures aligned. Run the Pi tests (`make test-pi`, including `background-claim.test.ts`) and the applicable Go spec/embed/install checks through the repository's quota workflow; no installer redesign is needed.
- Verify in a live Pi session with the real tool schema, agent and manual starts, lifecycle updates, a narrow/resized terminal, other footer statuses, and coordinated project/global copies. Record environment limitations honestly; fake UI tests alone cannot prove footer behavior. Installation/live checks belong to implementation, not this ticket-only refinement.
