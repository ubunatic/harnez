# 1080 — gokay check skips untracked files, so a repo cannot be checked before its first commit

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 

---

## 1. Problem & Motivation
`gokay check` skips untracked files with both `--base main` and `--all`, so a brand-new repo cannot be checked before its first commit.

## 2. Technical Specification / Findings
gokay (`~/go/bin/gokay`) is driven by the harnez `gokay` skill; the fix may belong in gokay itself. File list probably comes from `git ls-files`/diff only.

Reported by the taia session from horg worker runs (epic horg#6), 2026-11-01. Source log: codeberg harnez/horg docs/studies/2026-11-01-agent-performance-log.md, section "Tool Experience".

## 3. Implementation & Verification Plan
- Include untracked, non-ignored files (`git ls-files --others --exclude-standard`) for `--all`, and for `--base` when untracked files exist.
- Test: fresh `git init` repo with one Go file is checked.
