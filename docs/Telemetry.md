---
title: Telemetry and Evolution Architecture
weight: 32
---

# Telemetry and Evolution Architecture

Documents the multi-layered telemetry, usage tracking, and Git artifact evolution architecture in `harnez`.

---

## 1. System Overview & Sourcing Layers

`harnez` gathers developer and agent telemetry across four distinct layers without impacting active agent context windows:

```mermaid
flowchart TD
    subgraph Upstream AI Providers
        ANTHROPIC["Anthropic OAuth / Quota API"]
        OPENAI["OpenAI Wham Quota API"]
        GEMINI["Antigravity LanguageServer RPC"]
    end

    subgraph Agent Client Stores
        CL_SESS["~/.claude/projects/*/*.jsonl"]
        CL_STAT["~/.claude/stats-cache.json"]
        CX_SESS["~/.codex/sessions/*.json"]
        AGY_HIST["~/.gemini/antigravity-cli/brain/*/transcript.jsonl"]
    end

    subgraph Git Repository Plumbing
        GIT_TREE["Git Tree Objects (git cat-file --batch)"]
        GIT_LOG["Commit Headers (git log --raw)"]
    end

    subgraph Harnez Core Engines
        USAGE_ENG["internal/usage (Quota & Tokens)"]
        ASSESS_ENG["internal/assess (Multi-Track Git Sparks)"]
        HOOKS_ENG["internal/telemetry & hooks (Tool Catalog)"]
    end

    subgraph CLI & Dashboards
        CLI_USAGE["harnez usage (--watch, --project, --host)"]
        CLI_HIST["harnez dochistory / repo-history (--diff, --tracks)"]
        CLI_LOG["harnez log / stats"]
        UMAN["uman status / uman tui"]
    end

    ANTHROPIC --> USAGE_ENG
    OPENAI --> USAGE_ENG
    GEMINI --> USAGE_ENG

    CL_SESS --> USAGE_ENG
    CL_STAT --> USAGE_ENG
    CX_SESS --> USAGE_ENG
    AGY_HIST --> USAGE_ENG

    GIT_TREE --> ASSESS_ENG
    GIT_LOG --> ASSESS_ENG

    USAGE_ENG --> CLI_USAGE
    ASSESS_ENG --> CLI_HIST
    HOOKS_ENG --> CLI_LOG
    USAGE_ENG --> UMAN
    ASSESS_ENG --> UMAN
```

---

## 2. Multi-Agent Usage & Quota Telemetry (`internal/usage/`)

### 2.1 Live Quotas vs. Raw Token Accounting
1. **Live Quotas (Rolling Capacity Windows)**:
   - Polled periodically from upstream endpoints and cached in `harnez-quota-cache.json`.
   - Upstream providers disclose **utilization percentages** and `reset_at` timestamps (e.g. 5-hour session burst and 7-day rolling window).
   - Providers conceal raw capacity denominators (absolute token ceilings).
2. **Time-Series Quota Snapshotter (`quota-history.jsonl`)**:
   - `AppendQuotaHistoryForAgent` records timestamped quota states upon cache refresh.
   - Enforces a 15-minute cross-process file lock (`flock`) and deduplication interval to prevent log spam during rapid `--watch` intervals.
3. **Multi-Node Fleet Federation**:
   - Per-host snapshot logs (`t14.jsonl`, `x600.jsonl`, `um760.jsonl`) in `~/.claude/harnez/usage-history/`.
   - Remote fetch via `harnez usage history fetch <host>` merges fleet-wide timelines and computing unified burn velocity.

### 2.2 Project-Level Token Attribution & Cost-of-Change
- `CollectProjectUsage(repoPath)` scans Claude Code, AGY, and Codex session stores and filters sessions matching the target working directory (`cwd` / `workspacePaths`).
- Correlates cumulative token spend against repository changes to compute:
  - **Tokens / Net Code LOC Added**: Generative throughput ratio.
  - **Tokens / Shipped Ticket**: Average AI compute cost per sprint task.
  - **Prompt Cache Leverage Ratio**: Cache read vs. raw write efficiency.

