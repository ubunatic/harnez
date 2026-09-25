# 594 — agent: codex compaction never acknowledged, over-limit resume always fails

**Status**: In Progress
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Codex can complete an over-limit `/compact` resume without providing the acknowledgement text Harnez currently looks for. Harnez then correctly refuses to send the next resume prompt, but the worker cannot recover. The field report occurred in ticket 591 (commits `52e769d`, `cd22fe2`, `dc279d5`).

## 2. Technical Specification / Findings
Canary: Codex CLI `0.157.0`, using `codex exec --json ...` followed by the same `codex exec resume <id> --json --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check /compact` invocation as `CodexDriver.Compact`. Raw stream: `/tmp/harnez-codex-compaction/events.jsonl`.

Codex emitted `thread.started` (`thread_id`), `item.completed` (`item.type=error` model mismatch warning), `turn.started`, `item.completed` (`item.type=agent_message`, `item.text="Compacted."`), then `turn.completed` (`usage.input_tokens=34255`, `cached_input_tokens=19968`, `output_tokens=66`, `reasoning_output_tokens=49`). No `last_token_usage` field or `token_count` event appeared. The first short turn reported `usage.input_tokens=15051`; this small canary therefore did not show a context drop (compaction turn count was higher). Event excerpt (5 lines):

```jsonl
{"type":"thread.started","thread_id":"01a0daa7-f3ac-74c2-b5db-42a28a5ad7e6"}
{"type":"item.completed","item":{"id":"item_0","type":"error","message":"This session was recorded with model `gpt-5.6-luna` but is resuming with `gpt-6-luna`. Consider switching back to `gpt-5.6-luna` as it may affect Codex performance."}}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Compacted."}}
{"type":"turn.completed","usage":{"input_tokens":34255,"cached_input_tokens":19968,"cache_write_input_tokens":0,"output_tokens":66,"reasoning_output_tokens":49}}
```

## 3. Implementation & Verification Plan
Treat successful Codex `turn.completed` for the dedicated `/compact` operation as completion acknowledgement when no assistant acknowledgement text was parsed. Continue requiring provider-reported `ContextTokens` to drop below the pre-compaction count. Add parser/driver coverage based on the captured JSONL event sequence. Verify with the single Quota-1 run (`make test-q1`).
