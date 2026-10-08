# 753 — Audit and document bash heredoc appends vs native edit tool guidelines for agents

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: docs/studies/2026-10-08-bash-heredoc-appends-vs-native-edit-tools.md

---

## 1. Problem & Motivation

Agents frequently deliberate between using native structured edit tools (`replace_file_content`, `write_to_file`) and shell-based here-docs (`cat << 'EOF' >> ...`).
For discrete end-of-file appends (e.g., ticket addenda, temporary prompt files), native edit tools introduce heavy JSON schema and anchor-reading token overhead (~4x higher), whereas shell tools are lean and chainable. Conversely, for code changes in the middle of files, shell text manipulation (`sed`) is brittle and unsafe compared to structured edit tools.

There is currently no explicit standard or cross-provider audit determining whether Claude, Codex, or AGY agents naturally follow this distinction, nor is there a concise guide instructing them when to choose each.

## 2. Technical Specification / Findings

- Study document initialized: [`docs/studies/2026-10-08-bash-heredoc-appends-vs-native-edit-tools.md`](file:///home/uwe/projects/harnez/docs/studies/2026-10-08-bash-heredoc-appends-vs-native-edit-tools.md).
- Target areas:
  1. Audit current provider behavior across Claude, Codex, and AGY.
  2. Add concise guidelines into copyable docs (`docs/practices/` or `docs/lang/Bash.md`).

## 3. Implementation & Verification Plan

### Goal
/goal Audit provider agent behaviors regarding bash heredoc vs edit tools, and integrate a concise guidance section into agent documentation, or stop and report when blocked on user input or denied permission.

### Milestones
- **M1: Cross-Provider Audit**:
  - Sample agent transcripts (Claude, Codex, AGY) to inspect tool selection patterns for file appends and temporary file generation.
- **M2: Guideline Documentation**:
  - Draft and commit a short guideline section in `docs/lang/Bash.md` (and related practice docs) advising agents when to use bash heredoc appends vs native edit tools.
- **M3: Sync & Close**:
  - Run doc copy sync (`harnez init -d .`), verify tests, and close ticket.
