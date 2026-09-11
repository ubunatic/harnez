# 315 — init drops previously opted-in docs on re-run; Canary.md hard-references opt-in PrototypingFeatures.md

**Status**: Open
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Bug

---

## Summary

Found by dogfooding `harnez init` on the harnez repo itself (2026-09-11). A plain
`harnez init` (no `--docs` flags) silently un-installed a previously opted-in doc,
leaving a bundled default-on doc referencing a file that no longer existed in the
project.

## What happened

1. This repo had previously run `init --docs prototyping-features` at some point,
   giving it `docs/PrototypingFeatures.md` and the line
   `Feature prototyping @docs/PrototypingFeatures.md` in AGENTS.md's Language
   Conventions block.
2. Running plain `harnez init` again regenerated the managed AGENTS.md section from
   `config.yaml` bundled defaults only. `prototyping-features` has `default: false`
   in `config.yaml`, so it was dropped — both the AGENTS.md line and (apparently)
   the file were not reconciled, and `docs/PrototypingFeatures.md` was left
   orphaned/stale relative to what AGENTS.md now advertised.
3. Meanwhile `docs/other/Canary.md` (source for `docs/Canary.md`, `default: true`,
   always bundled) contains a hardcoded cross-reference:
   ```
   For the broader feature-prototyping framing and its limits, see
   `@docs/PrototypingFeatures.md`.
   ```
   `config.yaml`'s `prototyping-features` entry has `depends_on: [canary]`, which
   only guarantees Canary is present *before* prototyping-features can be offered —
   it does not guarantee the reverse (that Canary's body is safe to reference
   prototyping-features unconditionally). After step 2, this reference pointed at a
   file the project no longer had installed via the current bundle selection.

## Root causes

- `harnez init` (no `--docs` flag) does not preserve previously-selected opt-in
  (`default: false`) docs across a re-run — it only reconciles the bundled-default
  set, so re-running plain `init` on a project that has opted into extra docs can
  quietly regress the project's doc set.
- `docs/other/Canary.md`'s bundled body hard-references an opt-in doc
  (`PrototypingFeatures.md`) by a project-relative `@docs/` path, with no guarantee
  that path resolves for a given project/init run.

## Reproduction

In the harnez repo itself:
```
git diff AGENTS.md   # after a plain `harnez init`, Language Conventions loses the
                      # "Feature prototyping @docs/PrototypingFeatures.md" line
ls docs/PrototypingFeatures.md   # missing
grep -n PrototypingFeatures docs/Canary.md   # still references it
```
Fixed locally for this repo by re-running `harnez init --docs prototyping-features`.

## Recurrences

Reproduced live a second and third time in the same week, on completely
unrelated trigger actions:

- 2026-09-11, onboarding `docs/lang/ManPages.md` (adding a new auto-detected
  lang doc) — same drop.
- 2026-09-11, onboarding a one-line `timeout` convention addition to
  `docs/lang/Bash.md` (issue 319) — running `harnez init` purely to verify
  the doc propagated dropped `prototyping-features` again, on a session with
  no relation to the doc-selection logic at all.

Three independent hits from two unrelated actions indicate this fires on
*any* plain `harnez init` re-run in a project that has ever opted into a
`default: false` doc — not an edge case. See
`docs/studies/2026-09-11-first-live-codex-advisor-design-gate-and-a-third-init-drop-recurrence.md`
for the full write-up. Worth revisiting priority (currently P2/Moderate)
given how easily and repeatedly this actually fires in practice.

## Suggested fix

- `init` should detect docs already present/previously selected in a project
  (e.g. by scanning the existing managed AGENTS.md block or an installed-docs
  manifest) and keep them installed on a bare re-run, rather than silently
  reverting to `config.yaml` bundled defaults only.
- Alternatively/additionally: don't let a `default: true` doc's body hard-reference
  a `default: false` doc by a project-local path — either inline the minimal framing
  in Canary.md and drop the cross-link, or make the cross-link conditional /
  point at the source path only in docs about harnez's own docs pipeline.

## Verification

- [ ] `harnez init` run twice in a row on a project that has an opt-in doc
      installed does not remove that doc or its AGENTS.md entry.
- [ ] No bundled (`default: true`) doc contains an `@docs/` reference to a doc
      that isn't guaranteed installed alongside it.
