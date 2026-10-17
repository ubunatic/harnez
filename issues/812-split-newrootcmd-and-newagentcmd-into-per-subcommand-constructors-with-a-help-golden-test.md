# 812 — Split newRootCmd and newAgentCmd into per-subcommand constructors with a help golden test

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: 170, 807

---

## 1. Problem & Motivation
`newRootCmd` (920 lines, main.go) and `newAgentCmd` (876 lines, agent.go) are the longest functions in the repo (`harnez assess --hotspots`). Supersedes the remaining scope of 170.

## 2. Technical Specification / Findings
Extract per-subcommand `newXCmd()` constructors into focused files; each owns its flag variables. Behaviour identical.

## 3. Implementation & Verification Plan
Golden test: capture `--help` for every command in the tree before the refactor and assert identical output after.
