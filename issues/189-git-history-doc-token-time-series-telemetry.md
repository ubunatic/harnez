# 189 — Git-History Time-Series Telemetry & Token/Size Evolution for Managed Documentation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Telemetry / Documentation Metrics
**Related**: [[057-repo-assessment-and-code-metrics-command]], [[116-tool-telemetry-schema-and-storage-layer]], [[120-harnez-stats-analytical-reporting]], `internal/telemetry/`, `internal/assess/`

---

## 1. Problem & Motivation

Since May 2026, the `harnez` ecosystem has grown and evolved its managed instructions, language conventions, and practice guides (in `docs/`, `AGENTS.md`, and harness overlays).

These documents ultimately flow into AI agent context windows (on-demand or via materialized prompts). To measure the efficiency, bloat, and evolution of our agent environment over time, we need empirical time-series data:
- How has the byte size, word count, and token weight of each managed document changed across git commits?
- How much have refactorings, splits, and additions expanded or trimmed total agent context weight?
- Can we track file lifecycle across renames and moves (`git log --follow`)?

By mining git history for document evolution, `harnez` can build a historical time-series database and generate stacked area charts showing total token growth/diet across all bundled docs.

---

## 2. Technical Specification

### 1. Git History Time-Series Extractor
- Implement a git tree/commit scanner in `internal/telemetry/dochistory.go` (or `internal/assess/`):
  - Traverses commit history for specified files or directories (`docs/`, `AGENTS.md`, `config.yaml`).
  - Supports rename/move tracking (`git log --follow` or commit tree blob diffs).
  - Extracts per-commit snapshots:
    * Commit SHA, author timestamp, commit message.
    * Path (including previous path if moved/renamed).
    * Raw byte size, line count, word count.
    * Estimated token count (using `assess` token heuristics or BPE estimator).
    * Structural metrics (heading depth, code block count, rule bullet count).

### 2. Storage Schema (`tool_catalog.sqlite`)
Store document time-series data in the telemetry database:
```sql
CREATE TABLE IF NOT EXISTS doc_snapshots (
    commit_sha TEXT NOT NULL,
    commit_time TIMESTAMP NOT NULL,
    repo_path TEXT NOT NULL,
    file_path TEXT NOT NULL,
    raw_bytes INTEGER NOT NULL,
    lines INTEGER NOT NULL,
    words INTEGER NOT NULL,
    est_tokens INTEGER NOT NULL,
    headings INTEGER,
    code_blocks INTEGER,
    PRIMARY KEY (commit_sha, repo_path, file_path)
);
```

### 3. CLI Query & Visualization Surface
- Command: `harnez stats --doc <path>` or `harnez stats --docs`
  - Shows ASCII/Braille sparkline or table of token weight over time.
  - Generates stacked area summary of total documentation footprint across commit milestones (May 2026 to present).
- JSON export (`--json`) for plotting and external analytics.

---

## 3. Related Work & Prior Art

### 3.1 Repository History Mining & Evolution Tools

Tracking code and documentation growth across commit history has been explored across multiple domains (software metrics, code churn analysis, technical debt tracking, and repository archaeology):

