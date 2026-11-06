---
kind: topic
date: 2026-11-06
tickets: [230, 350, 518, 619, 763, 914, 960, 1007, 1094]
---
# Deinvest candidates from the Jules backlog cleanup (for owner confirmation)

## Context

The 2026-11-06 Jules batch audits ([cleanup study](2026-11-06-jules-backlog-cleanup.md)) rated these
backlog tickets priority `drop`. Current focus is taia and taia-plugins; harnez should stay a
companion tool and Go library for them. Proposal: close each ticket below as "deinvested", unless
the owner keeps it. Nothing is closed until the owner replies in this PR.

## Outcome (proposal)

| Ticket | Prio | Title | Why deinvest | Proposal |
| --- | --- | --- | --- | --- |
| 230 | P2 | Android direct-release scaffold | App release work for one sibling project; not harnez core, not taia | close |
| 350 | P3 | `harnez memory` for Claude auto-memory | Its own study says "candidate, not implementing now"; Claude-only | close |
| 518 | P3 | Measure gpt-6-sol plan-quota cost | One-off measurement; model data moves to the catalog (942, 1177) | close |
| 619 | P3 | Explore Phantom Looper concepts | Open-ended research, no consumer | close |
| 763 | P2 | DeepSeek Harness vs Pi video study | Partly covered by the 2026-09-10 plugin-systems study | close |
| 914 | P3 | Study Harness2 | Open-ended research, no consumer | close |
| 960 | P2 | research skill: iterative web fanout | Skill idea without a user; can return as a taia skill | close or move to taia |
| 1007 | P2 | Shared .agents skills discovery | taia#126 (closed) already supports the `.agents/skills` layout | close (covered by taia) |
| 1094 | P2 | Research AutoSwarm (HKUDS) | Open-ended research, no consumer | close |

## What Went Wrong

None for this list. The `drop` ratings come from Jules batch reports, some of which were discarded
for wrong references; the reasons above were rechecked against each ticket by the host.

## Learnings

- Research tickets without a consumer pile up: 7 of 9 candidates are "study / explore" tickets.
- Check the taia tracker before keeping cross-agent features in harnez (1007 vs taia#126).

## Follow-Up Tickets

- On confirmation: `harnez issues close -d . <n> --no-code "Deinvested 2026-11-06: <reason>"` per kept
  row (no code was needed; the decision is the outcome).
- None filed yet.
