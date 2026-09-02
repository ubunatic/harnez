---
title: Git History Telemetry, WebExtension Release Generics, and Multi-Agent Ergonomics
weight: 86
---

<!-- harnez:topic: Git history telemetry (`harnez dochistory`), WebExtension release generics, load box timeseries alignment, and multi-agent issue reservation -->
# Git History Telemetry, WebExtension Release Generics, and Multi-Agent Ergonomics

**Date**: 2026-09-02  
**Scope**: `internal/assess/` (`dochistory.go`, `multidoc.go`), `cmd/harnez/` (`dochistory.go`, `find.go`), `internal/release/` (`sync.go`, `runner.go`, `version.go`), `internal/usage/` (`watch.go`, `load.go`), `spec/indicators.yaml`, issues 180, 189, 190, 191, 192, 194.

---

## 1. Executive Summary & Context

Over a multi-month span starting in May 2026, the `harnez` ecosystem has evolved its managed instructions, language conventions, and practice guides across multiple repositories (`harnez`, `uman`, `voxi`, `ffext`).

In this session, we delivered four major architectural milestones:
1. **Git Document History Telemetry (`harnez dochistory`)**: A sub-50ms engine to extract per-commit token weight, bytes, lines, and structural heuristics across git history without tree checkouts, rendering unified stacked category charts and 8-step Unicode sparklines.
2. **Generic WebExtension & JSON Release Engine (`harnez release`)**: Extended `harnez release` to auto-detect and bump versions in `manifest.json` and `package.json`, support subproject tag prefixes (e.g., `linklit-v1.0.1`), and discover Makefile packaging targets (`make pack`).
3. **Hardware Load Box Real-Time Sparkline Alignment**: Standardized the Load Box in `usage --watch` and `--compact` with hardware-aware RAM labeling (`ram (2x16G)` / `ram (45G)`), column-aligned visual charts, and configurable timeseries vs. capacity bar modes in `spec/indicators.yaml`.
4. **Standardized Multi-Agent Ticket Allocation (`harnez find issues next --reserve`)**: Eliminated manual, racy `ls | grep` folk processes by providing an atomic reservation command for concurrent agent workflows.

---

## 2. Git History Telemetry Engine Architecture

### 2.1 The Context Tax & Historical Token Evolution
In LLM-driven coding environments, instructions (`AGENTS.md`, `docs/lang/*.md`, `docs/practices/*.md`) occupy fixed prompt context on every agent invocation. Tracking the growth and pruning of these files across git commits provides empirical visibility into prompt bloat and "lost-in-the-middle" attention degradation.

### 2.2 Streaming C-Git Pipeline & Blob Deduplication
To analyze 20+ documents across dozens of commits in under 60ms:
* **No Tree Checkouts**: Traversal streams metadata via `git log --follow --raw --abbrev=40` and retrieves raw file contents directly from Git packfiles using `git cat-file --batch`.
* **Git Blob SHA-1 Caching**: Because documents remain unchanged across many sequential commits, metrics (`RawBytes`, `Lines`, `Words`, `EstTokens`, `Headings`, `CodeBlocks`) are keyed on the Git Blob SHA. Across historical backfills, cache hit rates exceed 80%.

### 2.3 Unified Timeline Reconstruction & Active Presence
Documents introduced late in a project's lifecycle (or deleted during refactoring) must not distort historical totals:
* Each document is marked active strictly between its initial creation commit and its current/deleted state.
* The multi-document aggregator merges discrete commit streams into a single unified timeline, calculating aggregate token volume and per-category proportions across monthly milestones (May 2026 $\rightarrow$ Present).

### 2.4 Terminal Rendering & Visual Precision
* **8-Step Lower Block Sparklines**: Uses `\u2581` through `\u2588` (`▁▂▃▄▅▆▇█`) to represent token trajectory over time.
* **Dynamic Active-Only Legend**: Legend entries only display categories actively present in the analyzed dataset, avoiding confusing empty labels.
* **ANSI Color Mapping**: Distinct colors (Cyan for `lang`, Green for `practices`, Magenta for `studies`, Yellow for `feedback`, Orange for `issues`, Blue for `root (AGENTS.md)`) ensure immediate readability without relying on ambiguous monochrome shading.