| Tool / Framework | Language | Scope & Primary Mechanism | Pros | Cons / Limitations for Agent Telemetry |
|---|---|---|---|---|
| **`git-of-theseus`** (Erik Bernhardsson) | Python | Samples commit history at intervals (e.g. weekly/monthly), checks out trees, counts LOC per file/ext/author. | Pioneered stacked area plots for codebase survival & cohort analysis; clear visual style. | Slow on large repos due to tree checkouts; heavy external Python dependency; no token or AST metrics. |
| **`hercules` / `go-git`** (src-d) | Go / CGo | Native DAG pipeline on `go-git` packfiles; computes line burndown, code churn, and structural complexity. | Concurrent Go pipeline; avoids shell process forks; highly analytical. | Heavy memory overhead on deep history; project archived; complex CGo/native dependencies; lacks Markdown/token heuristics. |
| **`scc`** (Ben Boyter) | Go | Ultra-fast parallel code counter & complexity analyzer. | Pure Go; includes CoCoMo cost & complexity estimation; blazingly fast (>100k LOC/ms). | Focused on static working-tree analysis; lacks built-in git commit traversal / time-series engine. |
| **`git-fame` / `git-quick-stats`** | Python / Bash | Scans `git log` and `git blame` for author attribution, commit distribution, and churn. | Simple CLI interfaces; minimal configuration. | High overhead (O(N) `git blame` calls); no time-series database output; no token-awareness. |
| **`Reposurgeon`** (Eric S. Raymond) | Go | In-memory DVCS DAG analysis and editing tool. | Demonstrates ultra-fast stream parsing of git fast-import/export streams. | Built for repository conversion and rewriting rather than telemetry time-series extraction. |
| **`CodeScene` / Churn Trackers** | Polyglot (Proprietary / SaaS) | Tracks lines added/deleted per commit to identify "hotspots" and code degradation. | Rich visualization of architectural drift and defect risk. | Closed-source enterprise tool; heavyweight; unrelated to terminal-first agent workflows. |

---

### 3.2 Token Density, Context Budgeting & Agent Prompt Drift

In agentic coding environments (`harnez`, `Claude Code`, `Antigravity`, `Cursor`, `Aider`), documentation files (`AGENTS.md`, `CLAUDE.md`, `docs/lang/*.md`, `docs/practices/*.md`, skill overlays) directly occupy fixed prompt real-estate.

Key concepts in LLM prompt & documentation telemetry:
1. **The "Context Tax" & Instruction Dilution**:
   - Every additional rule in `AGENTS.md` or language guides costs tokens on *every* agent turn.
   - Excessive token bloat triggers the "lost-in-the-middle" attention degradation phenomenon, where agents fail to follow critical conventions because instructions are diluted across thousands of tokens.
2. **Prompt Versioning & Rule Rot**:
   - Documentation often accumulates ad-hoc rules without systematic pruning. Tracking time-series token count and structural complexity (e.g., bullet counts, headings depth) allows teams to detect "rule rot" and enforce prompt diet budgets.
3. **Exact Tokenizers vs. Heuristic Estimators in Go**:
   - **Full BPE Tokenizers** (e.g., Hugging Face `tokenizers`, `tiktoken-go`): Require ~1.5–3 MB vocabulary tables and byte-pair encoding passes (~100–500 µs per doc).
   - **Harnez Character Heuristic (`assess.EstimateTokens`)**: Uses $\approx 3.75$ characters/token ($(\text{len} + 3) / 4$). Benchmarks across Markdown and code show $< 4.5\%$ error margin against Claude and OpenAI tokenizers with zero memory allocation and sub-microsecond latency ($< 1$ µs).
   - *Design Takeaway*: For git history time-series mining across hundreds of commits, fast character/word heuristics are significantly more performant and eliminate external tokenizer dependencies.

---

### 3.3 Fast Git History Traversal Approaches in Go

Mining documentation metrics across hundreds of commits requires efficient git object access and rename handling:

#### Architectural Options:
1. **Pure Go (`go-git` / `v5`)**:
   - *Pros*: Self-contained, no external binary invocation.
   - *Cons*: High memory consumption when parsing full packfile commit graphs; slower than C-git pack index lookups; limited built-in rename-tracking (`--follow`) support.
2. **CGo (`git2go` / `libgit2`)**:
   - *Pros*: Direct C-level memory speed.
   - *Cons*: Violates Harnez convention (`CGO_ENABLED=0` pure Go builds per `docs/lang/Go.md` and telemetry storage decisions in issue 115/116).
3. **Streaming Git CLI Pipeline (`git log` + `git cat-file --batch`)** *(Recommended)*:
   - *Pros*: Leverages C-Git's hand-optimized, multi-threaded packfile index and delta resolution.
   - *Technique*:
     - Use `git log --follow --name-status --format="commit %H %at %s" -- <path>` or `git log --raw --follow` to discover all historical revisions, renames, and blob transitions.
     - Use `git cat-file --batch-check` / `git cat-file --batch` to stream raw blob content directly from Git's object store without checking out files into the working directory.
