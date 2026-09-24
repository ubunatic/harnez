# 556 — Show basic context numbers in the Claude status line

**Status**: In Progress
**Priority**: P2
**Severity**: Moderate
**Category**: Feature
**Related**: [[534-capture-agent-session-tokens-continuously-not-only-at-session-end]]

---

## Problem & Motivation

Claude Code exposes live context-window usage in its status-line payload, but the
Harnez Claude status line does not show basic context numbers. Users should be
able to see how much context is in use while working.

## /goal

Show useful, current context usage numbers in the Claude status line, using the
available status-line data. The numbers update as context usage changes and fit
the existing status-line layout.

