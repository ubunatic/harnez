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

## 2. Code Facts (refinement, 2026-10-27)

- `tokenSummary` (`cmd/harnez/agent_stream.go:249`) prints `TurnResult` fields as-is; for Codex these are the
  rollout's cumulative counters, hence session totals labelled as if per turn.
- The store already has what a delta needs: `Session.InputTokensTotal/CachedTokensTotal/OutputTokensTotal`,
  `TokenTotalsKnown`, and `TurnRecords []TurnRecord{NewInputTokens, CachedInputTokens, OutputTokens}`
  (`internal/subagent/session.go`). No new persisted field is required unless a provider's semantics differ.
- Context size: `TurnResult.ContextTokens` / `Session.LastContextTokens()`; `-1` means unknown (agy, failed
  Codex rollout read). Same source as `agent list` CONTEXT_USED.
- Thresholds: `spec/agent.yaml` already holds `compact_threshold_tokens` and per-model `compact_thresholds`
  (969 groundwork). New cost-check keys go next to them, with schema in `spec/schemas/agent.schema.json`.

## 3. Scope (decided)

In scope — make the done line a check, not a counter:

1. **Per-turn vs. session numbers.** Done line becomes
   `tokens: turn 17k new (2k out), 0.8M cached; session 538k new, 47.1M cached`.
   Per-turn values come from the `TurnRecord` written for this turn. The driver decides cumulative vs. per-turn;
   `tokenSummary` must not guess. Unknown values print `unknown`, never `0`.
2. **Context at turn end.** Append `context 310k/400k` (or `context unknown`).
3. **Cost check line**, printed only when a threshold is crossed:
   `[check: turn re-read 7.1M cached for 175k new (40x); context 310k; fresh start ~25k; consider: harnez agent start --role <role> --model <model> -f <ticket>]`
   Triggers (any one), values in `spec/agent.yaml` under `cost_check:`:
   - `cached_to_new_ratio` per turn (proposed 20),
   - `context_fraction` of the model's compaction limit (proposed 0.75, reuses `compact_thresholds`),
   - `min_turns` before any check fires (proposed 3) to avoid noise on short sessions.
   The suggested command uses the session's real role and model.
4. **Quota unknown.** When the pre-turn quota reading is `unknown`, the done line says
   `quota unknown: cost check is the only signal`.

Out of scope (follow-up tickets if wanted after this lands):

- `agent handoff` / `resume --fresh` one-command restart.
- Per-session `--budget` stop.
- Why Codex 5h quota stayed `unknown` for an hour (819 freshness) — file separately; this ticket only surfaces it.
- Auto-compaction policy changes (969).
- Lean-sprint policy text; only add a one-line pointer to the check if the host workflow needs it.
- Ungameable acceptance checks (sprint 983 study).

## 4. Milestones

- **M1 Canary** — one 2-turn session each on codex, claude, agy; record raw usage event excerpts here and state
  per provider: per-turn or cumulative, cached field present, context known. Stop and report if a provider gives
  neither form.
- **M2 Delta** — normalize per-turn usage in the driver/store layer (`internal/subagent`), unit tests: first turn,
  cumulative provider, per-turn provider, resume after compaction (totals drop → no negative delta; print session
  only), missing numbers → `unknown`.
- **M3 Done line + context** — update `tokenSummary` and its three call sites incl. plan-first; fix its comment.
- **M4 Cost check** — spec keys + schema, check function in `internal/` (pure, table-tested), quota-unknown note.
- **M5 Docs** — the agent output section in the relevant evergreen doc; `docs/AutoCompaction.md` cross-link.

## 5. Acceptance

- Over a live 3+ turn resumed Codex session, per-turn values sum to the session totals (± rounding).
- With thresholds lowered in `~/.harnez/config.yaml`, the `[check: ...]` line appears on the crossing turn and
  names the session's role and model; with defaults on a short session it does not appear.
- agy (context unknown) prints `context unknown` and no ratio check based on missing values.
- No threshold literal in Go code; `make lint`, `make test-q1`, `make install` pass.

## 6. Open Questions for the Owner

- Replace the session totals in the done line, or keep both (proposal: both, turn first)?
- Default ratio 20 and context fraction 0.75 acceptable, or start with the check off by default?
