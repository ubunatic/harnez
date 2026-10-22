# 866 — release: refuse while the breaking-changes table still has Unreleased rows

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: docs/GoRelease.md; loom docs/Upgrading.md

---

/goal `harnez release` (also `--dry-run`) stops before bumping when the project's upgrade doc still has
rows labelled `Unreleased`, naming the file and row count, and a documented way relabels them with the
new version; tests cover found/none/no-doc; or stop and report when blocked on a user decision.

## Finding (loom v0.4.0, 2026-10-22)

20 rows in loom `docs/Upgrading.md` still said `Unreleased`, most shipped in v0.2.22–v0.3.11. Nothing
checked them; the host relabelled them by hand from `git tag --contains` of each row's commit.

## Open questions

- Where the doc path comes from (version.yaml key, default `docs/Upgrading.md`).
- Refuse vs. relabel automatically to the new version.
