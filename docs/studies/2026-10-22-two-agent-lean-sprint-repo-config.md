# Two-agent lean sprint: per-repo config (854)

Date: 2026-10-22. Host: Claude Opus. Developer: `dev-854` (sol:low).

## Discovery

Searcher skipped: three read-only research agents (luna:med: skills, code, prose) had audited the init
surface in the same session; their findings are in 853, 854, 856.

## Milestones

| Milestone | Commit | Review outcome |
|---|---|---|
| M1 (repo.yaml schema, loader, detect-once) | `85f6732a` | Two detections switched features on for every repo (any Makefile, `Quota.md` that init always writes) → pre-work |
| M2 (gating + notice) | `b2e5e5ad` | Host scratch check passed; found merge driver still installed (853) and Makefile scaffold/doc copies (860) |
| M3 (docs) | `4b5a5121` | Variadic `writeRules` param replaced; CLIDesign, Spec updated |

## Learnings

- Detection rules must look for the harnez feature itself (a target, a marker), not a file that init creates
  or that every repo has; otherwise "detect once" records today's over-reach as the repo's choice.
- The host scratch run found two divergences the developer's own scratch check missed, because it used a
  foreign `issues/README.md` and a plain Makefile rather than an empty repo.

## Follow-up sprint: 853 (tracker gating), dev-853 (luna:high)

| Step | Commit | Review outcome |
|---|---|---|
| Gating by `issues.*`, index no-op note | `08f92875` | Host scratch: foreign README untouched, no driver; but a fresh repo could no longer adopt the tracker |
| Adoption on first `issues new` | `b14ac743` | Host fresh-repo flow: repo.yaml switches to harnez, next init installs driver, notice removed |

## Follow-up sprint: 856 (repo-aware skills), dev-856 (luna:high)

Plan turn gave per-file guard wording; approved with one rule: guards name only the exception
(`issues.tracker: none`, `docs.studies: false`) and point to the Index.md notice, so the harnez tracker stays the
normal path. Delivered in `d36a4ba8` (8 skills, quota rule fallback to `harnez exec --quota-1`); installed
skills verified, tests green, self-init clean. No review rounds needed.

Learning: when gating a recommended default, test the adoption path (empty repo → first use), not only the
foreign-repo path.
