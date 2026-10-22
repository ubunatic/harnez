# 860 — Makefile scaffold and Make doc stay the default, with an opt-out in repo.yaml

**Status**: Open
**Priority**: P3 (Low) — current behavior is the intended default; only the opt-out is missing
**Severity**: Minor
**Category**: Feature
**Related**: 854 (repo.yaml, audit), 853

---

/goal `harnez init` keeps adding the harnez Makefile scaffold (`.PHONY` emoji line, color vars, `help`
target) and copying the Make doc by default, and a repo can opt out via `.harnez/repo.yaml`
(`make.scaffold: false`, or dropping `make` from `docs.copies`), which init then respects. Verified in a
scratch repo, or stop and report on a design choice.

## Decision (user, 2026-10-22)

The harnez way of writing Makefiles is the sane default; older Makefile styles are why people keep inventing
new task runners. So this is not a bug: like the file-first tracker, the scaffold is recommended and on by
default. Only add the opt-out, never fail without it.

## Findings (854 M2 host check, 2026-10-22)

Scratch repo with a plain `build:` Makefile: init added the `.PHONY`/`help` scaffold and created `docs/` with
the Make doc (detection recorded `docs.copies: [make]`). Both stay.

## Way forward

- Add `make.scaffold` (schema; detection default true when a Makefile exists), gate the scaffold on it.
- Confirm init respects `docs.copies` without `make` on later runs.
