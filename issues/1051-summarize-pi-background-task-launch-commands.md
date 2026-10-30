# 1051 — Summarize Pi background task launch commands

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1011, 1021, 1046, 1050

## Problem and goal

When an agent launches a background task, the user-facing tool display shows the full command, including potentially long prompts. The owner wants a brief display or ellipsis there too, consistent with Pi's other truncated tool displays. This concerns the launch display, distinct from completion output (1046) and output capture (1021).

/goal Make background-task launch commands concise in the user-facing Pi display, with a clear truncation indicator and access to full details where supported, while preserving existing command execution semantics and useful task identity; verify in real Pi and automated tests, or stop and report if an API, user decision or permission is needed. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Re-check live code, Pi rendering APIs and recent history first. Investigate Pi's existing tool-call ellipsis and expansion behavior and what the agent and persisted transcript actually receive; don't imply visual truncation hides sensitive data. Test short, long, multiline and sensitive commands, expansion or retrieval where available, and compatibility with task labels and lifecycle.

## Decision and sprint scope

Implement launch presentation as an independent slice, without changing output capture or completion delivery (1021/1046). Use Pi's supported tool rendering and expansion where practical; keep the executed command byte-for-byte unchanged, and ensure the immediate launch result does not dump it again. Verify both collapsed and expanded views in real Pi, plus actual agent-visible tool result and transcript behavior. Any new configurable bounds belong in spec and schema. Expected write set: `.pi/extensions/background-tasks.ts`, relevant `test/pi/` tests and stubs, spec/schema and spec tests if needed, and `docs/PiAgent.md`; update if live discovery proves more is needed. Preserve existing lifecycle and coordination assertions.

## Review pre-work

Review of `b34640d0` found a derived-label leak candidate: shell assignments such as `TOKEN+=dir/sk_live_x` may expose the final path component in the launch label. Reproduce with a synthetic secret and harden label derivation without changing command execution; preserve existing label and lifecycle tests. Verify actual Pi collapsed/expanded presentation and agent-visible result, document the global-install path and safely determine how to refresh it without unrelated global changes. The manual `/task` launch still prints the command and is outside this agent-launch slice; document the limitation. The existing runner trims outer whitespace, so this issue preserves existing semantics rather than promising byte-for-byte input preservation.

## Delivery and verification

`b34640d0` makes agent launch results concise; `81b2d5bb` guards assignment-derived labels. Host `make test-q1` passed after both changes (77 Pi tests and Go checks). A real Pi model launch returned the concise result and a disposable persisted transcript retained the full arguments; an isolated Pi TUI with a synthetic saved tool call showed truncation and Ctrl-O expansion. The TUI canary was not itself model-driven. Independent review found no blocking repository-code defect. Manual `/task` and later completion/list/result views may still show commands; truncation is not secrecy.

Global install is not updated: `make install` does not refresh it and the available broad apply can affect unrelated configuration. Safe targeted refresh and a global canary are parked in 1052; until then the global extension retains the old behavior.
