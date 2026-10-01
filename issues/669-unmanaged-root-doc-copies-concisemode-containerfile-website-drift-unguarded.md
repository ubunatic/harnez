# 669 — Unmanaged root doc copies (ConciseMode, Containerfile, Website) drift unguarded

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Docs / Templates
**Related**: 666, 667; [LanguagePipeline](../docs/LanguagePipeline.md#lite-variants-and-root-copies), [drift test](../internal/claude/root_docs_sync_test.go)

---

## 1. Problem & Motivation
`TestRootDocCopiesMatchSources` checks only root `docs/*.md` copies that carry a
`harnez:stop` marker, i.e. the ones `harnez init -d .` writes in this repo.
`docs/ConciseMode.md`, `docs/Containerfile.md` and `docs/Website.md` have no stop marker:
they are not in this repo's init doc set, so nothing updates them when their source changes.
Today they match their sources except for marker lines (two lack `<!-- harnez:bundled -->`),
but they will drift silently.

## 2. Technical Specification / Findings
If one of them is later added to the init doc set, `prepareManagedDoc` replaces the markerless
copy with a warning ("replacing differing markerless doc"), so local edits there would be lost.

## 3. Implementation & Verification Plan
Pick one per file: add it to this repo's init doc set (init then manages it and the drift test
covers it), or delete the root copy if nothing links to it. Verify with
`TestRootDocCopiesMatchSources` and a `grep` for links to the deleted files.
