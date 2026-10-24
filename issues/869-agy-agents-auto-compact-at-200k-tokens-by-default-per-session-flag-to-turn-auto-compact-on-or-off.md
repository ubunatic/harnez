# 869 — agy agents auto-compact at 200k tokens by default, per-session flag to turn auto-compact on or off

**Status**: Open
**Priority**: P2 (Medium) — long agy sessions slow to a crawl (dev-714: 4,100+ steps, 5 MB transcript)
**Severity**: Minor
**Category**: Feature
**Related**: 591 (compaction gate, agy excluded), 644 (agy skips compaction), 823 (agy `/compact` works,
auto threshold left off), 714 (sprint where it hurt), `spec/agent.yaml` `compact_threshold_tokens: 200000`

---

/goal `harnez agent` auto-compacts agy sessions before the next prompt once context passes the spec
threshold (200k), like the other providers, and `harnez agent start/resume` take a flag that turns
auto-compact on or off for that session (stored with the session). Verified with tests and one live agy
resume that compacts, or stop and report if agy exposes no usable context size.

## Findings (2026-10-23)

- The gate from 591 uses "last-turn full input tokens"; agy was excluded (644) because its context size was
  unknown. 823 made `/compact` work for agy in print mode but left the auto threshold off.
- agy's print-mode JSON reports per-turn `usage` (`input_tokens`, `cache_read_tokens`, `total_tokens`,
  `internal/subagent/agy.go:163-177`). Check first whether one of these is the last call's full context; if
  not, the canary must find another measure before wiring the gate.
- Live: dev-714 ran on one conversation from M1 to M4 and grew to 4,100+ steps; the M4 turn read code for
  15+ minutes without an edit. The user ran `/compact` by hand.

## Research (2026-10-23, luna:med advisor, read-only)

- agy 1.4.6: no flag, help text or `settings.json` key for an auto-compact threshold. `agy changelog` says
  automatic compaction exists but documents no threshold or default; logs show manual `/compact` only.
  So harnez cannot configure it natively; the harnez gate (send `/compact` above the spec threshold) stays
  the way.
- Context size: the advisor guessed `input_tokens`, but the dev-714 transcript shows per-call
  `input_tokens` ~2.2k with `cache_read_tokens` ~106k, so `input_tokens` alone is only the uncached part.
  Use `input_tokens + cache_read_tokens` of the last call; confirm in the canary (step 1).

## Way forward

1. Canary: confirm which agy usage field equals the context size (compare before/after a manual `/compact`).
2. Enable the existing gate for agy with that measure; default threshold stays the spec value.
3. Flag on `harnez agent start` and `resume` (name per `docs/CLIDesign.md`, e.g. `--auto-compact=false`),
   saved in the session record so later resumes keep it; `harnez agent list`/status shows it. Tests for on,
   off, and default.

Status check (2026-10-24): nothing built yet. `cmd/harnez/agent_run.go` still skips automatic compaction
for agy; the context measure (`input_tokens + cache_read_tokens` of the last call) has no canary yet.
