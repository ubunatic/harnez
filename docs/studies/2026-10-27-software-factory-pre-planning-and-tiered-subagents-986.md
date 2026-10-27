---
kind: study
---

# Software factory pre-planning and tiered subagent allocation (Issue 986)

Date: 2026-10-27. Host: Antigravity (Gemini 3.8 Flash). Architect: `agy:argon:low` (`refine-986`, `arch-986`).
Target: [Issue #986](file:///home/uwe/projects/harnez/issues/986-per-project-machine-readable-glossary-and-harnez-assess-glossary-wording-check.md) (per-project machine-readable glossary and wording check).

## 1. Context & Motivation

Agentic workflows often fall into one of two traps:
1. **Monolithic sprawl**: A single high-end model reads the entire codebase, invents architectures on the fly, writes code, debugs, and runs into context exhaustion or massive token bills.
2. **Rigid scripted sprints**: Fixed multi-stage frameworks that enforce boilerplate turns even when the technical unknowns have already been resolved.

For Issue #986, we explore an experimental **software factory pattern**:
- Orchestrate specialized tiers across available subscriptions (`haiku:med`, `argon:low`, Gemini host).
- Minimize cumulative token consumption while strictly preserving code quality.
- Execute a structured **pre-planning phase** (Ticket Refinement -> Factory Design -> Adversarial Plan Review) before writing code.
- Serve as the blueprint to codify into a reusable skill (`/software-factory`).

---

## 2. Factory Role Specialization & Subscription Economics

| Role | Model | Lifecycle | Responsibility & Token Guardrails |
|---|---|---|---|
| **Host Orchestrator** | Antigravity (Host) | Continuous | Pure delegation; never ingests raw code/doc files; runs background turns (`HTO=0`), commits verified milestone diffs, enforces [Quota.md](file:///home/uwe/projects/harnez/.harnez/rules/Quota.md). |
| **Searcher** | `haiku:med` | Resumable | Codebase discovery; mandated to use `neus ask "<question>"` first before ripgrep/file reads. Resumed across queries to leverage prompt caching. |
| **Architect / Decisions** | `argon:low` | Short, focused turns | High-leverage decision maker. Evaluates API shapes, JSON schemas, edge-case math, and reviews execution plans. Strategy: feed aggregated info in one rich prompt and get a fast, decisive single-turn reply (<1m, ~50k tokens). |
| **Mechanical Developer** | `haiku:med` | Fresh per milestone | Pure translator of explicit specs into Go/YAML and tests. Never invents algorithms or resolves ambiguous architecture. 100k context limit (80k auto-compact); reset to 0k context each milestone to prevent prompt rot. |

### Circuit Breakers & Fallback Policy (Exit Criteria)
- **Haiku underperformance**: If `haiku:med` struggles (loops, breaks tests, weakens assertions, or fails Quota-1 twice), abort the low-tier worker and switch to **`claude:opus:low`** as developer.
- **Argon token/latency spikes**: If `argon:low` takes too long (>2m) or consumes excessive tokens, switch brain/advisory queries to **`claude:opus:med`**.
- **Search stability**: Searchers remain on `haiku:med` (with `neus ask`) regardless of developer fallbacks.

---

## 3. The Pre-Planning Protocol

Before initiating coding on Issue #986, the factory completed two pre-flight advisory gates:

### Step 1: Upstream Ticket Refinement (`refine-986`)
- Model: `agy:argon` (turn: 5m35s, 1.0M tokens incl. cached).
- Resolved §3 Discovery: Settled scope on domain vocabulary + ubiquitous language (`term`, `definition`, `synonyms`, `code_names`, `forbidden`). Excluded general prose style linting and AST refactoring.
- Resolved §4 Prior Art: Evaluated Vale, `textlint`/`prh`, and SKOS/DDD. Chose a native concept-centric YAML schema (`spec/schemas/glossary.schema.json` + `spec/glossary.yaml`) with Vale-style substitution mechanics.
- Produced initial Milestones M1–M4. Committed directly to the issue tracker (`e7566893`).

### Step 2: Adversarial Plan Review (`arch-986`)
- Model: `argon:low` (turn: 51s, 50k tokens).
- Question: *Does it make sense to completely skip a mid/high-tier developer in this experiment, relying solely on `haiku:med` guided by `argon:low`?*
- **Verdict**: **Yes**, because conceptual ambiguity was eliminated in Step 1. `haiku:med` functions with high fidelity when restricted to mechanical implementation of unambiguous specs.
- **Identified Failure Modes for `haiku:med`**:
  1. *Token position math*: Off-by-one errors in `go/scanner` multi-line strings/comments and Markdown fence tracking.
  2. *Regex boundaries*: Go regexp `\b` treating `_` as word characters (breaking `CONTEXT_SIZE` or code tokens), plus non-deterministic map iteration in deduplication.
  3. *Assertion weakening*: Relaxing test assertions to pass `make test-q1` when edge cases fail.
  4. *Copyable-doc drift*: Editing `docs/Spec.md` directly instead of `docs/other/Spec.md`.
  5. *Context fatigue*: Degradation if one session spans multiple milestones.

---

## 4. Guardrails & Milestone Adjustments Adopted

Following the architect's review, the factory incorporated four structural safeguards:

1. **Pre-Bake Algorithms Before Implementation**:
   For complex scanner or boundary logic (M2), `argon:low` writes the exact Go struct definitions and 15-line pseudocode before `haiku:med` starts coding.
2. **Split Milestone 3 into M3a and M3b**:
   - **M3a (CLI Wiring)**: Wire `harnez assess --glossary [--json]` and `internal/lint` with synthetic `t.TempDir()` fixtures.
   - **M3b (Repo Alignment)**: Run `harnez assess --glossary` against the live repository to generate an automated punch list of violations, then assign `haiku:med` to mechanically align them. Prevents overloading the developer with simultaneous CLI wiring and multi-file refactoring.
3. **Lock Test Cases (Anti-Tampering)**:
   Table test cases are embedded verbatim in developer prompts with explicit instructions forbidding assertion weakening. Host verifies test diffs at each commit.
4. **Session Lifecycle Policy**:
   - Resumable Searcher & Architect: retain cache, small state.
   - Fresh Developer per milestone: clean working memory (<25k tokens), eliminating compaction friction.

---

## 6. Execution Progress & Telemetry

### Milestone 1 Delivery (`87b45d83`)
- **Searcher `search-986` (`haiku:med`)**: 47s, 177k tokens (incl. cached). Successfully located `github.com/google/jsonschema-go/jsonschema`, `gopkg.in/yaml.v3`, embedded FS rules, and repo config patterns.
- **Architect `arch-986` (`argon:low`)**: 1m25s, 163k tokens (incl. cached). Designed full JSON Schema (`spec/schemas/glossary.schema.json`), base glossary (`spec/glossary.yaml`), Go structs & loader (`internal/glossary/glossary.go`), and locked table tests.
- **Developer `dev-986-m1` (`haiku:med`)**: 46s, 197k tokens (incl. cached). Fresh session. Mechanically implemented all 4 files; all unit tests passed on first run (`TestLoadBaseGlossary`, `TestValidate` across 11 subtests, `TestLoadForRepoAndMerge` across 4 subtests). Zero assertion tampering.
- **Host**: Verified `go test -v ./internal/glossary/...`, built and installed binary via `make install`, and committed `87b45d83`.
- **Verdict**: M1 completed with zero friction. The pattern of *Searcher locates patterns -> Architect locks exact types/code -> Mechanical Dev translates* executed in ~3 minutes total subagent time without needing any mid/high-tier developer.

### Milestone 2 Delivery (`b698808a`)
- **Searcher `search-986` (`haiku:med`)**: 37s, 187k tokens (incl. cached). Confirmed zero external glob dependencies, examined `internal/lint/lint.go` (`Finding`, `dropSuppressed`), and confirmed `go/scanner` patterns.
- **Architect `arch-986` (`argon:low`)**: 1m34s, 288k tokens (incl. cached). Provided complete `internal/glossary/glob.go` (pure-Go `**` glob compiler), `check.go` (regex compiler, Go comment/string token scanning, markdown fence tracking, closed ticket detection), and table test suites.
- **Developer `dev-986-m2` (`haiku:med`)**: 53s initial turn, 58s fix turn.
  - *Key observation*: `haiku:med` successfully caught a logic conflict between `argon:low`'s `hasFileLevelSuppression` (which checked line 1) and the test case (which used line 1 to suppress line 2). Instead of weakening the test assertion, `haiku:med` paused, diagnosed the root cause, and presented two concrete resolution options (A: separate `harnez-lint:ignore-file`; B: remove file check).
  - Adopted Option A; `dev-986-m2` also independently fixed inline comment suppression so inline comments don't suppress the line below. All unit tests passed cleanly.
- **Gokay Diagnostics & Health Verification**:
  - `gokay check --all --json`: `fmt` ok (531ms), `vet` ok (1820ms), `test` ok (34.7s). Found **0** lint/vet/fmt issues in `internal/glossary` (all 108 report issues were pre-existing in untouched packages).
  - `gokay assess --json`: `internal/glossary` verified at 818 code lines, 528 test lines, 65% test ratio, 0 untested packages, 0 orphan schema keys, and clean spec posture.
- **Host**: Ran `make install` and committed `b698808a`.

### Milestone 3 Delivery (`ad760614`, `403cab95`)
- **Searcher `search-986` (`haiku:med`)**: 36s, 234k tokens (incl. cached). Scouted `cmd/harnez/assess_cmd.go` flag conventions and `internal/lint/lint.go` rule interfaces.
- **Architect `arch-986` (`argon:low`)**: 1m7s, 501k tokens (incl. cached).
  - *Key architectural discovery*: Caught that `internal/lint`'s `ruleApplies` checks `rule.Languages()` against `lang`, so returning `nil` did *not* match all files—it caused the rule to be skipped entirely! Architect corrected `Languages()` to return `[]Language{LangMarkdown, LangGo}`.
  - Specified `AssessGlossaryReport`, `runAssessGlossary`, `--glossary` flag wiring, `GlossaryRule` in `internal/lint`, and table tests.
- **Developer `dev-986-m3a` (`haiku:med`)**: 52s, 301k tokens (incl. cached). Cleanly implemented CLI wiring and lint rule. All unit tests passed (`ad760614`).
- **Tool-Driven Punch List Generation**: Running the live `harnez assess --glossary` immediately discovered 36 forbidden terminology findings across 10 active docs, Go comments, and issues.
- **Developer `dev-986-m3b` (`haiku:med`)**: 8m9s, 971k tokens (incl. cached). Mechanically aligned all 10 files using the automated punch list and added inline suppression directives for problem-statement quotes in issue tickets.
- **Verification**: `harnez assess --glossary` confirmed **0 findings**. `make install` passed cleanly. Committed as `403cab95`.
The searcher (`search-986`, `haiku:med`) is mandated to call `neus ask "<question>"` first before ripgrep or file reads:
- **Turn 1 (M1 - Schema validation & embed)**: `neus ask "how does harnez embed and validate spec JSON schemas?"` returned `internal/jsonc/jsonc.go` (JSON syntax parser rather than JSON schema validation). Searcher quickly fell back to inspecting `internal/assess/ramp_spec.go` and `go.mod` to find `github.com/google/jsonschema-go/jsonschema`.
- **Turn 2 (M2 - Glob matching & lint rules)**: `neus ask "how does harnez match glob patterns with double stars in filepath?"` returned thin results because Harnez had no existing `**` glob library. Searcher quickly confirmed lack of dependencies and located `internal/readcard/compress.go` for `go/scanner`.
- **Turn 3 (M3a - CLI assess flags & DefaultLinter)**: `neus ask` returned high-level summaries; searcher supplemented with direct targeted reads of `cmd/harnez/assess_cmd.go` and `internal/lint/lint.go`.
- **Takeaway**: `neus ask` serves well as an initial orientation check, but broad natural language questions can surface superficial keyword matches. Future prompts should test symbol-targeted queries (e.g. `neus ask "DefaultLinter Rule interface"`). We will periodically collect 1-line feedback notes from agents without interrupting the development loop.

### Milestone 4 Delivery (`5ee4ae3f`)
- **Searcher `search-986` (`haiku:med`)**: 34s, 426k tokens (incl. cached). Located `Output.md` entry in `config.yaml`, copyable doc guidelines in `docs/other/Spec.md`, and command table in `docs/CLIDesign.md`.
- **Developer `dev-986-m4` (`haiku:med`)**: 2m56s, 525k tokens (incl. cached).
  - Added Canonical Terminology rules to `.harnez/rules/Output.md` via `config.yaml` with suppression tags on quote lines.
  - Added glossary spec entry to `docs/other/Spec.md`.
  - Added `harnez assess --glossary` documentation and flag in `docs/CLIDesign.md`.
  - Ran `harnez init -d .` to synchronize root doc copies.
- **Verification & Quota-1 Pass**:
  - `TestRootDocCopiesMatchSources` passed.
  - `harnez assess --glossary` passed with **0 findings**.
  - `make test-q1` flagged golden CLI help drift (`TestHelpGolden`) due to the new `--glossary` flag (as predicted in `arch-986`'s review). Updated `cmd/harnez/testdata/help.golden` via `-update`.
  - Full `make test-q1` ran green across all 67 packages.
  - `make install` passed cleanly. Committed as `5ee4ae3f`.

---

## 7. Experiment Retrospective & Software Factory Findings

1. **Was skipping a mid/high-tier developer successful?**
   **Yes, completely.** All four milestones (M1–M4) were delivered, verified, and committed without ever needing a high-cost developer model.
2. **Key Factor: High-Information Single-Turn Architect Consultations (`argon:low`)**:
   Providing aggregated facts from the cheap Searcher to `argon:low` allowed it to output complete, production-ready types, regex algorithms, and locked table tests in ~1-minute single turns (~150k–280k tokens incl. cached).
3. **Mechanical Developers (`haiku:med`) Followed Rigid Specs Verbatim**:
   `haiku:med` excelled when given unambiguous types and locked test cases. When it encountered edge cases (such as the line-1 suppression conflict in M2), it accurately diagnosed the root cause and proposed valid options instead of silently tampering with test assertions.
4. **Session Lifecycle Policy Succeeded**:
   - Resumable Searcher (`search-986`) kept high prompt-cache efficiency across M1–M4.
   - Fresh Developer sessions per milestone (`dev-986-m1`, `dev-986-m2`, `dev-986-m3a`, `dev-986-m3b`, `dev-986-m4`) kept developer context low (<25k novel tokens), completely bypassing context fatigue and compaction degradation.
5. **No Fallback Needed**:
   Neither the Haiku underperformance nor the Argon latency circuit-breakers were triggered; the fallback to Opus was not required.

---

## 8. Next Steps for Skill Codification (`/software-factory`)

This live run on Issue #986 will measure the real cost, turn counts, and failure rates of this multi-tier pipeline. Upon completion, the following steps will be abstracted into a generic skill:
1. Automated role assignment based on task typology (Searcher -> Architect Gate -> Fresh Mechanical Dev -> Host Review).
2. Standard prompt templates for pre-flight adversarial plan review.
3. Automated test-assertion diff checking to catch assertion-relaxation anti-patterns.
