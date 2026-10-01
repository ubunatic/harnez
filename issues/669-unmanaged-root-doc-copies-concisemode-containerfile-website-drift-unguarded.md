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

Preflight (host, 2026-10-01): delete is ruled out. All three root copies are linked:
`docs/commands/mode.md` and the studies/bench data (`@docs/ConciseMode.md`),
`docs/EnvironmentSetup.md` (`docs/Containerfile.md`), `docs/commands/website.md` and
`docs/commands/publish.md` (`@docs/Website.md`). Decision: add all three to this repo's init doc set.

### M1 (manage the three copies via init)
- Find how this repo selects which copyable docs `harnez init -d .` installs (config.yaml
  `default:` / auto-detection, a project-level docs list, or `--docs`), and add `concise-mode`,
  the Containerfile entry and the Website entry so a plain `harnez init -d .` writes them.
  Use the config.yaml keys (check them), not file names.
- Run `make install`, then `harnez init -d .`. The three copies gain a `harnez:stop` marker and the
  missing `harnez:bundled` lines. `git checkout --` any unrelated file init rewrites.
- AGENTS.md's harnez-managed Language Conventions block may gain three entries; keep them.
- Acceptance: `TestRootDocCopiesMatchSources` now covers all three (prove it: temporarily edit one
  copy, see the test fail, revert). Full suite green via `HTO=0 make test-q1 > <scratch>/q1.log 2>&1`,
  then grep for `--- FAIL`.
- Update `docs/LanguagePipeline.md` § "Lite variants and root copies": drop the sentence about
  skipped copies (or say no root copy is currently skipped).
- Close: `harnez issues close -d . 669 "ConciseMode, Containerfile and Website root copies now installed by init and covered by the drift test"`.
