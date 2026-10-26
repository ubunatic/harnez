# 971 — agent start --stream stats drops the final reply when it was already printed

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 968 (sprint where it was seen)

---

## 1. Problem & Motivation
`--stream stats` promises "heartbeats and final reply only". In sprint 968 (2026-10-26) a host captured the
reply with `harnez agent start|resume ... --stream stats | sed -n '/\[reply/,$p'`. For the sol:med developer the
`[reply: 1m59s]` header was followed only by `(the final message was already shown above)`; for the
agy:argon:low advisor no `[reply` line appeared at all. The host had to read `response` from the session JSON in
`~/.harnez/agents/<id>.json`. The luna (codex) searchers printed their reply after `[reply`, as expected.

## 2. Technical Specification / Findings
- In stats mode the final message can be printed before the `[reply ...]` header and then replaced by the
  "already shown above" note, so any consumer that keys on the header loses the reply.
- agy (argon) turns printed no `[reply` header at all in stats mode.
- `harnez agent status --name <s>` shows no reply text, so there is no CLI way to re-read the last reply.

## 3. Implementation & Verification Plan
1. In stats mode always print the full final reply after the `[reply ...]` header, never the "already shown"
   note (that note fits `--stream full` only).
2. Print the `[reply ...]` header for every provider, including agy.
3. Add `harnez agent status --name <s> --reply` (or similar) that prints the stored `response`.
Verify: tests per provider driver that the stats output contains `[reply` followed by the reply text; a live
run with sol, luna and argon through `sed -n '/\[reply/,$p'` shows the reply each time.
