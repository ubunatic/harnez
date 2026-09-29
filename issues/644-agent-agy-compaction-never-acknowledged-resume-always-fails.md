# 644 — agent: agy resume blocked by harnez's own compaction check

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Bug
**Related**: [[594-agent-codex-compaction-never-acknowledged-over-limit-resume-always-fails]], [[640-stop-sound-hook-player-fallback-chain-or-go-native-playback-never-blocking]]

---

/goal Make `harnez agent resume` work for agy sessions: stop harnez from sending `/compact` to
agy and from reading the turn total as context size; verify by resuming an agy session twice in a
row after a long tool-using turn; or stop and report if agy exposes no per-call context size.

## Evidence (sprint 640, 2026-09-29)
- `agy:flash37:med` sessions dev640 and dev640b both refused the next resume with
  `Error: refusing to send resume prompt: compaction completed without an acknowledgement`.
- Recorded "context" per turn: 0.7M (plan turn), 4.2M and 5.4M (coding turns). A real context
  window is far smaller, so these are sums over the turn's model calls.

## Root cause (host analysis 2026-09-30)
agy compacts its own context automatically; harnez never needed to. Two harnez faults:
1. `internal/subagent/agy.go:211` sets `ContextTokens = Input + Cache` from agy's usage, which
   totals all model calls in the turn. Any tool-heavy turn exceeds the 200k threshold
   (`DefaultCompactThresholdTokens`, `internal/subagent/compact.go:12`).
2. Over the threshold, `cmd/harnez/agent_run.go:546` calls `EnsureContextUnderThreshold`, which
   runs `AgyDriver.Compact` (`agy.go:149`): it sends `/compact` as a plain prompt. agy has no such
   command, the reply lacks the word "compact", and `VerifyCompaction` (`compact.go:66`) refuses.

## Direction
- Skip harnez-driven compaction for agy, as `agent_run.go:546` already does for codex
  (non-interactive), since agy auto-compacts.
- Report agy context size from the last model call if agy's JSON exposes it; otherwise mark it
  unknown instead of the turn total. Check other users of `ContextTokens` (status, telemetry).

## Quick fix (2026-09-30)
`cmd/harnez/agent_run.go`: resume now skips harnez-driven compaction for agy (like codex), with a
`TODO(644)` in place. Fault 2 (turn-total `ContextTokens`) is still open.

## TODO: our own compaction for agy via jev
harnez has a new jev-based compaction that could compact an agy session's data on disk. Canary
first (docs/Canary.md): does agy accept a resumed session whose on-disk data was compacted, and
does it save tokens compared with agy's own auto-compaction? Only if both hold, re-enable
compaction for agy through jev instead of the `/compact` prompt.
