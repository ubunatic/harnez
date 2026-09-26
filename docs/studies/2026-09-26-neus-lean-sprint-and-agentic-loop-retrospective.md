# Neus Lean Sprint & Agentic Loop Retrospective (2026-09-26)

**Status**: Published Retrospective  
**Author**: Antigravity Host Orchestrator  
**Repository**: `../neus` (evaluated from `harnez`)  
**Related**: `docs/AgenticLoop.md`, `docs/IssueTracking.md`, `docs/studies/2026-09-25-finder-plugin-design.md`, `docs/Neus.md`

---

## Executive Summary

On 2026-09-26, a sequential lean sprint was conducted across the `neus` (neural search) codebase to close the remaining feature and design backlog: **Ticket 013** (Search Quality P0), **Ticket 016** (Sprint Handoff P1), **Ticket 004** (Component Architecture P1), **Ticket 005** (Go Cross-Module Search P2), and **Ticket 007** (Neural Tech Research P3).

The sprint strictly followed the **Lean Fresh-Handoff Sprint** protocol (`/lean-sprint`), employing fresh `luna:med` (Codex `gpt-6-luna`) subagents for implementation, independent `terra:med` (Codex `gpt-5.6-terra`) subagents for critical reviews, and an active host orchestrator executing diff-only inspections, plausibility checks, and invariant enforcement.

All five tickets were verified, delivered, and closed with zero active zombies, clean issue tracker synchronization, and 100% test passage under Quota-1 rules.

---

## Sprint Delivery & Milestone Summary

### 1. Ticket 013: Search Quality Audit & Ceiling Documentation
- **Objective**: Audit ground truth labels, evaluate reranking and query expansion levers, and establish whether the Recall@10 $\ge$ 0.7 target was achievable on current models.
- **Outcome**:
  - Audited evaluation queries in `eval/queries.yaml`, eliminating noisy behavior targets (`ports.go` which only contained constants; `session.go` which stored chat sessions rather than daemon termination).
  - Measured Hybrid Recall@10 at **0.449** overall (Code: 0.413, Docs: 0.478, Behavior: 0.500, Holdout: 0.354).
  - Validated that `bge-reranker-v2-m3` and HyDE query rewrites did not provide net recall lift over Nomic hybrid search on this corpus. Established and documented the verified 0.449 ceiling in `docs/Neus.md`.
- **Commits**: `ad40116`, `7e1ef28`.

### 2. Ticket 016: Handoff & Cache Hygiene
- **Objective**: Consolidate session state, ensure all preceding tickets were clean, and purge legacy database backups.
- **Outcome**:
  - Purged 144+ MB of stale database snapshots (`index-prev.db`, `index-pre013.db`, orphan `-shm`/`-wal` files).
  - Fixed `docs/README.md` studies index anchor table to pass `harnez index --check`.
  - Confirmed test suite green under `make test-q1`.
- **Commits**: `271a75b`, `1208a8a`.

### 3. Ticket 004: Pluggable Component Architecture
- **Objective**: Specify the extensible in-process Go architecture for extractors, chunkers, enrichers, stores, and query stages.
- **Delivery & Review Loop**:
  - **M1 (Initial Design)**: Delivered `docs/ComponentArchitecture.md` with in-process Go contracts.
  - **Review Findings (`terra:med`)**: Identified contract gaps: undefined `DocumentID` deletion/invalidation key, missing config/version provenance in `Enricher`, and conflated multi-channel retrieval with single-list ranking.
  - **M2 (Refinements)**: Developer resumed to define canonical `DocumentID` URI format, added version/config provenance to `Enrichment` records, and decoupled `ChannelRetrieval`, `FusionStage` (RRF), and `RankingStage`.
- **Commits**: `59eb521`, `a0dc1ff`, `492d1e3`.

### 4. Ticket 005: Go Cross-Module Search & Usage Guidance
- **Objective**: Build a CLI capability to scan local Go repositories, extract `go.mod` dependencies, rank module adoption, and format guidance.
- **Delivery & Review Loop**:
  - **M1 (Implementation)**: Created `internal/gomodules` package and `neus modules [dir...]` CLI command with JSON output and `~/projects` default.
  - **Review Findings (`terra:med`)**: Flagged hand-rolled parser weaknesses (lack of comment/multiline quote support), brittle error handling that failed entire scans on one bad file, and project list duplication in ranking calculations.
  - **M2 (Hardening)**: Switched to `golang.org/x/mod/modfile`, added resilient warning collection on skipped/invalid manifests, deduplicated projects in ranking, and added comprehensive test coverage.
  - **Live Plausibility Check**: Ran `neus modules /home/uwe/projects` across 31 real local projects:
    - Cobra CLI: 74% adoption (23/31)
    - `yaml.v3`: 71% adoption (22/31)
    - `golang.org/x/sys`: 42% adoption (13/31)
    - `golang.org/x/term`: 32% adoption (10/31)
    - `codeberg.org/ubunatic/loom`: 23% adoption (7/31)
    - `modernc.org/sqlite`: 6% adoption (2/31)
