# 743 — Cloud agent compatibility: tests and bootstrap in sandboxed sessions

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Tooling / Cloud Agents
**Related**: #744, #745, #746, #747, #748

---

## 1. Problem & Motivation
Cloud agent sessions (Claude Code on the web, Jules, Codex cloud) start from a fresh
Ubuntu container. There, `make test-q1` on a clean `main` fails in four packages, and
`harnez` is not on PATH although `CLAUDE.md` asks agents to run `harnez read` first.
Agents then cannot tell their own regressions from environment noise (seen while
reviewing GitHub PR #19).

## 2. Technical Specification / Findings
Observed on 2026-10-13: git 2.43.0, Go 1.26, PID 1 `process_api`, no `minisign`.

| Ticket | Package | Cause |
|--------|---------|-------|
| #744 | `internal/release` | `minisign` required on PATH even in dry-run tests |
| #745 | `internal/skillreg` | `git checkout --end-of-options` unsupported by git 2.43 |
| #746 | `cmd/harnez`, `internal/sound` | process-group kill probe sees surviving pid |
| #747 | `cmd/harnez` | agent foreground resume: session not found |
| #748 | bootstrap | no SessionStart hook installs `harnez` |

## 3. Implementation & Verification Plan
- Fix each sub-ticket without skipping tests: tests must not depend on host tools or
  init behaviour they do not set up.
- Done when `make test-q1` passes on a clean `main` in a fresh cloud session, and the
  session can run `harnez read` without manual setup.
