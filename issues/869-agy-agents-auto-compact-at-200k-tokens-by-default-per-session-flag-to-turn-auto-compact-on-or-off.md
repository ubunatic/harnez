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
resume that compacts, or stop and report if agy exposes no usable context size. <!-- harnez-lint:ignore glossary -- problem statement quote -->

## Findings (2026-10-23)

- The gate from 591 uses "last-turn full input tokens"; agy was excluded (644) because its context size was <!-- harnez-lint:ignore glossary -- problem statement quote -->
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
- Context size: the advisor guessed <!-- harnez-lint:ignore glossary -- problem statement quote --> `input_tokens`, but the dev-714 transcript shows per-call
  `input_tokens` ~2.2k with `cache_read_tokens` ~106k, so `input_tokens` alone is only the uncached part.
  Use `input_tokens + cache_read_tokens` of the last call; confirm in the canary (step 1).

## Way forward

1. Canary: confirm which agy usage field equals the context size (compare before/after a manual `/compact`). <!-- harnez-lint:ignore glossary -- problem statement quote -->
2. Enable the existing gate for agy with that measure; default threshold stays the spec value.
3. Flag on `harnez agent start` and `resume` (name per `docs/CLIDesign.md`, e.g. `--auto-compact=false`),
   saved in the session record so later resumes keep it; `harnez agent list`/status shows it. Tests for on,
   off, and default.

Status check (2026-10-24): nothing built yet. `cmd/harnez/agent_run.go` still skips automatic compaction
for agy; the context measure (`input_tokens + cache_read_tokens` of the last call) has no canary yet.

## Learnings: Claude per-session compaction (2026-10-26)

Scope note: the Claude side of this topic was parked in 956 (closed, item 4). Its findings are recorded here
because this ticket owns the per-session flag.

- **Where the trigger is.** `/context` (interactive Claude Code, Haiku 5.5) shows an "Autocompact buffer" line
  (33k in this session). Compaction starts at the auto-compact window minus that buffer: 100k − 33k = 67k, so a
  100k window compacts at about 67k, not 100k. This is the answer to "where is the % at which compaction starts".
- **The window did not come from settings.** `~/.claude/settings.json` sets `autoCompactWindow` to 200000
  (top level and `claude-sonnet-5-5`), and the shell env had no `CLAUDE_CODE_AUTO_COMPACT_WINDOW` or
  `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE`. The 100k value most likely came from the launch command (a flag or env in the
  launcher). Confirmed 2026-10-27: the owner launched with `--autocompact 100000` (969 §2a, `docs/AutoCompaction.md`,
  which also has the full trigger formula and the `-p` canaries).
- **Print mode is not proof.** 956's `-p` canary (100k window, 10% override) did not compact at about 52k. That
  does not show interactive sessions ignore the values; the interactive `/context` trigger above is the
  evidence that they apply. Still open: a canary that sets `CLAUDE_CODE_AUTO_COMPACT_WINDOW` and
  `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` (or `--autocompact`) in an interactive or `-p` run, checked by a
  `compact_boundary` near the expected size.
- **Harnez's `/compact` is a different path.** `compact_threshold_tokens` (200000 in `spec/agent.yaml`) is sent as
  `/compact` by harnez; it does not change Claude's own trigger. The per-session setting for Claude therefore
  belongs in the driver's child env (956 item 4), not in harnez's threshold.
