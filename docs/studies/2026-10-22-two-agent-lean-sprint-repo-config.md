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
