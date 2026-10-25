# 957 — Docs refresh for the DecompileD talk: README, docs/, skills and commands current

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: ubunatic.com epic 113 (DecompileD talk), harnez.org 035 (site lists skills), 374 (last README refresh)

---

/goal harnez docs match the code today: README covers the current CLI commands and skills, `docs/README.md`
indexes every evergreen doc, stale docs (commands, flags, models that no longer exist) are fixed or archived,
and every skill in `docs/commands/` has a correct one-line description that harnez.org (035) can reuse. Checked
against `harnez --help` output and the installed skills. Or stop and report when a doc describes behaviour that
needs an owner decision. Circuit breaker: on a loop or no progress, clear the goal and write a failure report
here.

## Notes

- Edit copyable docs at their source (`docs/lang/`, `docs/practices/`, `docs/other/`), then sync root copies
  (`harnez init -d .`); see AGENTS.md "Where Repo Rules Go".
- Split into slices if large: README, evergreen index, skill descriptions, stale-doc sweep.
