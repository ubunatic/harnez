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

## M0 delivered (dev-1305, claude -p 2.1.330, Haiku 5.5, 2026-11-27)
Load: 10,000 lines read in 20 `harnez read` ranges, 21 requests per run; temp `--settings` file with
`autoCompactWindow: 200000` in every run; real settings untouched. Spend about $0.26.

| Run | env `CLAUDE_CODE_AUTO_COMPACT_WINDOW` | Peak prompt | Requests > 100k | Compactions (pre-compact) |
|---|---|---|---|---|
| control | unset | 164.3k | 9 | none |
| A | 80000 | 64.7k | 0 | 3 (65.9k, 65.8k, 68.4k) |
| B | 100000 | 65.5k | 0 | 3 (65.8k, 66.0k, 69.1k) |

- Env beats settings: confirmed for this pair.
- 80000 and 100000 behave the same: a clamp to a 100k floor, or ignored below it (unverified).
- Trigger is about 66k for a 100k window, not 100k. This conflicts with the 1264 canaries, which used the
  *settings* key at 100k and saw compaction at 96-106k. Either env and settings differ in meaning, or Claude
  reserves about 33k below the window.

## M0b pre-work (before M1): three more canary runs, same load and method
1. env 150000, settings 200k: compaction near 117k means a reserve formula (window minus about 33k); near 150k
   means the env is the trigger itself.
2. env 60000, settings 200k: compaction near 66k means a 100k floor; near 27k or none means another rule.
3. no env, settings file `autoCompactWindow: 100000`: compaction near 100k means settings and env differ in meaning;
   near 66k means the 1264 numbers came from another cause.
Add the rows to the M0 table; report the reserve (or none) with numbers.

## Decision (host, decide mode, after M0)
- M0b runs before M1 (about $0.2 more): the mapping from threshold to env value depends on it. Inside M0's purpose.
- Default mapping until M0b says otherwise: harnez sets the env to the threshold itself. Claude may then compact
  earlier than the threshold (about 66k for 100k), never later. That keeps "0 requests over threshold" and is
  predictable. Adding a measured reserve on top (env = threshold + reserve) is not done in this ticket: it would hard-code
  an undocumented Claude internal. If M0b shows a clean reserve, record it here and the docs (M3) state it.
- Thresholds under 100k (Claude floor if M0b confirms it): set the env anyway; Claude clamps, and the pre-turn
  `/compact` plus the watchdog enforce the lower harnez value. No error, one `--debug`-level note is enough.
- Milestones stay as written: M1 env from the threshold (no flag yet), M2 the flag. The developer's plan merged them;
  splitting keeps each diff small for review.

## M0b delivered (host re-checked the raw stream files)
| Run | env | settings file | Peak prompt | Requests > 100k | Compactions (pre-compact) |
|---|---|---|---|---|---|
| b1 | 150000 | 200k | 113.2k | 2 | 1 (117.8k) |
| b2 | 60000 | 200k | 65.1k | 0 | 3 (65.9k, 65.7k, 68.8k) |
| b3 | unset | `autoCompactWindow: 100000` | 67.6k | 0 | 3 (66.0k, 66.9k, 71.3k) |

- Claude triggers about 33k below the window (150k -> 117.8k, 100k -> about 66k), and values under 100k act as
  100k. Env and settings key mean the same; env wins. The 1264 "96-106k at 100k" reading does not reproduce in
  claude 2.1.330 (M3 notes it, no re-run).
- Consequence: a harnez threshold T on Claude compacts natively at about T - 33k (floor about 66k). The watchdog
  at T is a backstop that should rarely fire. The Decision's mapping (env = T) stands.

## M1 delivered (9d46d363): Claude children get the harnez window
`claudeChildEnv` (`internal/subagent/claude.go`) sets `CLAUDE_CODE_AUTO_COMPACT_WINDOW` from `CompactThreshold` on
Run, Resume, RunStream, ResumeStream (compact goes through Resume); a caller value is kept, warned only when above.
`runProviderCLI` takes an env (nil inherits). Tests use a fake `claude`/`pi` on PATH and a temp HOME.
`make test-q1` green (83 packages), but it ran twice: the first run failed, and the rerun should have covered only
the touched package, with the full run left to the host.

## M2 pre-work / required refinements (do first)
1. `claude_env_test.go`: `os.Unsetenv(claudeWindowVar)` does not restore the outer value. Use
   `t.Setenv(claudeWindowVar, "")` first, then unset, so the test is hermetic.
2. Disabled threshold (config 0): a warning on every Claude turn is noise for an intentional setting. Stay silent
   for the disabled case and warn only on a real resolution error. Test both.
3. A non-numeric caller value (`CLAUDE_CODE_AUTO_COMPACT_WINDOW=abc`) is kept without a word. Warn that it is
   malformed (it stays as-is; Claude decides). Test it.

## M2 scope: `--autocompact` on `harnez agent start` and `resume`
- Values `100k`, `100000`, `<num>`, `auto` (the resolved threshold). `0`, negative, `1.5k`, `12x` fail before launch.
- Precedence: flag > caller env > threshold. With the flag, the Claude child gets the flag value even when the
  caller set the env (one stderr note that the flag overrides it). Watchdog and pre-turn `/compact` use the same
  effective value.