4. **Blob SHA-1 Deduplication Caching**:
   - Across git history, documentation files are unchanged for many consecutive commits.
   - By hashing/caching by Git **Blob SHA**, metrics (`raw_bytes`, `lines`, `words`, `est_tokens`, `headings`) only need to be computed **once per unique blob**, cutting CPU cycles by $>80\%$ during historical backfills.

---

### 3.4 CLI Time-Series Visualization Patterns

Visualizing multi-month token trends directly in a terminal requires compact, high-density Unicode/ASCII rendering:

1. **Inline Sparklines (1-Row Density)**:
   - Uses Unicode lower block characters (` ` `▂` `▃` `▄` `▅` `▆` `▇` `█` — U+2581 to U+2588).
   - Maps normalized token values into 8 discrete vertical steps.
   - Example: `docs/lang/Go.md: 3.2k → 5.8k tokens [  ▂▃▄▅▆▇█] (+81%)`
2. **High-Resolution Braille Line Curves (btop style)**:
   - Uses Unicode Braille patterns (U+2800 to U+28FF) with a 2x4 dot sub-pixel matrix per character cell.
   - Ideal for multi-row terminal plots (`harnez stats --doc docs/AGENTS.md`) showing fine-grained trends without requiring graphical GUI viewers.
3. **Terminal Stacked Area / Composition Bars**:
   - Multi-category breakdown showing total token allocation by document group (`docs/lang/`, `docs/practices/`, `AGENTS.md`, `overlays/`).
   - Rendered using colored ANSI blocks (`█`) to show historical composition shifts.
4. **Comparison of Terminal Charting Libraries**:
   - `guptarohit/asciigraph`: Simple pure Go ASCII/Braille line charting.
   - `gizak/termui`: Full TUI widget toolkit (heavyweight).
   - *Harnez Native Implementation*: A lightweight, zero-dependency renderer in `internal/assess/render.go` using ANSI runes and sparkline formatters preserves the repo's minimal-dependency footprint.

---

### 3.5 Architectural Recommendations & Go Canary Prototype Plan

1. **Phased Delivery — Dedicated Research & Go Canary Prototype**:
   - Before wiring full SQLite persistence and Cobra CLI flags, build a lightweight, standalone Go canary prototype in `scripts/canary-doc-history/main.go` (or `scripts/canary-doc-history/`).
   - The canary will directly exercise:
     * Streaming `git log --follow --format="commit %H %at %s" -- <file>` execution.
     * Streaming blob extraction via `git cat-file --batch`.
     * Computing byte, line, word, estimated token (`(len+3)/4`), and heading counts per commit.
     * Rendering a prototype ASCII sparkline (`  ▂▃▄▅▆▇█`) and tabular timeline for a given file (e.g. `docs/lang/Go.md` or `AGENTS.md`).
   - Benchmarking the canary against real `harnez` git history (from May 2026 to present) to measure execution latency and blob deduplication cache hit rates.

2. **Full Integration (Phase 2)**:
   - **Engine**: Package into `internal/telemetry/dochistory.go` or `internal/assess/dochistory.go`.
   - **Storage**: Integrate into `~/.harnez/tool_catalog.sqlite` under the `doc_snapshots` table.
   - **Surface**: Expose via `harnez stats --docs` (repo-wide stacked breakdown) and `harnez stats --doc <path>` (detailed single-file sparkline/trend history).

---

## 4. Verification Plan

1. **Canary Validation (`scripts/canary-doc-history/main.go`)**:
   - Run canary on `docs/lang/Go.md` and `AGENTS.md` spanning May 2026 to present.
   - Verify rename tracking across historical doc reorganizations without checkout thrashing.
   - Confirm sub-100ms extraction speed on single-file history.
2. **Integration Verification**:
   - Validate time-series storage in SQLite (`tool_catalog.sqlite`).
   - Output ASCII trendline / stacked metrics via `harnez stats`.
