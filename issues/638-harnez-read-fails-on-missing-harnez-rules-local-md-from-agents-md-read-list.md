# 638 — harnez read reports optional .harnez/rules/Local.md as an error when missing

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: #625 (multi-file read skips missing files), internal/mode/mode.go (Local.md is git-excluded)

---

## 1. Problem & Motivation
Generated AGENTS.md tells agents to run `harnez read ... .harnez/rules/Local.md` before any work.
`Local.md` is per-machine and git-excluded, so it is absent in fresh clones and new projects.
harnez 0.1.18 skips it correctly (exit 0) but prints
`harnez read: .harnez/rules/Local.md: open ...: no such file or directory` on stderr.
Agents read that as a failure: in the `settings` project session (2026-09-29) the agent reported
it as a discovery problem and created a placeholder Local.md to silence it.

## 2. Technical Specification / Findings
- Repro: in a project without Local.md, run the AGENTS.md read line; exit is 0, stderr has the error.
- Local.md is optional by design; other missing rule files are real problems and should stay loud.

## 3. Implementation & Verification Plan
- In `harnez read`, treat a missing `.harnez/rules/Local.md` as optional: print
  `=== .harnez/rules/Local.md (absent, optional) ===` on stdout, nothing on stderr.
- Alternative: drop Local.md from the generated read line when the file does not exist at init time
  (weaker: goes stale when the file is created later).
- Test: multi-file read with missing Local.md → exit 0, empty stderr; missing Tools.md → stderr error kept.
