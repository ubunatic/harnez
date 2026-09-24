# 549 — Keep agent Bash calls short and manage prompt files

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #548

---

## 1. Problem & Motivation

Long `harnez agent` Bash invocations repeat large prompts in chat and are difficult to review. Redirecting output to manually named temporary files adds noise; `harnez agent` should capture session output automatically. Agents also need a clear convention for creating and managing prompt files.

## 2. Goal

Keep agent Bash calls concise by using named prompt files with a documented lifecycle convention, and have `harnez agent` capture session output automatically without requiring manual temporary-file redirection.

## 3. Implementation & Verification Plan

Define and document the prompt-file convention, provide automatic output capture for `harnez agent` sessions, and verify agents can resume with a file-based prompt without embedding long text or constructing output-capture paths in the command.
