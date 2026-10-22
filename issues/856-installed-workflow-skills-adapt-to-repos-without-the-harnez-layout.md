# 856 — Installed workflow skills adapt to repos without the harnez layout

**Status**: Open
**Priority**: P2 (Medium) — skills are installed globally and run in every repo
**Severity**: Minor
**Category**: Bug
**Related**: 853 (tracker opt-in), 854 (generic init, per-repo config), 568 (generic copyable docs)

---

/goal Every skill harnez installs works in a repo that is not built like harnez: repo-agnostic harnez
commands stay, but steps that need the harnez tracker, `docs/studies/`, generated indexes or `make test-q1`
first check that the repo has them (or that `.harnez/` config enables them, once 854 lands) and otherwise use
the repo's own conventions or keep the notes in the session. Verified by reading each skill against a scratch
repo without `issues/` or `docs/`, or stop and report when blocked on 854's config design.

## Findings (research pass 2026-10-22, re-verify before use)

| Skill source | Unconditional assumption |
|---|---|
| `docs/commands/issue.md:17-30` | harnez tracker workflow first; local-tracker fallback only at line 91 |
| `docs/commands/lean-sprint.md:24-25,63-66,121-122` | single `issues/` ticket, `harnez issues close`, index refresh (study notes already "when present") |
| `docs/commands/lean-sprint-v1.md:31,58,103` | same, no fallback |
| `docs/commands/reverse-sprint.md:57,88` | starts from `issues/`, ends with index sync |
| `docs/commands/sprint.md:65,90-91` | ticket and `issues/README.md` sync |
| `docs/commands/roadmap.md:7,32-33,87-89` | backlog = harnez active issues; points at this repo's tickets as references |
| `docs/commands/story.md:11-13,55` | always writes `docs/studies/` and edits a docs index |
| `docs/commands/evergreen.md:33-39` | edits issue and docs index files |
| `.harnez/rules/Quota.md:6` | `make test-q1` without checking the target exists |

## Way forward

- Skills keep recommending the harnez tracker (core concept, see 854 scope decision); they must only not
  fail without it.
- Rely on 854's safety notice in the always-loaded rules instead of per-skill detection prose; skills only
  need short "when the repo uses the harnez tracker" guards where a step would otherwise fail.
- Tracker steps run only for a harnez tracker (853's opt-in); else follow the repo's tracker or keep the
  handoff in the session and say so.
- Quota rule: use `make test-q1` when the target exists, else `harnez exec --quota-1 <repo test command>`.
- Remove or label the harnez-repo pointers in `roadmap.md`.

## Sprint 2026-10-22 (after 853/854)

Now available: `.harnez/repo.yaml` (`issues.tracker`, `docs.studies`, `rules.quota`) and notices in
`.harnez/rules/Index.md` when tracker/studies are off; a first `harnez issues new` adopts the tracker. Since
854 M2 gates the quota rule on `rules.quota`, check whether `Quota.md` is still installed without a
`test-q1` target before changing it. Guards should name repo.yaml/the notice, one short clause per step, no
duplicated detection logic. Edit skill sources in `docs/commands/`, then `make install && harnez apply`.

## Implementation (2026-10-22)

- Added short `repo.yaml` / `Index.md` guards to the issue, sprint, roadmap, story, and evergreen skills; kept the Harnez tracker workflow recommended and labeled roadmap references as Harnez repo examples.
- Updated both embedded Quota rule copies in `config.yaml` and synced `.harnez/rules/Quota.md`; sprint skills now use `make test-q1` when available and otherwise `harnez exec --quota-1 -- <repo-test-command>`.
- `make test-q1` passed; full output is in `/tmp/harnez-856-test-q1-final.log`, and `rg -- '--- FAIL'` found no matches. `make install && harnez apply` completed; installed `~/.claude/skills/issue/SKILL.md` contains the tracker guard. `harnez init -d .` synced Quota.md; rerun after commit to confirm a clean tree.
