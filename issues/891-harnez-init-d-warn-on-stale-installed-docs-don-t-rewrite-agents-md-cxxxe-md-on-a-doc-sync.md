# 891 — harnez init -d .: warn on stale installed docs, don't rewrite AGENTS.md/CxxxE.md on a doc sync

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 751 (dirty-write warnings), docs/CLIDesign.md, TestRootDocCopiesMatchSources

---

/goal Syncing root doc copies in this repo with `harnez init -d .` uses the current doc sources (or
says clearly that it doesn't), and touches only the doc copies it is asked to sync; verified with tests;
or stop and report when a CLI-design decision (`init` vs `apply` scope) is needed.

## Problem (session 2026-10-24)

1. **Stale embedded docs.** `harnez init` copies the docs embedded in the *installed* binary. After editing
   `docs/practices/AgenticLoop.lite.md`, `harnez init -d .` left `docs/AgenticLoop.md` stale and
   `TestRootDocCopiesMatchSources` failed, with no warning from `init`. It worked only after `make install`.
2. **Unrelated rewrites.** Every `harnez init -d .` run during a doc sync also rewrote `AGENTS.md` (9 lines
   removed) and the agent-specific instruction file, which then had to be restored by hand with `git checkout --`
   (twice in one session). The repo instructions currently document that workaround.

## Wanted

- `init` detects that the embedded doc sources are older than or differ from the repo's own sources (when run
  inside the harnez repo) and warns or refuses, naming `make install`; or reads the repo sources directly.
- A doc-only sync (a flag or the default for `-d .` with `--docs`) leaves `AGENTS.md` and the instruction
  files alone, or `init` explains why it changes them.
- Read `docs/CLIDesign.md` first: `init` and `apply` stay separate.
