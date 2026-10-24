# 855 — harnez init wizard: user picks which harnez features a repo gets

**Status**: Open
**Priority**: P3 (Low) — convenience on top of 854's config and detection
**Severity**: Minor
**Category**: Feature
**Related**: 854 (per-repo config and detection), 853

---

/goal An interactive `harnez init` mode shows the detected setup and lets the user turn each harnez feature on
or off (issue tracker, studies index, merge driver, rules, doc copies, Makefile, quota rules), then writes the
choices to the per-repo `.harnez/` config; non-interactive init keeps working from config and flags. Stop and
report if 854's config design is not settled yet.

## Notes

- Depends on 854: the wizard only edits the config that 854 defines. 854 is closed
  (`.harnez/repo.yaml` exists); the wizard itself is not built yet (status check 2026-10-24).
- Defaults come from detection; the wizard explains each feature in one line.
- Must not block agents: without a TTY, or with `-y`, it uses detection plus existing config.
