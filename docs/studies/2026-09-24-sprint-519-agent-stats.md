# Sprint 519 retro: agent stats, token use and plan-quota drain (2026-09-24)

Lean sprint, Opus host (Claude Code), zero-coding orchestrator. Tickets: 519 (feature),
520, 498, 521 (fixes found or pulled in on the way).

## Outcome

- `harnez stats --agents --days N [--all] [--json]`: per-session and per-model tokens
  (new = input − cached), 5h drain with source (`measured`, `fitted`, `fitted/shared`,
  `unavailable`), turn ratings. Documented in [Telemetry.md](../Telemetry.md).
- Quota readings before/after each `harnez agent start/resume` turn in
  `~/.harnez/agents/quota-readings.jsonl`; both forced fresh.
- `harnez agent rate --name <s> <1-5> "<reason>"` rates a session's latest turn.
- 498: claude resume fixed (session_id was never parsed; env stripping was refuted).
- 521: not a bug; canary with agy default on a 100% pool confirmed resume uses the
  session model.

## Developer cost (observed)

| Model | Turns | New tokens / turn | 5h drain |
|---|---|---|---|
| codex:terra:med | 6 | 60k–620k | ~1 point per big turn, ~6 points total (3% → 9%) |
| codex:luna:med | 3 | 42k–65k | not measurable (below 1 point) |
| agy:flash37:med | 1 + stop | — | stopped by "Individual quota reached" on turn 2 (cause open, see 516) |

terra was cheap enough to keep for the whole feature; luna did the small fixes.

## What worked

- Plan-first turn, then resume with write authority; review from diff and live output only.
- Reviewing the real report output (not just the diff) found every real bug: total vs new
  input, empty host model, double-counted fitted drain, rating failing on older sessions,
  deleted sessions vanishing.
- Pre-work written into the single ticket kept the developer on one context.

## What didn't

- Leaf roles cannot run `harnez agent`, so every live start/resume check had to be done
  by the host. Dispatch prompts must say so up front (two prompts asked for it anyway).
- Whole-percent provider quota makes single-turn drain unprovable; per-model rates need
  ≥ 5 measured turns. Parallel sessions on the same plan (another Opus host) add noise.
- A developer fixed a test after its single `make test-q1` run; the next worker's run
  verified it. Accept that, but track it in the ticket.
- The harnez tip ("N tool calls failed without a rate report") lands on stderr and keeps
  failing 509.
