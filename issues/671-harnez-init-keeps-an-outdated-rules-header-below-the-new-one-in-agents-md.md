# 671 — harnez init keeps an outdated rules header below the new one in AGENTS.md

**Status**: Closed — Fixed: init replaces an older generated rules header; regression tests added; cati re-init has one header and is idempotent
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 567, 625, 629

---

## 1. Problem & Motivation
`harnez init -d .` in `../cati` (2026-10-01) printed "updated rules header" and left AGENTS.md
starting with two "Before any work" lines: the current one-call header (issue 625) above the
older "read `.harnez/rules/Index.md` if it exists, then `.harnez/rules/Local.md`" header
(issue 567). Agents then get two conflicting read instructions.

## 2. Technical Specification / Findings
`backfillRulesHeader` (`internal/claude/init.go`) only checks for the current header and
otherwise prepends it, so any older generated header stays.

Also checked: the current header names `Quota.md` and `Local.md`, which do not exist in every
project (psync, cati). That is fine: `harnez read` skips missing files since issue 625.

## 3. Implementation & Verification Plan
- If the first line is a generated header (bold, starts "Before any work, read", mentions
  `.harnez/rules/`), replace it instead of prepending; a user's own bold first line stays.
- Tests: `TestRunInit_ReplacesOutdatedRulesHeader`, `TestRunInit_KeepsUserBoldFirstLine`.
- Verify: re-run init in cati, AGENTS.md has one header; second run prints "No changes."
