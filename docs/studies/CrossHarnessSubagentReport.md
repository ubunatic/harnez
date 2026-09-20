# Cross-Harness Subagent Dispatch Benchmark: Codex (Luna) vs. Claude Code (Haiku)

## 1. Executive Summary

During the lean-sprint execution for **Issue #417** (`harnez agent` MVP), we orchestrated multi-step subagent dispatch across two low-cost model providers:
1. **OpenAI Codex** (`gpt-5.6-luna`, low reasoning effort) via `codex exec`
2. **Claude Code** (`claude:haiku`, latest Haiku tier) via `claude -p`

Both models served as autonomous developer agents tasked with implementing distinct milestones of the subagent driver engine, session manager, and CLI command surface.

---

## 2. Empirical Performance & Milestone Execution

| Milestone | Target | Dispatched Coder | Execution Duration | Tests Produced / Status | Output Quality & Autonomy |
|---|---|---|---|---|---|
| **M1** | Core Driver Engine & Provider Parsers (`internal/subagent/driver.go`, `codex.go`, `claude.go`) | `codex:luna:low` | ~2m 40s | 3 tests / PASS | Created complete `Driver` interface, JSONL streaming parser for Codex, and Claude JSON parser with zero syntax errors. |
| **M2** | Session Manager, Ancestry Tracking & Compaction Guard (`internal/subagent/session.go`) | `claude:haiku` | **18s** | **24 tests** / PASS | Exceptionally fast; produced comprehensive session store, lineage validation (`CanManage`), and threshold checks with 100% pass rate. |
| **M3** | CLI Command Surface & Reconnect Banner (`cmd/harnez/agent.go`) | `codex:luna:low` | ~2m 15s | 2 tests / PASS | Implemented all Cobra subcommands (`start`, `resume`, `list`, `status`, `compact`, `stop`, `delete`), Reconnect Banner formatting, and `--json` support. |
| **M4** | Live Cross-Harness Verification & Telemetry Ingestion | `harnez agent` CLI | <5s per turn | Live tests PASS | Live execution confirmed full session resumption, cached token reporting, and clean teardown across both backends. |

---

## 3. Qualitative Model Comparison

### 3.1 OpenAI Codex (`gpt-5.6-luna:low`)
- **Strengths**:
  - **Complex Infrastructure Fluency**: Handled multi-file Go packages, Cobra CLI command structures, and JSON streaming buffers without needing intermediate prompts or micro-guidance.
  - **Tool Invocation**: Correctly called testing tools and committed changes at milestone boundaries adhering to conventional commit requirements.
- **Observations & Nuances**:
  - Higher execution latency due to reasoning turn overhead (~2-3 minutes per milestone turn).
  - High token efficiency in prompt cache re-use once sessions were initialized.

### 3.2 Anthropic Claude (`claude:haiku` / `haiku-latest`)
- **Strengths**:
  - **Unmatched Speed**: Completed full code synthesis and 24 unit test cases in just **18 seconds**.
  - **Test Rigor**: Wrote extensive edge-case assertions (e.g. invalid IDs, empty directory creation, lineage checks for self/child/ancestor/unrelated callers, non-JSON file handling).
  - **Spec Adherence**: Followed Go style and interface conventions without redundant boilerplate.
- **Observations & Nuances**:
  - `claude -p` requires non-interactive flag `--dangerously-skip-permissions` and benefits from structured `--output-format=json` when capturing machine-readable telemetry.
  - Specifying `--model haiku` allows Claude CLI to automatically track upstream model improvements (e.g. Haiku 3.5 to Haiku 4.5) without hardcoded version strings.

---

## 4. Key Orchestration Insights for Sprint Workflows

1. **Optimal Role Allocation in Sprints**:
   - **Fast Feature & Test Implementation (M1/M2)**: `claude:haiku` excels at ultra-fast TDD iterations, unit test authoring, and file-bounded components.
   - **Multi-File CLI Wiring & Architecture (M3)**: `codex:luna:low` excels at broad cross-package integrations and reasoning through CLI parameter interactions.
2. **Unified Command Surface Benefits**:
   - The standardized `harnez agent start <provider>:<model>` syntax cleanly abstracts away provider-specific CLI flags (`--dangerously-bypass-approvals-and-sandbox` vs `--dangerously-skip-permissions`).
   - The caller immediately receives token consumption telemetry (`tokens_cumulative`, `cached_tokens`) and a single-line resume command.
3. **Session Lineage Invariant**:
   - Persisting `parent_session_id` and checking `CanManage()` prevents concurrent IDE, terminal, and CI agent processes from terminating each other's active workers.
