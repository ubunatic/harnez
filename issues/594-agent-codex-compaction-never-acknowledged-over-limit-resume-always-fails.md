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
Canary: Codex CLI `0.157.0`, new real session with `-c model_auto_compact_token_limit=10000`, followed by two resumes (third turn used limit `1000`). Session `01a0daad-c81f-7552-9846-a9868b142aea`, rollout `~/.codex/sessions/2026/09/26/rollout-2026-09-26T00-26-57-01a0daad-c81f-7552-9846-a9868b142aea.jsonl`. The third turn produced a `compaction` rollout record. Its `last_token_usage.input_tokens` was `13937`, while cumulative `turn.completed.usage.input_tokens` was `41691`; the preceding turn usage record was `27754`. **C succeeded**: auto-compaction works and the context measure drops to a per-turn count.

Evidence excerpt (5 lines; encrypted compaction payload omitted):

```jsonl
{"type":"turn.completed","usage":{"input_tokens":27754,"cached_input_tokens":24064,"output_tokens":10}}
{"type":"compaction","id":"cmp_06e964c86f3f6504016ab6f55168e487d2ad0aa932bea59e8a","encrypted_content":"<redacted>"}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":41691},"last_token_usage":{"input_tokens":13937,"cached_input_tokens":11008,"output_tokens":5}}}}
{"type":"turn.completed","usage":{"input_tokens":41691,"cached_input_tokens":35072,"output_tokens":15}}
```

## 3. Implementation & Verification Plan
Set `model_auto_compact_token_limit` to `agent.compact_threshold_tokens` on every Codex exec and resume. Read `ContextTokens` from the latest rollout `last_token_usage.input_tokens` record; never infer it from cumulative `turn.completed` usage. Remove the Codex `/compact` prompt path and rely on Codex auto-compaction during exec. Test argument construction, rollout parsing using the real token-count excerpt, and the selected dispatch path. Verify with the single Quota-1 run (`make test-q1`).
