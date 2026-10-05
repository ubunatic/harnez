# 712 — Add support for the Pi coding agent CLI 1.0+

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [#291 external coding-agent dispatch](291-add-harnez-agent-command-standardized-non-interactive-dispatch-to-external-coding-agent-clis-codex-agy.md), [#302 Pi and agent harness research](302-research-plugin-systems-for-pi-agents-deepseek-harness-and-prime-agent-self-modification.md), [#070 Pi cross-agent canaries](070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode.md)

---

## 1. Problem & Motivation
Harnez's agent runtime has no first-class driver for the Pi coding agent.
Support the current Pi CLI, focusing on version 1.0 and later, so it can be
selected alongside existing agent backends for coding tasks.

## 2. Technical Specification / Findings
Verify the current 1.0+ CLI's documented invocation, model selection,
non-interactive output, and session lifecycle before implementing the adapter.
Treat older pre-1.0 behavior as out of scope unless compatibility proves
necessary. Existing Pi canary and extension research do not provide agent-driver
support.

## 3. Implementation & Verification Plan
/goal Add and document a Pi 1.0+ driver that supports the Harnez agent task and
session workflow, with command-construction tests and a live canary against a
current Pi release; or stop and report when blocked on user input or denied
permission.
