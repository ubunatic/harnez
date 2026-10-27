---
kind: sprint
---

# Two-agent lean sprint: 973, Haiku tiers and compaction spec defaults

Date: 2026-10-27. Host: Claude Opus 5.5. Developer: `codex:luna:med` (owner: no Haiku in this sprint), one session
`dev-973`, five turns. No Searcher: the ticket cited file:line locations found the same day; the plan-only first turn
re-verified them.

## Milestones

| Milestone | Commit | Result |
|---|---|---|
| M1 guide text | 51620455 | `haiku:high` no longer recommended; `luna:med` cheap default developer, `haiku:med`/`haiku:low` for easy/repetitive work; Luna "not Go development" line reworded |
| M2 accurate compaction claim | e863c408 | M1 had written "harnez compacts Haiku at 80k", true only in the owner's config |
| M3 spec defaults and switches | e9ea64de | `compact_thresholds` (Haiku 80000) and `auto_compact` (provider modes, watchdog grace, legacy growth threshold) moved from Go literals into `spec/agent.yaml` with schema |

M3 was added mid-sprint (owner: ship the Haiku limit as default), paused by the owner mid-turn, and resumed with an
extra requirement (all compaction switches and thresholds in spec). The paused uncommitted edits were the resume point.

## Host decisions

- `luna:med` as the default developer, from the owner's stated direction (recorded in the ticket's Decision section).
- Threshold lookup: user per-model > spec per-model > user global > spec global, so a user-global value cannot push
  Haiku past its 100k price step. Covered by a test.

## Verification

- Full `make test-q1`: 58 `ok`, 0 `--- FAIL` (M2 and M3); host reruns of `./cmd/harnez ./internal/subagent` ok.
- Empty-HOME probe: Haiku (bare and all tiers) 80000; Sonnet and Luna 200000.

## Friction

- The developer quoted `make[1]: Leaving directory` as the test summary in M1. Caught by the host; the M2 pre-work
  named the required form. The lean-sprint skill already requires quoting the final test summary line.
- A guide claim was taken from the owner's local config and written as a shipped fact (M1). Host diff review caught
  it; the general safeguard is issue 972 (verify before claiming).

## Efficiency

Developer turns: plan 45k new / 185k cached; M1 177k / 1.8M; M2 194k / 3.2M; M3 plan 233k / 4.0M; M3 313k / 12.5M
(Luna has no price step below 272k per request).
