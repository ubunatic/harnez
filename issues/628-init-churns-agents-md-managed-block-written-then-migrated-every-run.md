# 628 — init churns AGENTS.md: managed block written then migrated every run

**Status**: Closed — fixed; second init run prints No changes
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 567, 625

---

## 1. Problem & Motivation
Dogfooding `harnez init` in this repo on a clean tree prints, on every run:

```
  exists  AGENTS.md (unchanged)
  wrote AGENTS.md
    Harnez Managed Conventions: added
  wrote   .harnez/rules/Tools.md
3 change(s).
```

`git status` stays clean afterwards. The output is false: nothing changed, init is not
idempotent in its reporting, and the user cannot tell real drift from noise.

A second finding: `.harnez/rules/Tools.md` repeats its own header bullets (install hint,
`apply_patch`, `harnez find`) inside the appended "Harnez Managed Conventions" block, so every
rule-read pays for the same text twice.

## 2. Technical Specification / Findings
- `internal/claude/init.go` ~1143: the `agents_md.local` section loop still calls
  `applySectionMD(agentsPath, "Harnez Managed Conventions", ...)`, which re-adds the block to
  AGENTS.md (`changes++`, "added").
- `migrateInitRules` (init.go:652) then moves that block to `.harnez/rules/Tools.md`
  (`changes++`), and `writeRules` counts Tools.md as written (`changes++`). Net diff is zero.
- Since issue 567 M4 the block belongs in Tools.md; the AGENTS.md write is a leftover.

## 3. Implementation & Verification Plan
- Skip sections that `migrateInitRules` routes to `.harnez/rules/` when writing AGENTS.md, or
  write them straight to their rule file; count a change only when bytes on disk differ.
- Drop the duplicated header bullets in the Tools.md template (keep the managed block).
- Test: run init twice on a temp project; the second run must print `No changes.`.
