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
  Correction (2026-11-01, checked from the Jules audit in `reports/jules/971.md`): `agent status --reply` now exists (`cmd/harnez/agent_manage_cmds.go:175`), so step 3 below is done.

## 3. Implementation & Verification Plan
1. In stats mode always print the full final reply after the `[reply ...]` header, never the "already shown"
   note (that note fits `--stream full` only).
2. Print the `[reply ...]` header for every provider, including agy.
3. Add `harnez agent status --name <s> --reply` (or similar) that prints the stored `response`.
Verify: tests per provider driver that the stats output contains `[reply` followed by the reply text; a live
run with sol, luna and argon through `sed -n '/\[reply/,$p'` shows the reply each time.

## Audit 2026-11-03

Jules audit (PR GH#65, report `reports/jules/971-2.md`, checked against d8904997, all references held): partly done. `harnez agent status --reply` exists (`cmd/harnez/agent_manage_cmds.go:179`). Still open: `cmd/harnez/agent_stream.go:231-233` replaces the reply with "(the final message was already shown above)" when `lastShown` is set. The agy missing-`[reply` header part is not verified.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-971 | 7808375416013930707 | github #96 (closed) | done: partly done, 3/3 refs hold; repeats 2026-11-03 audit |
| implement | harnez-jules-impl-971 | 8076505807217153207 | none | failed 2026-11-08: stalled after 2 replies, no PR |

## Developer plan findings (d971, codex:terra:low, 2026-11-10; searcher skipped: ticket cites file:line)

- Codex/Claude stats: the reply after `[reply]` is replaced by the note when the last live message was
  labeled confirmation or plan. Full mode: Codex prints the reply only as a live event; Claude's result
  can differ from the last streamed text, so full mode can omit the real final response.
- agy uses the non-streaming path, so stats never prints a `[reply]` marker.

## Decision

- 2026-11-10 host (decide mode): stats always prints the returned reply after the marker, for every
  provider. Full mode prints the returned reply after the live events unless it equals the last shown
  message exactly. Plan-first turns keep their gate. Reason: the returned reply is the source of truth.

## M1 delivered: stats and full mode print the returned reply (3f1178a9)

Host live check: codex luna advisor in `--stream stats` piped through `sed -n '/\[reply/,$p'` printed the reply.

## M2 Pre-Work (reviewer r971, codex:terra:low)

1. P3: `cmd/harnez/agent_stream_test.go:253` full-mode dedupe case checks only that the `[reply]` marker is
   absent; also assert the message text appears exactly once (strings.Count == 1).
2. Host decision: the raw `CONFIRM:` tag after `[reply]` in stats mode stays (verbatim returned reply, per Decision).
