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
- Review (agy:neon:low, 1m31s, 133k tokens incl. cached): 1 high (since-filter claim order), 2 medium (subagent-only claim, ResolveCommands swallowing errors), 2 low (missing tests). All checked against the diff and moved to M2 pre-work. The host review had missed the claim-order bug.
- Turn 3, M2 (3m14s, 3.94M tokens including cached): dda05d3f (`claude-opus-4-8` price), bbded485 (the claim fix, config errors propagated, the missing tests), e2c1c831 (docs).
  - The host checked the Opus 4.8 row on the official pricing page ($5 / $6.25 / $10 / $0.50 / $25 per MTok) and reran the touched packages: green.
- The host hand-checked d957448b against taia's `session-usage.sh`. Claude Code's `cost-state` gives $2.09. A recompute with harnez's rules gives $2.4709, which equals the report. The gap is Claude Code's counter: it records fewer cache-write tokens than the transcript.
- Live acceptance with the taia override:
  - 148 sessions, 19 of them from taia;
  - API $126.03 = ATTRIB 123.59 + UNATTRIB 2.44;
  - weekly pp not explained locally: 1 (was 14);
  - 5-hour pp not explained: 4 (was 203);
  - weekly-equivalent gap: 1.09 pp;
  - no unpriced calls left.
- Retro: the cheap review caught the claim-order bug that the host diff review missed. The live check caught the unpriced model that the tests could not show. The developer held the one-run budget in M2 but not in M1.
