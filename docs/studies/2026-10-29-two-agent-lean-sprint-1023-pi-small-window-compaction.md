---
kind: sprint
date: 2026-10-29
issue: 1023
---

# Lean sprint 1023: Pi compaction overrides for small-window models

## Goal
Items 1 and 2 of issue 1023: stop Pi sessions on 32k local models (`lmcoder:coder`) from ending with a
one-token `length` reply, by deriving Pi compaction settings from the model's context window.

## Agents
- Searcher `haiku:low` (57 s, 435k tokens incl. cached): found the Pi launch path, that harnez passes no Pi
  settings, Pi 1.4.0's settings merge and `modelOverrides` key, and the spec home in `spec/agent.yaml`.
- Developer `opus:low`: plan turn (70 s, 232k) then M1 and M2 in one write turn (5 min 17 s, 1.66M). Claude
  models only, because OpenAI and Gemini quota was exhausted (see 1024).

## Decisions (host)
- Global Pi `settings.json` `compaction.modelOverrides["<provider>/<id>"]`, written by `harnez apply`, over a
  per-session `PI_CODING_AGENT_DIR` (loses auth, models and sessions) or a project `.pi/settings.json` (writes
  into the user's repo, ignored for untrusted projects).
- No state file: an entry is harnez's own only if it equals what it would write now.

## Milestones
- M1 `bb6b6386`: `internal/picompact`, spec block `pi_compaction` (threshold 65536, fractions 0.4 / 0.25).
- M2 `b5440c80`: start-time warning for small-window Pi models with no override.
- M3 (host, live): before apply 28,643-token prompt, 1 output token, `length`, 0 compactions; after apply
  2 compactions, peak 22,878, normal stop, answer in 95 s.

## Observations
- The developer's plan turn caught the key facts the searcher left open (override precedence, integer
  requirement, `lmcoder/coder` key), which made the write turn a single pass.
- The live before/after pair was cheap (under 3 min) and is the evidence the unit tests cannot give.
- Host slip: an issue path guessed from the title slug; look it up with `harnez find` first.
