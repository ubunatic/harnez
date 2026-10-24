# 866 — release: refuse while the breaking-changes table still has Unreleased rows

**Status**: Closed — Implemented configurable upgrade-table preflight refusal and manual relabel docs; make test-q1 passed with no failures (d5e3e492).
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

## Decision (owner delegated, 2026-10-24)

Default path `docs/Upgrading.md`, overridable by a `version.yaml` key; no doc means no check. Release refuses
(also in dry-run, reported) and names the rows; no auto-relabel.