- Codex: the flag value goes into `model_auto_compact_token_limit` (start and resume). Pi: honor it if the Pi path
  has a compaction threshold setting, else fail before launch. agy and Jules fail before launch with a clear error.
- Persist the value in the session; a resume without the flag reuses it, a resume with the flag replaces it.
- Read `docs/CLIDesign.md` first. Help text and the man page come from cobra; update the help string.
- Unit test from Acceptance: `--autocompact 120k` puts 120000 in the Claude child env and the watchdog fires at 120k.

## M2 delivered (45898eac pre-work, 3a56d0e1 flag)
- Pre-work: disabled threshold is silent (`errCompactDisabled`), a malformed caller value warns, test env hermetic.
- `--autocompact` on start and resume: `ParseAutocompact` (`auto`, `<n>`, `<n>k`; rejects 0, negatives, `1.5k`,
  `12x`, `120K`, overflow), `Model.CompactTokens` and `Session.CompactTokens` (`compact_tokens`, persisted), so
  `CompactThreshold` returns the override for the watchdog, the pre-turn compact, Codex `-c` and the Claude env.
  The flag replaces a caller env value with one warning. claude and codex only; agy, jules, pi, lmcoder fail
  before launch; `-i` rejects the flag. `auto` clears a stored value. Detached start forwards the flag.
- Host check: installed binary rejects agy and `1.5k` before launch; `go test ./internal/subagent ./cmd/harnez` ok;
  dev's single `make test-q1` green (83 packages).

## M3 pre-work (do first)
1. `--dry-run` does not show the effective window. Add one line, e.g. `compact window: 120000 (--autocompact)` or
   `(threshold)`, naming how it reaches the provider (Claude env, Codex `-c`). Test it.
2. Detached resume with `--autocompact`: add a test that the stored value reaches the worker (the start path has
   one; resume does not).

## M3 scope
- Rewrite the trigger part of `docs/AutoCompaction.md` around the Strategy table: one harnez threshold per model;
  primary vs backstop per provider; precedence (flag > caller env > threshold); the M0/M0b numbers (Claude 2.1.330
  triggers about 33k below the window, 100k floor, env equals the settings key and wins). Remove claims this
  contradicts (the 1264 "96-106k at 100k" reading; `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` stays as "no effect in -p").
- Remove `legacy_growth_threshold_tokens` from `spec/agent.yaml`, its schema and its Go readers if nothing else uses
  it; telemetry that needs a number uses the resolved threshold.
- `docs/AutoCompaction.md` is a root doc: check whether it is a copy of a copyable source (CLAUDE.md "Rules and
  Copyable-Doc Sources") and edit the source if so.
- Leave the ticket open; the host runs the live acceptance canary and closes it.

## M3 delivered (46d306db dry-run window, 3a52de28 docs)
Dry-run prints `compact window: <n> (<source>, <mechanism>)` (host checked Claude flag, Claude threshold, Codex flag;
argv matches). AutoCompaction.md "One threshold per model"; `legacy_growth_threshold_tokens` and `ShouldCompact`
removed. One `make test-q1`, 83 ok. `harnez init -d .` clean.

## Acceptance canary (host, 2026-11-27)
`harnez agent start --role advisor --model haiku:med` in /tmp/m0-1305, same 20-read load, owner's real Claude
settings, no caller env: peak context 65.7k, end context 34.5k (native compaction mid-turn), 0 requests over 100k.

## M4 fixes from the final neon:low review (host verified each at the cited lines)
1. HIGH `cmd/harnez/agent_run.go:1114-1124`: `runResume` saves `CompactTokens` before the refusal checks
   (`ResumeBlockedReason`, quarantine, blocked model, Codex quota pending, `canManageTarget` lineage). A refused
   resume, even from outside the caller's lineage, changes the stored value. Move the save after every refusal
   check (just before the turn launches) and test that a refused resume (out-of-lineage and blocked) leaves
   `compact_tokens` unchanged.
2. MED `cmd/harnez/agent_run.go:1664-1665`: `agent compact` builds `subagent.Model{...}` without `CompactTokens`; use
   `sessionModel(x)` for the driver and `Compact`. Test that the compact child gets the stored window.
3. MED `cmd/harnez/agent_cmds.go:413-426`: `resume -i` returns before the flag is parsed, so `resume -i --autocompact`
   silently ignores it and the guard is dead. Parse and reject before the interactive branch; test it.
4. LOW `cmd/harnez/agent_run.go:1074-1078`: `resume --dry-run --autocompact` skips `checkAutocompactProvider`; check
   it there too.
5. MED `docs/AutoCompaction.md:30-31`: Codex row says harnez's check is "Primary"; it is the backstop (Codex's own
   `-c` limit is primary). Pi/lmcoder row: name the pre-turn RPC compact and `pi_compaction` settings instead of
   "unverified" if the code shows it (it did per the Strategy table).

## Acceptance
- Live Haiku canary with the owner's settings at 200k: a harnez agent with threshold 100k shows 0 requests over 100k.
- `--autocompact 120k` on a Claude agent: the child env carries 120000 and the watchdog fires at 120k (unit test);
  on agy it fails before launch.
- `make test-q1` green; doc copies in sync.

/goal Deliver M0–M3 and pass acceptance, or stop and report when the canary contradicts the strategy or a flag/precedence
choice needs the owner. Circuit breaker: on a loop or no progress, clear the goal and write a failure report. Re-check live
code first; written 2026-11-27.
