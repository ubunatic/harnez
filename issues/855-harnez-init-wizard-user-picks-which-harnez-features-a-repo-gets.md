# 855 — harnez init wizard: user picks which harnez features a repo gets

**Status**: Closed — M1 9b98f6af, M2 5c2df12a, M3 9e60aaeb: TTY wizard with --wizard, -y/no-TTY unchanged; opt-outs survive later -y init (TestRunInitWizardTrackerOptOutSurvivesAssumeYes reproduced the blocker first); merge driver follows tracker; host reran internal/claude and cmd/harnez green after the dev's post-run test fixes; host live pty run passed
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

- Depends on 854: the wizard edits the settled `.harnez/repo.yaml` config; 854 is closed.
- Defaults come from detection; the wizard explains each feature in one line.
- Must not block agents: without a TTY, or with `-y`, it uses detection plus existing config.
- M1/M2 delivered in `9b98f6af` and `5c2df12a`; M3 fixes tracker opt-out adoption, ties
  merge-driver selection to the tracker, and clarifies empty optional-doc choices.

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

M3 implementation complete: adoption now occurs only on first init without
`.harnez/repo.yaml`; tracker opt-outs persist across later `-y` runs. The merge
driver follows the tracker choice, empty optional-doc selection is explained,
and the CLI design docs cover comment loss and first-init adoption. The required
M3 quota run exposed three stale assertions; they were updated after the one
allowed full run, so the final test edits were not rerun. The installed CLI smoke
passed for opt-out persistence, skipped merge-driver prompt, and empty docs notice.
