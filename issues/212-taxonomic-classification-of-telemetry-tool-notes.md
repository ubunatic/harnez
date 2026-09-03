# 212 — Taxonomic Classification of Telemetry Tool Notes for Safe Visual Analytics

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature / Telemetry
**Related**: [204-sanitized-telemetry-and-token-export-for-datavis.md](204-sanitized-telemetry-and-token-export-for-datavis.md), [120-harnez-stats-analytical-reporting.md](120-harnez-stats-analytical-reporting.md), [178-distill-smart-mode-error-pattern-preservation.md](178-distill-smart-mode-error-pattern-preservation.md), [ubunatic.com/issues/029-agent-usage-and-telemetry-visualization.md](../../ubunatic.com/issues/029-agent-usage-and-telemetry-visualization.md)

---

## 1. Problem & Motivation

The zero-prose privacy default (`harnez usage export --privacy=public`) correctly strips all free-form text, notes, and tool arguments to protect proprietary business logic, commit SHAs, and internal bug reports from leaking on public dashboards (like `ubunatic.com/telemetry`).

However, completely dropping all notes leaves visual dashboards without semantic context regarding *what kind of work* the agents performed.
For analytical exploration, visitors do not need the verbatim prose — they need high-level, aggregate categories (e.g., % time spent on `Testing`, `Building`, `Editing/Refactoring`, `Debugging`, `Git Workflow`, `Instruction Review`).

We need a classification pipeline that maps free-form tool notes into a fixed, safe taxonomy of categorical enums without compromising privacy.

### Critical Cost & Performance Invariant
- **Zero Heavy-LLM Spam**: We must **NEVER** invoke expensive frontier models (e.g. Claude Opus/Sonnet, GPT-5) row-by-row for thousands of database entries.
- **Rule-First & Heavy Cache**: The system must classify 80–90%+ of calls via instantaneous, zero-cost pattern matchers, and gate any optional fuzzy LLM fallback behind strict deduplication, small local models, and persistent content-hash caching.

---

## 2. Proposed Architecture & Taxonomy

### 2.1 Canonical Category Schema

Define a closed enum taxonomy for tool executions:

```go
type ActivityCategory string

const (
    CategoryTest         ActivityCategory = "test"          // go test, pytest, jest, assertion checks
    CategoryBuild        ActivityCategory = "build"         // go build, cargo, gcc, syntax/compiler runs
    CategoryEdit         ActivityCategory = "edit"          // file edits, patches, refactors
    CategoryInspection   ActivityCategory = "inspection"    // read, grep, find, view_file
    CategoryGit          ActivityCategory = "git"           // status, diff, commit, branch, log
    CategoryDebug        ActivityCategory = "debug"         // reproducing crashes, inspecting panics, traces
    CategoryWorkflow     ActivityCategory = "workflow"      // agent handoff, issue tracking, protocol injection
    CategoryConfig       ActivityCategory = "config"        // settings, hooks, dotfiles, environment setup
    CategoryOther        ActivityCategory = "other"         // unclassified fallback
)
```

### 2.2 Multi-Tier Classification Pipeline

```
Raw Note / Tool Call
         │
         ▼
┌──────────────────────────────────────┐
│ Tier 1: Fast Deterministic Rules     │  <-- Instant (<1µs), zero-cost, runs on-the-fly
│ (Tool name, command regex, exits)    │      Matches ~80-90% of all calls
└──────────────────┬───────────────────┘
                   │
         [Match?]  ├────── Yes ──► Assign Category Enum
                   │
                   ▼ No
┌──────────────────────────────────────┐
│ Tier 2: Content-Hash Cache           │  <-- sha256(raw_note) lookup in SQLite cache table
│ (note_category_cache)                │      Avoids re-evaluating seen notes
└──────────────────┬───────────────────┘
                   │
         [Cached?] ├────── Yes ──► Return Cached Enum
                   │
                   ▼ No
┌──────────────────────────────────────┐
│ Tier 3: Opt-In Batch Classifier      │  <-- OPT-IN ONLY (harnez usage export --classify)
│ (Small local model or batched prompt)│  <-- Chunked (50-100 unique notes per call)
└──────────────────┬───────────────────┘  <-- Maps strictly to allowed enum strings
                   │
                   ▼
       Persist to Cache Table
                   │
                   ▼
         Export Category Field
```

### 2.3 Tier Details

1. **Tier 1 — Fast Rule-Based Matcher (On-the-Fly)**:
   - Evaluates:
     - `tool_name`: `Edit` -> `edit`; `Read`/`Grep`/`find` -> `inspection`; `git` commands -> `git`.
     - Output / Note regexes already present in `internal/telemetry/score.go`:
       - Test failure / pass signatures -> `test`
       - Compiler / syntax errors -> `build`
       - Crash / panic / segfault -> `debug`
       - Ticket / tracker sync phrases (`"committed ticket..."`, `"updated README index..."`) -> `workflow`
   - In our current database of ~3,200 calls, Tier 1 immediately resolves over 2,700 calls with zero LLM involvement.

2. **Tier 2 — Persistent Content-Hash Cache**:
   - Stored in SQLite (`~/.harnez/note_category_cache.sqlite` or table in `tool_catalog.sqlite`).
   - Maps `sha256(note) -> category_enum`.

3. **Tier 3 — Small Local Model (SLM) Batch Classifier via `lmcoder` (Export Time Only)**:
   - Purpose: Disambiguates custom, fuzzy agent notes without paying for expensive cloud models. Text classification into 9 fixed enums does NOT require a multi-hundred-billion parameter frontier reasoning model; a lightweight 1B–8B local model is ideal.
   - **Local Runner Integration**: Leverage companion tool **[`lmcoder`](/lmcoder)** (which provides local model serving, sandboxing, and canary endpoints) or standard local backends (llama.cpp/Ollama) to run the batch classification turns locally at zero API cost.
   - Runs **only** when `harnez usage export --classify` is explicitly invoked.
   - Evaluates only unique cache misses in bulk batches (e.g. 50–100 distinct notes at once).
   - Prompt is strictly a classification matrix: takes a numbered list of short notes and outputs `[ {index: 1, cat: "test"}, {index: 2, cat: "workflow"}, ... ]`.
   - Results are permanently saved to `note_category_cache`, so each unique note is classified at most once in its lifetime.
   - Zero hard dependency: If `lmcoder` or the local SLM runner is offline, unmatched notes safely default to `"other"`.

---

## 3. Export & Analytics Integration

In `harnez usage export`:
- Add `activity_category` to `ExportToolCall`.
- Safe for public visual dashboards: `activity_category` is a sanitized enum, never leaking the original note text.
- Enables rich dashboard features on `ubunatic.com/telemetry`:
  - Activity distribution breakdown (Donut / Bar chart).
  - Time spent on testing vs. building vs. refactoring over time.
  - Failure rate breakdown by activity category.

---

## 4. Acceptance Criteria

1. Fast rule classifier in `internal/telemetry` tags tool calls without adding measurable latency (< 1ms).
2. No row-by-row LLM calls: any model classification is chunked, opt-in, and cached by SHA-256 hash.
3. `harnez usage export` includes the safe `activity_category` enum field.
4. Unit tests prove that secret strings (tokens, paths, client names) in the note are never exposed in `activity_category`.
