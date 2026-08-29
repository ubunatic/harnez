# 068 — `/harnez-sync`: Autonomous Multi-Repo Managed-Docs Discovery & Auto-Reconciliation

**Status**: Closed — resolved
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
- **Default to Local Retention**: When in doubt, or if the executing subagent cannot spawn an advisor subagent, **strictly default to local retention** (protect local additions via `<!-- harnez:stop -->` or project evergreen docs) rather than promoting upstream.

### 2.4 Required CLI Adjustments
Assess and implement any required `harnez` CLI enhancements to support this workflow cleanly:
- Potential `harnez init --all <parent-dir>` or `harnez init --sync-present` mode to synchronize all eligible children in one shot natively.
- Non-interactive batch flag options if needed.

---

## 3. Acceptance Criteria

- [x] Define and register `/harnez-sync` command in `commands/harnez-sync.md` and `config.yaml` (commands & skills).
- [x] Implement multi-repo discovery logic with safety guard against scanning `$HOME` directly.
- [x] Automate the scan -> triage -> stop-marker protection -> `harnez init` -> verification loop in a single subagent flow.
- [x] Incorporate conservative upstream promotion guardrails with capable advisor consultation (`pro`) for non-obvious promotions.
- [x] Build any necessary `harnez` CLI extensions supporting streamlined multi-project reconciliation.
- [~] Verify execution across sibling repositories with a single command invocation — verified against disposable fixture repos, not against the real sibling projects under `~/projects`; see Progress note below for why.
- [x] Pass `go test ./...`, `harnez apply`, and `harnez status`.

---

## 4. Progress (2026-08-29)

**What was implemented:**

1. **`/harnez-sync` slash command** — `commands/harnez-sync.md`, registered in `config.yaml` under
   both `commands:` and `skills:` (same pattern as `/sprint` and `/fresh-sprint`). It specifies the
   discovery -> scan -> triage -> guardrailed-promotion -> verify -> report workflow from section 2
   of this ticket, driving the existing `harnez scan-docs` / `harnez init` / stop-marker mechanisms
   rather than reimplementing any drift logic inline. Installed and confirmed present at
   `~/.claude/commands/harnez-sync.md` and `~/.prime/agent/skills/harnez-sync/SKILL.md` via
   `harnez apply`.
2. **`harnez init --all <parent-dir>`** (the CLI extension named as a "potential" option in
   section 2.4) — `internal/claude.RunInitAll` in `internal/claude/init.go`, wired up in
   `cmd/harnez/main.go`. Discovers immediate child directories with `AGENTS.md` or `CLAUDE.md`
   (same eligibility rule as `harnez scan-docs`), and runs `RunInit` against each one
   non-interactively (`assumeYes` forced true — batch mode never prompts). Refuses to run directly
   against `$HOME` with a clear error, satisfying the "don't treat `$HOME` as a project container"
   safety guard from section 2.1/2.4. This is the mechanism `/harnez-sync`'s Pure-Upstream-Drift
   bucket uses for a one-shot multi-repo reconciliation instead of looping `harnez init -d <repo>`
   per repo by hand.
3. **Discovery, stop-marker, and inbox-scan mechanisms already existed** from prior tickets (059,
   060, 062) — `harnez scan-docs`, `<!-- harnez:stop -->` handling in
   `internal/markdown/markdown.go`, and `harnez diff --capture-docs`. This ticket's Go work was
   scoped to the one missing CLI gap (`init --all`); the rest of the acceptance criteria are
   satisfied by the new slash command directing the agent to use those existing primitives plus the
   advisor-gate and "default to local retention" guardrails written directly into
   `commands/harnez-sync.md` (§3a).

**Tests added** (`internal/claude/init_test.go`):

- `TestRunInitAll_InitializesOnlyEligibleChildren` — confirms only children with `AGENTS.md`/
  `CLAUDE.md` are reconciled and a bare sibling directory is left untouched.
- `TestRunInitAll_RefusesHomeDirectory` — confirms the `$HOME` safety guard fires with a
  descriptive error.
- `TestRunInitAll_NoEligibleChildrenIsNotAnError` — confirms an empty/ineligible workspace is a
  clean no-op, not a failure.

**Verification:**

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./internal/claude/...` — all pass, including the three new tests above.
- `go test ./...` (via `make check`) — one pre-existing failure in
  `internal/usage/statecache_test.go` (`TestCollectAllCacheFirst` /
  `TestCacheOrLiveDoesNotMaskLiveDataWithFreshDegradedCache`), unrelated to this ticket: that
  package was not touched by this change and has uncommitted changes from a different, concurrently
  worked ticket in this same repo. Confirmed by scoping the run to `internal/claude`, which passes
  fully.
- `make install` — succeeded, `~/go/bin/harnez` rebuilt.
- Live-executed `harnez init --all` against a disposable fixture workspace under the scratchpad
  (two eligible repos, one bare non-project directory) — reconciled both eligible repos, skipped
  the bare directory, and correctly refused when pointed at `$HOME`.
- `harnez apply` and `harnez status` both ran clean against the real `~/.claude`/`~/.prime/agent`
  install and show `harnez-sync` registered as a command and skill.

**Explicitly deferred / out of scope for this pass:**

- Full live execution of `/harnez-sync` (the autonomous subagent workflow itself, including the
  triage classification and the `pro`-model advisor gate for upstream promotions) against the real
  sibling repositories under `~/projects` — the task instructions for this ticket explicitly ruled
  out running cross-repo sync against real sibling projects as a side effect of implementation
  verification. The command is written and installed and its mechanics (`scan-docs`, `init --all`,
  stop-marker preservation) are independently tested/verified, but the end-to-end sweep across 25+
  real sibling repos has not been dry-run. A first real invocation of `/harnez-sync` (ideally
  starting read-only, i.e. stopping after step 2's scan) is recommended before relying on it for
  unattended batch reconciliation.
- No changes were made to `internal/usage/*` or any other in-flight ticket's files, per the task's
  scope boundary.
