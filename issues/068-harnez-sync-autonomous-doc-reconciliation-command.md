# 068 — `/harnez-sync`: Autonomous Multi-Repo Managed-Docs Discovery & Auto-Reconciliation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature & Agentic Ergonomics
**Related**: [059-capture-managed-docs-drift-to-inbox.md](archive/059-capture-managed-docs-drift-to-inbox.md), [060-triage-sibling-managed-docs-drift.md](archive/060-triage-sibling-managed-docs-drift.md), [062-scan-docs-across-agent-projects.md](archive/062-scan-docs-across-agent-projects.md), [064-fresh-handoff-workflow-skill-and-friction-reporting.md](archive/064-fresh-handoff-workflow-skill-and-friction-reporting.md), `docs/AgenticLoop.md`

---

## 1. Problem & Motivation

Manually iterating through 25+ sibling repositories with per-repo confirmation prompts (`ask_question` / interactive approvals) is effective for initial policy grounding, but introduces substantial operator latency for routine maintenance.

Now that the stop-marker convention (`<!-- harnez:stop -->`), repository eligibility checks (`AGENTS.md`/`CLAUDE.md`), and `harnez init -d <repo>` synchronization semantics are proven, the entire sweep and auto-reconciliation loop should be packaged into an autonomous slash command / skill (`/harnez-sync` or `/harnez-sweep`).

---

## 2. Autonomous Workflow Specification

When triggered (e.g. `/harnez-sync` or `/harnez-sync <path>`), the command spawns a fresh dev subagent that executes the entire sweep automatically:

```
                  /harnez-sync
                       │
                       ▼
        [Autonomous Subagent Dispatch]
                       │
       1. Discovery (., .., guard if .. is ~)
                       │
       2. Multi-Repo Scan & Inbox Check (`harnez scan-docs`)
                       │
       3. Autonomous Diff Classification & Triage
          ├── Pure Upstream Drift   ──> Auto-apply `harnez init -d <repo>`
          ├── Local Customizations  ──> Protect with `<!-- harnez:stop -->`, then sync
          └── Upstream Promotions   ──> Consolidate into `harnez/docs/`, then sync
                       │
       4. Verification (`harnez scan-docs <root>`)
                       │
       5. Consolidated Final Report to Operator
```

### 2.1 Workspace & Repository Discovery
- Discover eligible repositories in current directory (`.`) and parent directory (`..`).
- **Safety Guard**: If `..` resolves to `$HOME` (`~`), do not scan `~` recursively or treat `$HOME` as a project container.
- Use `harnez` eligibility filters (requires `AGENTS.md` or `CLAUDE.md`, ignore bare directories or `.git`-only folders).

### 2.2 Auto-Application Rules
- **Pure Upstream Drift**: Automatically apply `harnez init -d <repo>` to update standard managed docs (`Make.md`, `Git.md`, `Canary.md`, `AgenticLoop.md`, `IssueTracking.md`, etc.).
- **Local Customizations**: If repo-local additions exist in managed docs, ensure they reside below `<!-- harnez:stop -->` so local context is never lost.
- **Custom Section Preservation**: Preserve custom sections in `AGENTS.md` (e.g. project-specific rules outside the `<!-- harnez:begin ... -->` block).
- **Zero Operator Prompting**: Run the entire batch without stopping for interactive prompts per repo.

### 2.3 Conservative Upstream Promotion Guardrails
Upstream promotion must be treated with high skepticism to prevent polluting generic base docs with specialized paradigms:
- **Avoid False Generalization**: Patterns that feel effective in one repo (or a pair of siblings) are often paradigm-specific (e.g. CLI vs. backend daemon vs. GUI in Go, kernel vs. userland in C/Rust) rather than universally applicable.
- **Capable Advisor Gate**: Unless a proposed promotion is overwhelmingly obvious and truly universal, the automation subagent must consult a highly capable advisor model (`pro`) before modifying canonical docs in `harnez/docs/`.
- **Default to Local Retention**: When in doubt, protect specialized patterns locally (via `<!-- harnez:stop -->` or project-specific evergreen docs) rather than promoting them upstream.

### 2.4 Required CLI Adjustments
Assess and implement any required `harnez` CLI enhancements to support this workflow cleanly:
- Potential `harnez init --all <parent-dir>` or `harnez init --sync-present` mode to synchronize all eligible children in one shot natively.
- Non-interactive batch flag options if needed.

---

## 3. Acceptance Criteria

- [ ] Define and register `/harnez-sync` command in `commands/harnez-sync.md` and `config.yaml` (commands & skills).
- [ ] Implement multi-repo discovery logic with safety guard against scanning `$HOME` directly.
- [ ] Automate the scan -> triage -> stop-marker protection -> `harnez init` -> verification loop in a single subagent flow.
- [ ] Incorporate conservative upstream promotion guardrails with capable advisor consultation (`pro`) for non-obvious promotions.
- [ ] Build any necessary `harnez` CLI extensions supporting streamlined multi-project reconciliation.
- [ ] Verify execution across sibling repositories with a single command invocation.
- [ ] Pass `go test ./...`, `harnez apply`, and `harnez status`.
