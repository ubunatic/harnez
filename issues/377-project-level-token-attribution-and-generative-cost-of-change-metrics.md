# 377 — Project-level token attribution and generative cost-of-change metrics

**Status**: Closed — implemented project-level token attribution and cost-of-change
**Priority**: P2 (Medium)  
**Severity**: Minor  
**Category**: Feature  

**Related**: [023](023-usage-command-token-quota-tracking.md), [375](375-append-quota-window-snapshots-to-quota-history-jsonl-on-cache-refresh.md), [376](376-multi-track-git-history-evolution-sparks-across-code-tests-docs-skills-and-issues.md), [378](378-fleet-wide-multi-repo-git-history-sparks-and-token-attribution-matrix-for-uman.md)  

---

## 1. Summary & Motivation

While `harnez usage` tracks tokens globally per machine and per model, developers need to understand **how much AI compute was invested in specific projects**, and correlate token consumption with actual code/doc changes delivered.

Every agent harness records working directory context:
- **Claude Code**: `~/.claude/projects/<slug>/` records working directory paths (`cwd`).
- **Antigravity (AGY)**: `history.jsonl` and session transcripts record `workspacePaths`.
- **OpenAI Codex**: Session rollout files associate turns with target roots.

This ticket tracks building a project attribution scanner in `internal/usage/` that correlates cumulative project tokens with Git change metrics ("Cost of Change").

---

## 2. Proposed Metrics & Analysis

1. **Project Token Attribution**:
   - Aggregate lifetime input, output, cache-read, and cache-write tokens mapped to specific repository paths.
2. **Generative Cost-of-Change Metrics**:
   - **Tokens / Net Code LOC Added**: Measures generative ROI vs rework density.
   - **Tokens / Ticket Closed**: Computes average AI investment per sprint task.
3. **Rework & Debugging Churn Detection**:
   - High token spend with flat or undulating Git code sparks identifies refactoring churn or debugging test-fix loops.

---

## 3. Proposed CLI UX (`harnez usage --project <path>` / `harnez assess --tokens`)

```text
Project AI Telemetry: harnez (/home/uwe/projects/harnez)
Lifetime Attributed Tokens: 1.42B tokens
  - Claude Code: 1.38B tokens (Sonnet 5: 68%, Fable 5: 22%, Sonnet 4.6: 10%)
  - Antigravity:  35.2M tokens (Gemini 3.7 Flash)
  - OpenAI Codex: 4.8M tokens (gpt-5.6-sol)

Cost of Change:
  - Net Code Lines: +18.4k LOC (~1,690 tokens / LOC)
  - Shipped Tickets: 337 closed tickets (~4.21M tokens / ticket)
  - Prompt Cache Leverage: 97.2% cache read efficiency
```

---

## 4. Acceptance Criteria

- [ ] `internal/usage/` can scan Claude Code, AGY, and Codex session stores and attribute tokens by repository path.
- [ ] `harnez usage --project` (or `--cwd`) displays project-specific token rollups and model breakdowns.
- [ ] Cost-of-change metrics (tokens/LOC, tokens/ticket) are computed when run inside a Git repo with ticket history.
- [ ] `--json` output provides machine-readable project telemetry for tooling integrations (`uman`).
- [ ] Unit tests verify path normalization, cross-agent aggregation, and metric derivations.