- **Commits**: `1bce608`, `66e5542`, `d4215fc`.

### 5. Ticket 007: Neural Tech Research (Jev, Laya, Cactus Needle 3)
- **Objective**: Evaluate future neural technologies for potential local search, extraction, and reranking integration.
- **Delivery & Review Loop**:
  - **M1 (Doc Sync)**: Integrated research findings table into `issues/007-*.md` and `docs/Neus.md`.
  - **Review Findings (`terra:med`)**: Corrected historical vs. live service wording regarding Laya port 8744 and synchronized overall tracker state.
  - **Conclusions**:
    - **Jev**: Cloud-only decision API; not suitable for local-first retrieval.
    - **Laya**: 421M ModernBERT-large ONNX model; top candidate for local candidate scoring/classification, subject to live benchmark.
    - **Needle 3**: 29–121M Apache-2.0 compact model; ideal for extraction/classification on CPU; embedding retrieval unproven on repo search.
- **Commits**: `2ac7fc3`, `ab9f789`, `0bfa7cb`, `48e5386`.

---

## Agentic Loop & Policy Assessment

### 1. Concise Mode & Host Communication Discipline
The host orchestrator strictly adhered to the principles of **Concise Mode**:
- **Diff-First / No Fluff**: Replaced conversational chatter with terse status updates and structured links to diffs and logs.
- **Direct Resume Routing**: Review items raised by `terra:med` were translated directly into actionable milestone refinement prompts without intermediate host-worker conversational rounds.
- **Artifact-First Updates**: Referenced on-disk docs and tracker files rather than dumping multi-page generated documents into the main interaction stream.

### 2. Harnez Governance & Invariant Adherence

| Convention / Invariant | Implementation in Sprint | Verification & Evidence |
|---|---|---|
| **Invariant 0 (One-Level Delegation)** | Host only spawned leaf workers (`--role developer` or `--role reviewer`). Leaf workers were never permitted to spawn child agents. | Session logs confirm single-level call hierarchy. |
| **Invariant 1 (Sequential Write)** | Single-writer discipline on the checked-out branch. Developers executed in serial order (004 $\rightarrow$ 005 $\rightarrow$ 007) without git worktree merge overhead. | Clean commit history, no merge conflicts or branch divergence. |
| **Invariant 3 (Zero Zombie Guarantee)** | All subagents launched in background via host shell with `HTO=0` wait wrappers. Teardown confirmed via `harnez agent list`. | `harnez agent list` showed all sprint sessions cleanly completed; zero hanging background PIDs. |
| **Plausibility Verification Gate** | Host executed live `neus modules /home/uwe/projects` commands to verify data proportions, percentages, and totals rather than relying solely on green synthetic tests. | Real-world 31-project scan verified Cobra (74%), YAML (71%), and Loom (23%) adoption. |
| **Quota-1 Guardrail** | Single test run boundary per code modification enforced by `make test-q1` (`harnez exec --quota-1 -- make test`). | All agent turns respected the modification requirement; no duplicate test-suite calls. |
| **Always `make install`** | CLI binary re-compiled and installed after every functional change. | Installed binary at `/home/uwe/go/bin/neus` stayed synchronized with codebase. |
| **Issue Tracker Protocol** | Issue lifecycle managed exclusively via `harnez find`, `harnez issues close`, and `harnez index`. | All 16 repository tickets closed, with `issues/README.md` and `docs/README.md` 100% in sync. |

---

## Key Learnings & Takeaways

1. **Independent Reviewer High ROI**: In both Ticket 004 and Ticket 005, the initial developer agent delivered working code/docs that passed basic unit tests, but `terra:med` immediately caught contract ambiguities (missing `DocumentID` deletion keys) and parser robustness issues (`go.mod` multiline/comment edge cases). The two-phase developer $\rightarrow$ reviewer loop prevented premature ticket closures.
2. **`HTO=0` for Background Subagent Waits**: The default 60-second `harnez exec` timeout will terminate long-running subagent wait commands unless explicitly lifted via `HTO=0`. Wrapping `harnez agent wait` and `harnez agent resume` with `HTO=0` in host background tasks provides a rock-solid reactive notification loop without polling.
3. **Plausibility Testing Over Test Mocks**: Mock tests for `internal/gomodules` verified the algorithms, but running `neus modules` against the user's actual `~/projects` folder immediately proved the utility and accuracy of the output in real-world conditions.
