# 1305 — One compaction threshold per model: harnez sets the provider's native window, plus --autocompact on harnez agent

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1031 (flag shape, owner-decided 2026-11-10), 1264 (watchdog), 969 (per-model limits), 869 (agy), 1023 (Pi)

---

## Ask (owner, 2026-11-27)
Apply the findings of the compaction-strategy review and add `--autocompact` to `harnez agent`. The owner's concern:
too many overlapping controls (Claude env var, Claude `--autocompact` flag, Claude settings `autoCompactWindow`,
harnez thresholds, the token watchdog) and no clear strategy.

## Findings (neon:low review, 2026-11-27; host spot-checked codex.go:132/151 and `provider_modes`)
- **Single source of truth:** harnez's resolved per-model threshold (`CompactThreshold`: user `agent.compact_thresholds`
  > spec `compact_thresholds` > user global > spec global), `internal/subagent/compact.go`.
- **Codex** already passes it natively: `-c model_auto_compact_token_limit=<threshold>` (`internal/subagent/codex.go:132,151`).
- **Claude does not.** Its native window comes from the user's Claude settings (`autoCompactWindow`,
  `modelSettings.<model>.autoCompactWindow`), the `--autocompact` flag or `CLAUDE_CODE_AUTO_COMPACT_WINDOW` (docs: env beats
  flag and settings). The native window is the only control that compacts *before* a request crosses the threshold inside a
  turn; 1264's watchdog stops after one crossing (1264c/d canaries: native 100k → 0 requests over 100k; native 200k → 1).
- The pre-turn `/compact` and the watchdog stay as backstops (one oversized tool result, auto-compact disabled).
- `legacy_growth_threshold_tokens` (`spec/agent.yaml:93`) is telemetry-only and triggers nothing.
- `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` has no effect in `-p` (measured, docs/AutoCompaction.md).

## Strategy (to implement)
| Provider | Primary | Backstop |
|---|---|---|
| Claude | `CLAUDE_CODE_AUTO_COMPACT_WINDOW=<threshold>` on every child harnez starts or resumes | pre-turn `/compact`, watchdog |
| Codex | `-c model_auto_compact_token_limit` (unchanged) | pre-turn check per `provider_modes` |
| Pi / lmcoder | pre-turn RPC compact + `pi_compaction` settings (unchanged) | — |
| agy, Jules | auto-compaction off (unchanged); manual `harnez agent compact` on agy | — |

- **Precedence for harnez-started children:** `--autocompact` on `harnez agent` > a `CLAUDE_CODE_AUTO_COMPACT_WINDOW` already
  set in the caller's env (kept, with a warning when it exceeds the threshold) > harnez threshold. The user's Claude settings
  no longer decide for harnez agents; they still apply to interactive sessions.
- **`--autocompact`** on `harnez agent start` and `resume`, shape as decided in 1031: `100k|100000|<num>|auto`; `auto` uses
  the resolved threshold; `0`, negative or malformed values fail before launch; a provider that cannot honor it fails before
  launch with a clear error. The value persists with the session for resume. This delivers 1031's flag; 1031's lmcoder
  "must specify" rule stays in 1031 unless the developer finds it falls out for free.
- The watchdog and the pre-turn `/compact` use the same effective value as the native window.

## Milestones
- **M0 canary (before code):** one Haiku `-p` run each with `CLAUDE_CODE_AUTO_COMPACT_WINDOW` at 80000 and 100000 on the
  process only (never edit `~/.claude/settings.json`): does Claude honor, clamp (docs say 100k–1M) or reject a value
  under 100k, and does the env beat a settings value? Record peaks per request. The outcome decides whether thresholds
  under 100k are clamped, rejected or left to the pre-turn compact.
- **M1:** Claude child env from the effective threshold; precedence above; tests with fixtures, no real settings.
- **M2:** `--autocompact` flag on start/resume, persisted, validated per provider (Claude env, Codex `-c`, Pi if supported,
  else error); `docs/CLIDesign.md` read first; defaults stay in `spec/`.
- **M3:** docs/AutoCompaction.md rewritten around the table above (one threshold, primary vs backstop per provider);
  remove `legacy_growth_threshold_tokens` with its telemetry callers if nothing else reads it; man page/help updated.

## Acceptance
- Live Haiku canary with the owner's settings at 200k: a harnez agent with threshold 100k shows 0 requests over 100k.
- `--autocompact 120k` on a Claude agent: the child env carries 120000 and the watchdog fires at 120k (unit test);
  on agy it fails before launch.
- `make test-q1` green; doc copies in sync.

/goal Deliver M0–M3 and pass acceptance, or stop and report when the canary contradicts the strategy or a flag/precedence
choice needs the owner. Circuit breaker: on a loop or no progress, clear the goal and write a failure report. Re-check live
code first; written 2026-11-27.
