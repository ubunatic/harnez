# 768 — Store each agent turn's final reply so status and resume can show it again

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: 479, 610

---

/goal Make every agent turn's final reply recoverable from the session record after the job
exits, verified with a Claude and a Codex worker, or stop and report when blocked on a user
decision or denied permission.

## 1. Problem & Motivation
In a voxi sprint on 2026-10-10 (voxi issue 189) three of seven worker turns reached the host
without their final reply, and the reply could not be recovered:
- `claude:haiku:med` advisor (`audio-diag`, start): the job output held only the end of the
  reply (section 3 of 3); sections 1–2 were lost. The host had to resume the worker to resend them.
- `codex:luna:high` developer (`doctor-go`, resume for M1 and M4): the job output held only the
  `[session info]` line; no messages, no `[done]`. The commits existed, the report did not, so the
  M4 root cause had to be reconstructed from the diff.

`harnez agent status --name <s>` shows only id, status, provider, tokens and role, and
`~/.harnez/agents/<id>.json` has no reply field. A lost reply costs a re-ask turn or is gone.

## 2. Technical Specification / Findings
- Check first on HEAD whether the record or telemetry DB already stores replies.
- Unclear why the job output was truncated or empty (output-file buffering, `--stream full`
  handling of a resume, or the provider's own output). Reproduce before fixing.

## 3. Implementation & Verification Plan
- Persist the final reply of each turn (text, turn number, time) with the session.
- `harnez agent status --name <s>` (or a `--reply` flag) prints the last reply; older turns on request.
- Verify: run a start and a resume with a Claude and a Codex worker in Claude Code background
  jobs; the reply must be recoverable even when the job output is cut.
