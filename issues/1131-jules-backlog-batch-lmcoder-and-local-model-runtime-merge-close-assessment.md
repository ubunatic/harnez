# 1131 — Jules backlog batch: lmcoder and local model runtime (merge/close assessment)

**Status**: Closed — Batch applied, all kept, reports/jules/1131.md
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Chore
**Related**: 167, 231, 722, 730, 1023, 1030, 1031, 1032

---

/goal Get a checked per-ticket verdict (keep, merge into another, close as done/obsolete) for the backlog tickets
below and apply it; or stop and report when blocked on an owner decision.

## Tickets (grouped by title)
- 167 Local LLM Runtime Target Support & Telemetry (Ollama, llama.cpp, SGLang/vLLM)
- 231 local-compact doc profile for small local models
- 722 Run Claude Code against the local lmcoder model in the agent container
- 730 Smoke test LMCODER_CACHE_DIR takes a list and uses the first available disk
- 1023 Adopt lmcoder findings for Pi with local 32k models: compaction settings, capped reads and spill, context guard
- 1030 Agent start on lmcoder looks up the live context window and passes it to Pi as a per-run override
- 1031 Autocompact flag for harnez agent start, required for lmcoder models, rejected for models that cannot honor it
- 1032 lmcoder warmup: check the model is loaded and ready before sending the real task

## Plan
1. One Jules audit run over this ticket (report `reports/jules/1131.md`).
2. Check the report's evidence (jules skill Section 4), then close, merge or correct each ticket.

## Jules runs
| kind | session | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| batch audit | jules-batch-1131 | 1349301400268885726 | GH #92 (closed) | done: all kept. Rejected close of 231: 360 superseded only its mechanism, the ticket keeps the content-authoring work (`issues/231` header). 1031: no `--autocompact` flag; `subagent.AutoCompactMode` (`cmd/harnez/agent_run.go:818`) is per-provider, not a start flag. 167, 722, 730, 1023, 1030, 1032 kept |
| batch audit | jules-batch-1131 | | | pending |
