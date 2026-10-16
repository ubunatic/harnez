# 768 — Store each agent turn's final reply so status and resume can show it again

**Status**: Closed — agent start and resume save each turn's reply (reply_history, last 20, spec/agent.yaml); agent status --reply/--turn N/--all and --json show them; verified live with claude:haiku:low and codex:luna:low start+resume redirected to files; the cut-off job output did not reproduce, saved history is the recovery path
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
### Findings
- The session JSON already had `response` and `messages`; `start` saved them, but `resume`
  updated turn/token metadata without copying its reply before saving. No reply history existed.
- `--stream full` prints provider message events as they arrive and only a `[done]` marker at
  finish. `--stream stats` prints the reply at the end (or a placeholder when already shown).
  A missing event or lost output tail can therefore hide the reply; this inspection does not
  establish which host/provider behavior caused the reported job output.
- The requested redirected Claude/Codex reproduction was not run: this assigned developer is a
  leaf worker and the repository role rules prohibit starting Harnez or background agents.
  The host still needs to run that integration check before closing this ticket.
- The one `make test-q1` run found that the round-trip fixture lacked its reply record. The fixture
  was corrected and its timestamp comparison uses `Time.Equal`; it was not rerun because the
  requested quota suite may run only once in this turn.

## 3. Implementation & Verification Plan
- Persist bounded per-turn reply history (text, turn number, timestamp) with the session.
- `harnez agent status --name <s> --reply`, `--turn N`, and `--all` print retained replies;
  JSON status includes `reply_history`.
- Verify unit coverage and run redirected start/resume turns with Claude and Codex from the host;
  confirm history survives even when job output is cut.
