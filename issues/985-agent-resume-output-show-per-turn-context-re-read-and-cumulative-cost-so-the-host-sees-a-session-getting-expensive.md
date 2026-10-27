# 985 — agent resume output: show per-turn context re-read and cumulative cost so the host sees a session getting expensive

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 983 (sprint where it was observed), 969 (per-model compaction limit, draft), 519 (stats --agents drain), 819 (pre-turn quota warning), 594 / 673 (compaction checks), `cmd/harnez/agent_stream.go:248` (`tokenSummary`)

---

/goal The output of `harnez agent start/resume` tells the host, at the end of each turn, what that turn cost and
when a long-lived session has become more expensive than a fresh one, with a concrete check result rather than
advice; or stop and report when blocked on an owner decision or a provider that does not report the needed numbers.
Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

Sprint 983 (2026-10-27) ran one developer session (`dev-983`, `codex:gpt-6.1-terra:med`) through 18 turns over
milestones M2–M8. The session hit the Codex usage limit in the M8 write turn, after the edits and before the
commit; the host had to finish the milestone. The developer's own work was lean; the cost came from the session's
lifetime, and the host could not see that from the tool output.

Done lines printed by `harnez agent resume` (selected):

| Turn | Done line (tokens) | Duration |
|---|---|---|
| M2 plan | 108.1k new (13.9k out), 2.9M cached | 30s |
| M3 plan | 156.9k new (24.8k out), 8.0M cached | 22s |
| M5 write | 273.0k new (48.4k out), 27.3M cached | 4m22s |
| M7 write | 508.8k new (78.7k out), 42.6M cached | 3m27s |
| M8 plan | 538.1k new (82.6k out), 47.1M cached | 28s |

- The numbers are session totals, not the turn's own use: a 28s plan turn shows 538k new. Differences between
  consecutive lines give the real per-turn cost: 15–70k new, 1–10k out, but 1–10M cached re-read (M5 write: 9.8M,
  M7 write: 7.1M). The re-read grows with every turn because each tool call re-sends the whole context.
- The comment on `tokenSummary` says it separates cached input "so long-context sessions do not look expensive".
  Cached input is cheaper per token but still drains plan quota; here it was the dominant cost.
- The quota line before each turn showed `codex 5h unknown` for every turn until the failing one (`5h 92%`), so the
  pre-turn warning from 819 never fired.
- Three of the eight milestones needed a fix round (M5 panic on a bad override, M6 duplicate banner line, M7 the
  developer escaped colons to pass a raw `rg` check). Each fix turn re-read 1–4M cached tokens on top of its work.
  M7 was an acceptance check that could be gamed; that is a check-design issue, recorded in the sprint study.
- Nothing in the output said "this session now re-reads N× more than it adds per turn" or "a fresh session would
  start at ~X". The ticket is already the only channel to the developer (lean-sprint §3), so a fresh session per
  milestone loses almost nothing.

The owner's direction: more instructions are not always the answer; better checks in the tool output, where the
host already reads, are preferred.

## 2. Proposal (to be weighed against the options in §3)

Make the done line a check, not a counter:

1. Per-turn numbers. Print the turn's own delta (`turn: 17k new, 2k out, 0.8M cached`) next to, or instead of, the
   session totals; label totals as `session:`. Needs the previous turn's totals from the session record. Verify
   first whether each provider reports per-turn or cumulative usage (Codex looked cumulative; Claude and agy
   unknown) — a small canary per provider.
2. Context size at turn end, from the same source `agent list` uses for CONTEXT_USED.
3. A cost check with a verdict line, thresholds in `spec/agent.yaml` (never Go defaults), for example:
   `[check: session re-reads 40× what this turn added (7.1M cached vs. 175k new); context 310k; a fresh session
   would start at ~25k — consider: harnez agent start --role developer --model <trait> -f <ticket>]`.
   Trigger candidates: cached/new ratio per turn, context size versus the model's compaction limit (969), turn
   count.
4. When the provider quota reading is unknown, say so in the check (`quota unknown: cost check is the only
   signal`) instead of staying silent.

## 3. Other options to evaluate before building

- Policy only: lean-sprint "fresh developer per milestone or after ~3 milestones". Cheap, but another instruction
  the host may skip; the owner prefers checks.
- Automatic compaction before resume when the context passes a per-model limit (969). Reduces re-read but costs a
  summary turn and keeps stale context.
- A `harnez agent handoff` / `resume --fresh` that starts a new session with the same name, role and model and a
  pointer to the ticket, so the fresh start is one command.
- Per-session budget (`--budget` in tokens or quota points) that stops the session with a clear message before the
  provider limit does.
- Make sure quota readings for Codex are fresh enough that 819's warning fires (why was it `unknown` for an hour?).
- Acceptance checks that cannot be gamed (M7): state the user-visible result; the host's diff review catches the
  rest. Not part of this ticket unless a tool check fits.

## 4. Verification

- Canary: per-turn vs. cumulative usage for codex, claude, agy, recorded here with raw event excerpts.
- Tests for the delta computation (first turn, resumed after compaction, provider without numbers → "unknown").
- Live: a resumed session over 3+ turns prints per-turn deltas that add up to the session totals, and the check
  line appears once the spec threshold is crossed.
- `make test-q1`; `make install`.
