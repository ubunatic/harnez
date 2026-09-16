# 381 — init silently discards local edits in bundled docs without stop marker

**Status**: Closed — resolved in 55214d8
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: [060 — managed-doc stop-marker convention](archive/060-triage-sibling-managed-docs-drift.md), [233 — guidance on editing copied docs](233-agents-md-note-verify-doc-edits-target-the-right-file-repo-local-vs-docs-practices-lang-other-source.md)

---

## 1. Problem & Motivation

In the sibling `loom` repository, `docs/Spec.md` was a harnez-bundled document
containing a 35-line, Loom-specific “Widget Boundary” section. The file had
`<!-- harnez:bundled -->` but no `<!-- harnez:stop -->`. Running
`harnez init --variant lite --quota-1` copied `docs/other/Spec.md` over the
installed file and removed that section without a warning. Repeating from a
clean checkout with `harnez init --variant full --quota-1` removed the same
section. The loss is caused by recopying an unprotected bundled document, not
by the lite variant: Spec has no lite source and falls back to full.

The command reported `copied docs/other/Spec.md → ./docs/Spec.md`, but did not
signal that project-specific content was being discarded. Git could recover
this particular tracked edit; the same behavior can destroy uncommitted edits.
The default installed Spec document itself has no stop marker, so a user can
reasonably add a local section without realizing the next init will erase it.

`internal/markdown/markdown.go` documents the current behavior in
`MergeManagedDoc`: when the destination has no stop marker, it returns the
bundled content as-is. Issue 060 introduced the stop-marker convention; this
case exposes the unguarded path when the marker was never added.

## 2. Desired Behavior

Reconciliation should make destructive replacement of an existing bundled
document visible and prevent silent loss of local content. Decide how to
distinguish an outdated managed copy from a local customization, and provide
a safe path to preserve project-owned text after `<!-- harnez:stop -->` or in
a project-owned file. Do not silently infer that all content in a markerless
installed document is disposable.

## 3. Acceptance Criteria & Verification

- [ ] Reproduce with a markerless installed `docs/Spec.md` containing a local
      section and both `--variant lite` and `--variant full`.
- [ ] `harnez init` does not silently discard that section. It preserves the
      content or clearly reports and gates the replacement before writing.
- [ ] Existing stop-marker tails remain byte-for-byte preserved.
- [ ] Ordinary updates to unmodified bundled docs remain non-interactive.
- [ ] Tests cover the markerless local-edit case and the normal update path.
