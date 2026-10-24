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

## Milestone M3 — Pre-Work / Required Refinements (opus review of 9b98f6af, 5c2df12a)

M1 (scripted feature prompts + config write) and M2 (TTY detection, `--wizard`) delivered; host live pty run
passed (invalid answer re-asked, choices in `.harnez/repo.yaml`). Fix first:

1. Blocker, `internal/claude/init.go:1124`: tracker adoption re-enables a tracker the user turned off
   (`issues.tracker: none` + existing `issues/README.md` → a later `-y`/no-TTY init turns tracker and merge
   driver back on). Adopt only when nothing is recorded (first init, `repo.yaml` absent). Test: wizard
   disables tracker, then `-y` init, tracker still `none`. Update `docs/CLIDesign.md:196`.
2. `internal/claude/init_wizard.go:86-91`: merge driver can stay on with the tracker off; force it off and skip
   its prompt when the tracker is off; default it to the tracker answer when on.
3. `init_wizard.go:92-99`: "yes" to optional doc copies with nothing recorded/detected stores an empty list;
   say so in the output.
4. Nit: one line in `docs/CLIDesign.md` that the wizard rewrites `repo.yaml` without its comments.
5. Update this ticket's Notes (wizard now built).