### 2.3 Agent Turn Statistics (`harnez stats --agents`)

`harnez stats --agents --days N [--json]` joins harnez agent session JSON in
`~/.harnez/agents/`, the adjacent `quota-readings.jsonl` turn-boundary readings,
the telemetry database's `tool_calls` rows, and provider quota history in
`~/.claude/harnez/usage-history/quota-history.jsonl`. Deleted harnez sessions
remain in the session store's `deleted/` archive for reporting. Codex host model
names are recovered from `~/.codex/sessions/**/rollout-*.jsonl` where the rollout
contains a `turn_context` model for that session.

Harnez-launched turns use paired fresh quota readings and are labelled
`measured`. Host sessions have no per-turn readings: quota-history change is
fitted to the observed session interval and labelled `fitted`. When sessions
overlap, the fitted value is apportioned by their new-input token totals (or
equally when token counts are unavailable) and labelled `fitted/shared`; it is
an estimate, not a per-session measurement. Provider quota utilization is
reported in whole percentage points, so short turns can appear as 0 or 1 point
and a single-turn drain-per-token comparison is too coarse for proof.

Per-model `5H DRAIN` sums only complete, fresh, non-reset measured turn pairs.
`PTS/100K NEW` divides those summed points by the corresponding measured new
input tokens and scales to 100,000 tokens; it is `—` until at least five turns
are measured. Host fitted drain is excluded from both figures. By default the
report shows per-model totals and the newest 20 sessions; `--all` includes every
session. `harnez agent rate --name <session> <1-5> "<reason>"` records a rating
on that session's latest turn, and the QUALITY column and per-model average
show recorded ratings.

---

## 3. Git Artifact Evolution & Braille Sparklines (`internal/assess/`)

### 3.1 Multi-Track Git Classification
The Git evolution engine inspects commit trees via `git cat-file --batch` in sub-100ms runtime (zero disk checkouts), classifying blobs into five functional tracks:
- **`Code`**: Source code files (`*.go`, `*.rs`, `*.py`, `*.c`, `*.sh`, `*.ts`) excluding tests.
- **`Tests`**: Test files (`*_test.go`, `test_*.py`, `tests/**`) calculating test/code ratio.
- **`Docs`**: Markdown documentation (`docs/**/*.md`, `README.md`, `AGENTS.md`) excluding tickets.
- **`Skills`**: Agent skill declarations (`skills/**/SKILL.md`, `commands/*.md`).
- **`Issues`**: Issue tracker tickets (`issues/*.md`), tracking open vs. closed velocity.

### 3.2 Double-Resolution Braille Sparkline Engine
Using Unicode Braille patterns (`\u2800`–`\u28FF`):
- **2 Samples per Terminal Cell**: Left column encodes dots 1, 2, 3, 7; right column encodes dots 4, 5, 6, 8.
- A 10-character box displays **20 discrete time points** with 4 vertical levels per sub-column.
- **Baseline Zero**: Renders as `⣀` (`\u28C0` — dots 7 and 8) rather than empty whitespace or mid-height blocks.
- **Delta Velocity Color Tinting**:
  - Green (`\x1b[32m`): Growth / additions dominant ($v_{i+1} > v_i$).
  - Red (`\x1b[31m`): Pruning / removals dominant ($v_{i+1} < v_i$).
  - Muted/Default: Steady state ($v_{i+1} == v_i$).

### 3.3 Additions vs. Removals Breakdown (`--diff`)
- Evaluates consecutive sampled commit deltas to compute separate cumulative addition ($CumA$) and removal ($CumR$) trajectories.
- Allows developers to distinguish clean continuous additive progress from heavy restructuring/refactoring churn.

---

## 4. CLI Invocations & Commands Reference

