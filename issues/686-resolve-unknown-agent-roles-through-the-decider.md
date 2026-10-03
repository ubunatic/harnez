# 686 — Resolve unknown agent roles and infer role from prompt via neus classify

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 692 (static role aliases & catalog), 695 (positional role & fuzzy target resolution); neus 041 (`neus classify` CLI)

---

## 1. Problem & Motivation
`harnez agent --role <name>` rejects unfamiliar role names that do not match the static aliases in `spec/agent.yaml`. Furthermore, when `--role` is omitted entirely, harnez always falls back to `developer` even if the user's prompt clearly asks for research/planning (e.g. `advisor`) or code review (e.g. `reviewer`).

With `neus classify` (ticket 041) available as a stable CLI enum classifier on `127.0.0.1:8736`, harnez can resolve arbitrary role wording and automatically infer roles from prompts.

## 2. Technical Specification & Ergonomics

### A. Role Resolution Cascade
1. **Exact match**: against canonical roles in `spec/agent.yaml` (`orchestrator`, `developer`, `reviewer`, `advisor`).
2. **Static alias match**: (issue 692) e.g. `explorer` -> `advisor`, `coder` -> `developer`, `critic` -> `reviewer`.
3. **Neural fallback (`neus classify`)**:
   - Run `neus classify --json --classes orchestrator,developer,reviewer,advisor --timeout 1.5s "<input>"`.
   - If a confident canonical role is returned, log notice: `harnez: resolved --role "<input>" to "<role>"`.
   - If `neus` is not installed, unreachable, times out, or abstains (exit 2/4), fail gracefully with the known roles & aliases catalog.

### B. Auto-inferring Role from Prompt (when `--role` is omitted)
- When `--role` is not passed:
  - Call `neus classify` on the first prompt sentence/summary.
  - If a role is inferred (e.g. prompt asks to audit/explore/review): log `harnez: inferred role "<role>" from prompt` and set the role.
  - If ambiguous or unclassified: fall back to `default_role: developer`.

## 3. Implementation Plan
- **M1 (Resolver Integration)**: Add `subagent.ResolveRoleDynamic()` in `internal/subagent/agentspec.go` calling `neus classify` on unknown strings.
- **M2 (Prompt Auto-Inference)**: Add prompt classification in `cmd/harnez/agent_run.go` when `--role` is empty.
- **M3 (Verification & Tests)**: Unit tests with mock classifier + integration tests with `neus classify`.
