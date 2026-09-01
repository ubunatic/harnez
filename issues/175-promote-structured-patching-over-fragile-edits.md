# 175 — Agent Instruction & Practice: Promote Structured Patching Over Fragile Edits

**Status**: Closed — resolved in structured patching guidelines
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Practices / Prompting
**Related**: [[040-agent-context-duplication-and-file-read-discipline]], `AGENTS.md`, `internal/telemetry/`

---

## 1. Problem & Motivation

Telemetry data from `harnez stats` reveals a significant reliability gap between file editing approaches:
- **`Edit` (string replacement / line matching)**: **8.6% failure rate** (score 4.69)
- **`apply_patch` (structured diff / patch)**: **0.0% failure rate** (score 5.00)
- **`Write` / `Read`**: **0.0% failure rate** (score 5.00)

Single-line or narrow string replacement tools frequently fail due to whitespace mismatches, stale line offsets, or ambiguous target strings in refactored files. Structured patching and whole-block replacement provide deterministic, idempotent changes that drastically reduce editing retries and context pollution.

## 2. Technical Specification

1. **Workspace Agent Instruction Updates (`AGENTS.md`)**:
   - Add explicit guidance in global and template `AGENTS.md`:
     - *"Prefer structured patch tools (`apply_patch`) or whole-block replacements over narrow string substitution edits."*
     - *"When making multi-line edits, ensure sufficient surrounding context lines to avoid ambiguous pattern matches."*
2. **Harnez Config & Template Sync**:
   - Update `config.yaml` agent instruction templates (`apply` bundle) so that Claude Code, Codex, and other supported harnesses carry the structured patching guideline by default.
3. **Telemetry Tracking**:
   - Monitor `harnez stats --tool Edit` vs `harnez stats --tool apply_patch` to track long-term reduction in edit failure rates across agent sessions.

## 3. Verification Plan

- [x] Run `harnez apply` and `harnez diff` to confirm the updated practice renders cleanly across managed harness instruction files.
- [x] Validate that agent prompts prioritize robust block patching and reduce edit retry loops.
