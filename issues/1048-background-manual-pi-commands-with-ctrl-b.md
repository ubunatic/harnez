# 1048 — Background manual Pi commands with Ctrl-B

**Status**: Blocked — Pi 1.4.0 has no supported running shell transfer API
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1011, 1021, 1046, 1047; `.pi/extensions/background-tasks.ts`

## Ask

Let the user background their own manual Pi shell commands with **Ctrl-B**, integrated with the existing task system. For example, after starting a manual `sleep 10`, the user wants to leave it running in the background rather than wait for it in the foreground. The owner confirmed the chat remains active while a manual command runs and that Ctrl-B currently moves the cursor back one character; explicitly override that chat binding for this workflow.

## Discovery and blocker

A read-only audit of Pi 1.4.0 found no supported extension API to take ownership of an already-running interactive shell command. Starting a new managed task is not a transfer and rerunning the command could duplicate side effects. Pi runtime support or an owner decision to accept a different interaction is needed; reverify live APIs before resuming. No real-TUI transfer canary was run in this audit.

## Goal

/goal Provide and verify a Ctrl-B workflow for backgrounding manual Pi commands as managed tasks, or stop and report if Pi cannot support it or a user decision or permission is needed. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Re-check current Pi APIs, keybinding conflicts, extension behavior and recent history before implementation. Determine whether an already-running manual shell command can be transferred safely; if not, report the limitation and discuss an alternative rather than claiming equivalence. Preserve process ownership, output, exit status, cancellation and task labels; avoid rerunning commands or losing results. Test conflicts and lifecycle behavior and verify the interaction in a real Pi session.
