# 349 — Verify whether skillOverrides works on the installed Claude Code version

**Status**: Closed — skillOverrides live-verified functional on 2.1.273, including for bundled skills
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research

---

## Summary

Follow-up from issue 347's research pass. Claude Code documents a
`skillOverrides` settings.json field
(`{"skillOverrides": {"<skill-name>": "on"|"name-only"|"user-invocable-only"|"off"}}`)
that would let harnez selectively disable specific bundled skills instead of
the all-or-nothing `disableBundledSkills` boolean — but issue 347's research
found multiple GitHub issue reports (unverified against a live install)
claiming it's non-functional for user/project-scoped settings on current
versions, and at least one source claiming it's scoped to plain user/project
skills only and doesn't extend to bundled/plugin skills at all regardless.

This is the single highest-leverage open question from 347: if
`skillOverrides` actually works, it's strictly better than
`disableBundledSkills` and should be preferred once confirmed; if it's
broken or doesn't cover bundled skills, `disableBundledSkills` remains the
only lever and 347's "keep the blunt toggle" recommendation stands as-is.

## Task

Live-check against the actually-installed Claude Code version (not docs or
GitHub issue reports):

1. Add a `skillOverrides` entry to a test `settings.json` (or via
   `--settings` for an isolated one-off check, same technique used in issue
   316's live testing) targeting one specific bundled skill, e.g.
   `{"skillOverrides": {"dataviz": "off"}}`.
2. Run `claude -p "/context"` (or `/skills`) and confirm whether `dataviz`
   is actually absent from the Skills table / genuinely non-invocable, not
   just marked differently in a menu.
3. Try each documented mode (`"on"`, `"name-only"`, `"user-invocable-only"`,
   `"off"`) if the field has any effect at all, to characterize exactly what
   works and what doesn't on this installed version.
4. Note the exact Claude Code version tested, since this may change between
   releases (the GitHub issues 347 found suggest this is an active area of
   flux, not stable behavior).

## Definition of done

- [x] Documented, version-stamped finding: does `skillOverrides` work at all
      on the installed version, and if so, for which skill categories (plain
      user/project skills only, or bundled skills too)? — **Yes, on Claude
      Code 2.1.273, for both.** All four modes (`on`/`off`/`name-only`/
      `user-invocable-only`) produced distinct, correct behavior against
      `dataviz`, a bundled (`Built-in`) skill. See
      `docs/studies/2026-09-16-skilloverrides-live-verification.md`.
- [x] Updates issue 347's recommendation if this changes the answer — added
      a dated update note to 347; overall recommendation (keep
      `disableBundledSkills` as the catalogue-wide lever) is unchanged, but
      `skillOverrides` is now confirmed as a viable finer-grained option if
      ever wanted.
- [x] Recorded as a dated `docs/studies/` entry (separate file rather than
      an edit to the 2026-09-15 debloat study, since that file was being
      concurrently edited for issue 348 in the same session).

## Related

- Issue 316 — `harnez apply --debloat`.
- Issue 347 — disableBundledSkills scope/replacement research (source of
  this follow-up).
- Issue 348 — sibling follow-up (context-token measurement).
