# 348 — Measure disableBundledSkills real context savings in this repo

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Research

---

## Summary

Follow-up from issue 347's research pass. The `docs/studies/2026-09-15-debloat-context-usage-measurement-and-cli-flag-comparison.md`
study measured the `~/.claude` `Skills` category at ~3.1k tokens, but that
measurement used a clean, isolated test project directory — harnez's own
project skills (`sprint`, `lean-sprint`, `issue`, `review`, `commit`,
`tool-feedback-protocol`, etc.) were not loaded in that baseline. Issue 347's
research agent could only infer, not confirm, that most of the 3.1k is
bundled-skill weight.

## Task

Run a real `claude -p "/context"` A/B comparison **inside this actual harnez
repo** (not a clean test project), same methodology as issue 316:

- Baseline: `claude -p "/context"` from `/home/uwe/projects/harnez`, current
  settings (no `disableBundledSkills`).
- With toggle: `claude --settings '{"disableBundledSkills": true}' -p "/context"`
  from the same directory.
- Record the Skills category token delta, and confirm which skills survive
  in the "With toggle" run's Skills table (should be exactly harnez's own
  project skills, per issue 347 finding #1 — verify this holds, don't just
  assume).

## Definition of done

- Documented delta (append to the 2026-09-15 debloat study doc, or a new
  dated study entry) confirming the real, in-repo token savings from
  `disableBundledSkills: true`.
- Confirms or corrects issue 347's inference that the ~3.1k baseline is
  "plausibly almost entirely bundled-skill weight."
- Feeds into a go/no-go for adding `disableBundledSkills` as a `--debloat`
  toggle default (it already exists as an opt-in flag in
  `internal/claude/debloat.go`'s `DebloatOptions` — this ticket is about
  whether the measured benefit justifies promoting it, not about building
  the flag itself, which already exists).

## Related

- Issue 316 — `harnez apply --debloat` (the flag this measurement feeds a
  recommendation into).
- Issue 347 — disableBundledSkills scope/replacement research (source of
  this follow-up).