| Command | Scope | Description |
| :--- | :--- | :--- |
| `harnez usage` | Fleet / Agent | Live multi-agent token counters, active models, and 5h/7d quota windows (`--watch`, `--json`). |
| `harnez usage --project <path>` | Repo / Workspace | Attributed project token spend, model breakdown, and generative cost-of-change metrics. |
| `harnez usage history stats` | Multi-Node | Merged fleet-wide usage statistics, burn rates, and total token lifespan. |
| `harnez dochistory [files...]` | Single File / Glob | Document token growth and per-commit sparklines. |
| `harnez dochistory --tracks` | Repo (All Tracks) | Multi-track evolution card across Code, Tests, Docs, Skills, and Issues (`repo-history`). |
| `harnez repo-history --diff` | Repo (Diff Breakdown) | Expanded multi-track view separating additions ($+$ in green) from removals ($-$ in red). |
| `harnez log` | Invocations | Chronological record of all CLI tool executions and exit codes. |

---

## 5. Architectural Invariants & Pitfalls Avoided

1. **Zero Context Overhead**: Telemetry inspection runs out-of-band; raw logs and large sqlite databases are never dumped into model context.
2. **Plumbing over Checkout**: Git history inspection uses streaming object headers (`git cat-file --batch`), never checking out historical worktrees or disk commits.
3. **Cross-Process Flock Coordination**: All append-only JSONL writers (`quota-history.jsonl`, `t14.jsonl`, `x600.jsonl`) acquire exclusive file locks to prevent multi-agent race corruption.
4. **Baseline Zero Invariant**: Sparklines on absolute zero metrics must render baseline runes (`⣀` for Braille, ` ` for blocks), never mid-level glyphs.
5. **Serialized Store Migration**: `telemetry.Open()` runs schema init and migration inside one `BEGIN IMMEDIATE` transaction on a pinned `sql.Conn`, so concurrent first-openers cannot both see "column missing" and race on `ALTER TABLE` (issue 341: `duplicate column name` and a lost row). Migrations must stay idempotent: "duplicate column" counts as already migrated, and a DB stamped current but missing a column self-heals (issue 424). Every migration ships a legacy-DB fixture test; the 32-process cold-start test guards the race. Verified on Linux only; macOS is unverified.
6. **SQL Lives in the Spec**: schema DDL, every `INSERT` and all `query.go` statements are in `spec/telemetry.yaml` (loaded once via `sync.OnceValues`, validated by `spec/schemas/telemetry.schema.json`). Filters are named `predicates`; grouped queries use one template plus a `group_columns` allowlist. AST tests forbid SQL literals in `schema.go`, `insert.go` and `query.go` and fail on unused spec entries. Not yet moved: migration SQL and SQL in `classify.go`, `sanitize_cache.go`, `issuesnapshot.go`, `export.go`, `economics_query.go` (issue 458).
7. **Data-Quality Checks**: `harnez stats --quality` (`--json`, `--strict`) runs the spec's `quality_checks` read-only (`mode=ro`) against the live store and reports PASS/WARN per check with row counts and percentages. `warn_above_percent` is the only threshold. Odd data is the signal that the model is too thin; findings go into tickets, not inline fixes. Known gap: `tool_calls` has no `model` column (issue 461).

# Compaction economics (issue 423)

`harnez stats` reports recorded Codex compaction economics in both text and
`--json` output. The `compaction_economics` rows retain the model, pricing
revision, component rates, integer micro-USD estimates, and a completeness
status: `complete`, `partial`, or `insufficient_data`. `HARNEZ_PRICING_FILE`
may point to a JSON `model -> {model, revision, rates}` catalog; otherwise the
recorded `harnez-2026-09-18` catalog is used. Unknown models are explicitly
insufficient data.

These are model-price estimates, not provider billing or invoices. Cached,
uncached, output, and reasoning rates are applied to provider-reported token
counters where available; missing counters, missing rates, and counter resets
remain nullable and prevent unsupported savings claims. Reproducibility comes
from the pricing revision and rates stored with every result. A session with
no compaction reports zero compactions and `insufficient_data`, rather than
claiming a saving.
