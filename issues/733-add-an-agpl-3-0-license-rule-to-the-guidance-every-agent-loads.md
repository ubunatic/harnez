# 733 — Add an AGPL-3.0 license rule to the guidance every agent loads

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation
**Related**: [#329](329-gate-harnez-release-on-reuse-compliance-with-no-reuse-and-a-global-opt-out.md) (REUSE gate on release), [docs/GoRelease.md](../docs/GoRelease.md) (license and REUSE mentions), [docs/ExternalSkills.md](../docs/ExternalSkills.md) (third-party skill licenses), harnez.org issue 028 M2 (skill export labels first-party skills `AGPL-3.0-or-later`)

---

## 1. Problem & Motivation

Owner rule (2026-10-06): "We always use AGPL-3.0 where possible."

No agent guidance says so. Agents therefore guess or leave the license out: harnez itself has no
license file, while loom has `LICENSES/AGPL-3.0-or-later.txt`; the harnez.org skill export (028 M2.1)
labels 27 first-party skills `AGPL-3.0-or-later` with no file in the repo to back it. The rule must reach
every agent in every harnez-initialized repo, not only Claude, so it belongs in the harnez-distributed
docs, not in a per-agent instruction file.

## 2. Technical Specification / Findings

- Distribution: `config.yaml` `docs_profiles` decides which docs each repo bundles; `core` (agentic-loop,
  issue-tracking) is the smallest set and `dev`/`full` add more. A rule "every agent sees" must land in
  something all profiles render, e.g. a one-line entry in the bundled AGENTS.md block or a short doc in
  `core`. Choose the smallest place that every profile includes; do not grow `core` with a long document.
- Rule content to state: new first-party projects and files use `AGPL-3.0-or-later`, with the license
  text in `LICENSES/AGPL-3.0-or-later.txt` (REUSE layout, as in loom). "Where possible" means: keep the
  upstream license of vendored or bundled third-party code (for example the MIT skills under
  `third_party/skills/`), and ask the owner when a dependency's license conflicts.
- Apply the rule to harnez itself: add `LICENSES/AGPL-3.0-or-later.txt`, so the `license:` fields in
  `config.yaml` and the harnez.org export are backed by a file. Check the other owner repos under
  `~/projects` for missing license files and list them; fix only with owner consent.
- Do not duplicate #329: that ticket gates releases on REUSE compliance; this one states the default
  license.

## 3. Implementation & Verification Plan

/goal Every agent in a harnez-initialized repo is told to use AGPL-3.0-or-later by default, and harnez
carries its license file; or stop and report when blocked on an owner decision or denied permission.

Verify with `harnez init` (or the doc render) in a throwaway repo per profile (`core`, `dev`, `full`):
the rule appears in each rendered AGENTS.md. `make check` stays green.
