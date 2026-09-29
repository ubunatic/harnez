# 644 — agent: agy compaction never acknowledged, resume always fails

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Bug
**Related**: [[594-agent-codex-compaction-never-acknowledged-over-limit-resume-always-fails]], [[640-stop-sound-hook-player-fallback-chain-or-go-native-playback-never-blocking]]

---

/goal Make `harnez agent resume` work for agy sessions after compaction, as issue 594 did for
codex; verify by resuming an agy session twice in a row; or stop and report if agy offers no
acknowledgement signal.

## Evidence (sprint 640, 2026-09-29)
- `agy:flash37:med` sessions dev640 and dev640b both refused the next resume with
  `Error: refusing to send resume prompt: compaction completed without an acknowledgement`.
- Every agy turn ran to millions of tokens (dev640b 4.2M, dev640c 5.4M, mostly cached), so each
  one compacted; resume therefore never worked and every milestone needed a fresh session.
- Relevant code: `internal/subagent/compact.go`; the codex fix is in 18d93b5 and b0a4a83.