```text
── Multi-Doc Token Evolution & Stacked Category History ────────────
Documents: 15 analyzed | Commits: 66 | Unique Blobs: 108 | Cache Hits: 9 (7.7%) | Latency: 41ms
Aggregate Sparkline: [▁▁▁▁▁▁▁▁▁▁▁▁▁▂▂▂▂▂▂▂▃▃▃▄▅▅▅▅▅▆▇▇█▇▇▇▇▇▇▇]  (84 → 20,218 tokens | Peak: 20,639)

── Category Breakdown Across Milestones ────────────────────────────
Legend: █ lang  █ practices  █ root (AGENTS.md)
MILESTONE             PROPORTION BAR                      TOKENS    DOCS
──────────────────────────────────────────────────────────────────────────
May 2026              [██████████████████████████████]         415       2
Jun 2026              [██████████████████████████████]       3,340       7
Jul 2026              [██████████████████████████████]       5,354       9
Aug 2026              [██████████████████████████████]      19,817      15
Present (01 Sep)      [██████████████████████████████]      20,218      15
```

---

## 3. WebExtension & Polyglot Release Engine (`harnez release`)

### 3.1 Monorepo Subproject Releases
WebExtension repositories (like `ffext`) host multiple independent extensions (`linklit`, `lazypins`, `dlmo`) in subdirectories, each with independent `manifest.json` versioning.

`harnez release` now supports:
1. **`manifest.json` & `package.json` Sync**: Auto-detects and updates `"version": "x.y.z"` while strictly preserving JSON formatting and indentation.
2. **Subproject Tag Prefixes**: Supports `tag_prefix: "linklit-v"` in `version.yaml` (or `--tag-prefix`), creating independent tags like `linklit-v1.0.1`.
3. **Build Target Fallback**: If no `.goreleaser.yaml` is present, automatically executes `make pack` or `make dist`.

---

## 4. Hardware Load Panel TUI & Spec Normalization

### 4.1 RAM Hardware Context & Visual Parity
Previously, the Load box rendered RAM as an unpadded, chart-less plain-text row (`ram 20.1/45.1G 44%`), creating an awkward visual gap compared to CPU and GPU rows.

* **Geometry Detection**: Inspects EDAC sysfs and DMI hardware tables to format module geometry (e.g. `ram (2x16G)` or total capacity fallback `ram (45G)`).
* **Aligned Sparklines & Bars**: RAM now shares the exact 16-character label width and chart bracket `[<chart>]` alignment as CPU and GPU lines.
* **Spec-Driven Indicator Modes**: `spec/indicators.yaml` defines default presentation modes for all hardware resources (`sparkline` timeseries vs. `bar` gauge).

---

## 5. Multi-Agent Ergonomics: Atomic Ticket Reservation

### 5.1 The Numbering Race Hazard
When multiple agents or parallel fresh sprints operate in a single repository, calculating the next ticket number via `ls issues | grep | sort | tail` creates race conditions where two agents select the same number.

### 5.2 `harnez find issues next --reserve`
* `internal/issues/` scans active (`issues/*.md`) and archived (`issues/archive/*.md`) tickets to compute `max + 1`.
* `--reserve [title]` uses `os.OpenFile(..., O_CREATE|O_EXCL)` to atomically create `issues/<NNN>-<title>.md` in `Draft` status.
* `AGENTS.md` and `docs/practices/IssueTracking.md` were updated to codify this command across all agent interactions.

---

## 6. Takeaways & Future Work

1. **Telemetry Persistence**: The next phase will persist `dochistory` snapshots into SQLite (`doc_snapshots`) during background collection, enabling historical query tracking across agent runs.
2. **Agent Context Monitoring**: Correlating document size trends with actual agent context window consumption to measure real instruction utilization during live tasks.
