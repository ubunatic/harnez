# 1306 — Claude compacts near the threshold: env window = threshold + spec offset

**Status**: Closed — Claude window = threshold + claude_compact_window_offset_tokens (23000); Haiku 100k gets 123000. Live haiku:med canary on the 1305 load: peak 90.7k (was 65.7k), native compaction mid-turn, 0 requests over 100k. make test-q1 green (83 packages).
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1305 (env window, M0b canary), 1264 (watchdog)

---

/goal Claude children compact natively close to, but below, the harnez threshold (Haiku: near 90k instead of about
66k), with the watchdog at the threshold still the backstop; or stop and report if the live canary shows requests
over the threshold.

## Ask (owner, 2026-11-27)
"compact closer to 100k is better, 66k is tight". 1305 set `CLAUDE_CODE_AUTO_COMPACT_WINDOW` = threshold; Claude 2.1.330
compacts about 33k below its window (1305 M0b: 150000 → 117.8k, 100000 → about 66k), so Haiku at 100k compacted at 66k.

## Change
- Spec `claude_compact_window_offset_tokens: 23000` (`spec/agent.yaml`, schema minimum 0, required). The Claude
  child gets window = threshold + offset (`subagent.ClaudeCompactWindow`, `claudeChildEnv`). Haiku 100k → 123000,
  expected compaction near 90k (10k headroom for one turn's growth before the 100k cost cliff). 0 restores 1:1.
- Watchdog and pre-turn `/compact` stay at the threshold; `--autocompact` sets the threshold, so `120k` gives 143000.
- A caller's env value warns when above the window harnez would set (was: above the threshold).
- `--dry-run` line names the env value: `100000 (threshold, env CLAUDE_CODE_AUTO_COMPACT_WINDOW=123000)`.

## Decision
- Offset 23000, not 33000: 33k reserve would put Claude's trigger at the watchdog line, so one large tool result
  crosses 100k (5x Haiku price) before either fires. The 33k reserve is measured, undocumented Claude behaviour; the
  offset lives in spec so a Claude update needs only a value change.

## Acceptance
- Unit tests: window = threshold + offset on Run/Resume/RunStream/ResumeStream, `--autocompact`, agent compact.
- Live Haiku canary (same 20-read load as 1305): compaction between 80k and 100k, 0 requests over 100k.

## Delivered (host, 2026-11-27)
- Code, spec, schema, tests and `docs/AutoCompaction.md` in one commit. `make test-q1` green (83 packages).
- Installed `--dry-run` for haiku:med: `compact window: 100000 (threshold, env CLAUDE_CODE_AUTO_COMPACT_WINDOW=123000)`.
- Live canary (`harnez agent start --role advisor --model haiku:med`, /tmp/m0-1305, same 20-read load, owner's real
  Claude settings): peak context 90.7k, end context 27.1k (native compaction mid-turn), 0 requests over 100k
  (1305 acceptance on the same load: peak 65.7k).
