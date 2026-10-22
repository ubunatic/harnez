# 860 — init adds Makefile help scaffold and doc copies to foreign repos without repo.yaml opt-in

**Status**: Open
**Priority**: P2 (Medium) — init edits a file the repo owns
**Severity**: Minor
**Category**: Bug
**Related**: 854 (repo.yaml, audit), 853

---

/goal In a repo whose Makefile and docs/ were not set up by harnez, `harnez init` neither rewrites the Makefile
(`.PHONY` emoji line, color vars, `help` target) nor copies docs, unless `.harnez/repo.yaml` enables it
(e.g. `make.scaffold`, `docs.copies`). Verified in a scratch repo, or stop and report on a design choice.

## Findings (854 M2 host check, 2026-10-22)

Scratch repo with a plain `build:` Makefile: init added the `.PHONY`/`help` scaffold and created `docs/` with
the Make doc, because detection recorded `docs.copies: [make]` from the Makefile alone.

## Way forward

- Add a `make.scaffold` key (schema + detection: true only when the Makefile already has the harnez help
  target), gate the scaffold on it.
- Detection for `docs.copies` on first init of a foreign repo: record what is detected but copy only with
  opt-in, or keep current behavior and document it; decide with the user if unclear.
