---
kind: sprint
date: 2026-11-27
tickets: [1309]
---

# Lean sprint 1309: usage reads Claude transcripts from extra config dirs (taia)

## Setup
- The owner picked option A: a spec list of Claude config dirs, with taia's dir only in the user override. Hooks in confined taia sessions were assessed and dropped, because the transcripts on the host hold everything the report needs.
- Developer `dev-1309`, hydrogen:med. This was the owner's pick while the Claude weekly quota stood at 95%. The final review is one agy:neon:low pass.
- No Searcher: the ticket cites the code locations, and the developer's first turn is read-only.

## Log
- Filing: 1309 (c12f6010), the assessment (fc4cfc7a), the milestones (2861b557).
- Turn 1 (read-only plan): dispatched.
- Turn 1, the read-only plan (1m46s, 371k tokens including cached), was accepted. Its premise fixes are recorded in the ticket's Decision section.
- Turn 2, M1 (4m37s, 1.97M tokens including cached): eecc3519 and 9874666a, 19 files, +588/-37.
  - The developer ran targeted tests before `make test-q1`, which broke the one-run budget.
  - The host diff review found no blocker. The touched packages are green.
- The live check with the taia override works:
  - the weekly pp not explained locally fell from 14 to 1;
  - the 5-hour pp not explained fell from 203 to 6;
  - the weekly-equivalent gap fell from 2.36 to 0.86 pp.
  It also found two follow-ups, now M2 pre-work: 2 `claude-opus-4-8` calls that have no price, and the d957448b cost gap. Green tests could not show either one.
