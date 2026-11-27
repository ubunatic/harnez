# 1307 — Claude harnez sessions stay below the owner's 200k manual net: cap and claude default threshold

**Status**: In Progress
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 1306 (offset), 1305 (env window)

---

/goal Every harnez Claude session compacts natively below the owner's manual-session net (Claude settings
`autoCompactWindow` 200000, about 167k effective), or stop and report.

## Ask (owner, 2026-11-27)
"my system settings are at 200k, so harnez should lower its own sessions; the 200k is the last safety net that I use
in manual sessions".

## Finding (host)
1306 added the 23000 offset to every threshold. Opus, Sonnet and Fable had no spec key and resolved the global
200000, so their children got `CLAUDE_CODE_AUTO_COMPACT_WINDOW=223000` (installed `--dry-run`), above the owner's
200k net. A regression introduced by 1306.

## Change
- Spec `claude_compact_window_max_tokens: 200000`: the window never exceeds it (`ClaudeCompactWindow`; 0 = no cap in Go,
  required by the schema).
- Spec `compact_thresholds.claude: 160000`: Opus/Sonnet/Fable get window 183000, native compaction near 150k; Haiku keeps
  its own 100000 keys (window 123000, about 90k).
- Precedence is unchanged: the spec `claude` key beats a user global `compact_threshold_tokens` (as the Haiku keys
  already did); a user `claude` key overrides it. Tests that set a Claude threshold through the user global now use a
  user key.

## Decision
- 160000 for non-Haiku Claude: compaction near 150k sits clearly below the manual net (about 167k) with room for one
  large tool result. Unmeasured for Opus/Sonnet; the 33k gap is from Haiku canaries (1305 M0b).

## Acceptance
- Installed `--dry-run`: opus/sonnet/fable `160000 ... =183000`, haiku `100000 ... =123000`; no Claude window above 200000.
- Unit tests: cap at and above max; every shipped Claude model's window below 200000; `make test-q1` green.
