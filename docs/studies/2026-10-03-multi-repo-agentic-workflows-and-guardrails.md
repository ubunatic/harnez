# Multi-Repo Agentic Workflows & Guardrails: A Cross-Project Lean Sprint Case Study

**Date**: 2026-10-03  
**Scope**: Exploration, empirical benchmarking, cross-repo API contracts, and iterative milestone execution across `neus`, `lmcoder`, and `harnez`.  
**Starting State**: No public classification CLI in `neus`; `harnez agent --role` strictly rejecting non-exact strings; shell completion research open.  
**Delivered Outcome**: 4 tickets delivered and closed across 2 repositories (neus 041, harnez 692, 686, 695); stable `neus classify` contract; dynamic role inference and positional ergonomics in `harnez`.

---

## 1. Executive Summary

This session executed an autonomous, multi-repo engineering sequence exploring how to combine neural intent classification with strict CLI performance guardrails.

Beginning with a research investigation into whether neural search (`neus`) and local LLM serving (`lmcoder`) could provide fuzzy argument completion, parallel subagents proved empirically that neural inference (~176–285 ms) is categorically too slow for synchronous shell tab-completion (<50 ms).

Armed with this empirical boundary, the session orchestrated a sequential 4-ticket Lean Sprint across two distinct codebases (`neus` and `harnez`):
1. **Contract Definition & Fast Classifier**: Implemented `neus classify` with GBNF enum constraints on top of `lmcoder`'s resident `:8736` fast service.
2. **Deterministic Role Catalog & Aliases**: Added static alias resolution and catalog error rendering to `harnez`.
3. **Dynamic Resolver & Prompt Inference**: Cascaded unrecognised roles to `neus classify` and auto-inferred roles from prompt instructions.
4. **Positional & Fuzzy Resume Ergonomics**: Enabled positional role/name parsing and unique fuzzy substring session resumption.

The host orchestrator maintained strict zero-coding and diff-first review discipline, executing each milestone through low-cost `codex:luna:med` developer subagents guarded by read-only planning gates and Quota-1 test budgets.

---

## 2. What Worked Well

- **Parallel Specialized Exploration**: Spawning two concurrent `luna:med` research agents (`neus-lmcoder-eval` and `harnez-role-eval`) surfaced both the latency numbers (176 ms p50) and Go package boundaries (`internal/fallback` import restrictions) in under 3 minutes of wall-clock time.
- **Read-Only Plan-First Gating**: Requiring developer subagents to submit a concrete, read-only plan before granting write access caught multiple interface mismatches before a single line of code was modified (e.g. reserving `unknown` to avoid token collisions with GBNF abstention).
- **Single-Writer / Diff-First Host Review**: The Host Orchestrator never wrote code directly, keeping context consumption low and token velocity high by reviewing only `git diff HEAD~1` and verified test runs.
- **Fast Loopback Classification**: Connecting `neus classify` directly to `lmcoder`'s resident fast model (`127.0.0.1:8736`) eliminated cold-start penalties and delivered live model classifications in ~530–600 ms.
- **Clear Architectural Split**:
  - *Shell Tab-Completion (<5 ms)*: Kept 100% deterministic and static in Cobra.
  - *Invocation Time (~500 ms)*: Executed neural classification only after the user submits the command.

---

## 3. Honest Post-Mortem (Failures, Bugs & Near-Misses)

### A. Subagent Go Test Timeout in `neus` (10-Minute Hang)
- **Symptom**: During M1 of ticket 041, `neus-classify-dev` reported that `go test ./...` hit Go's 10-minute timeout.
- **Cause**: A unit test simulating HTTP timeout created an `httptest.Server` whose handler blocked indefinitely without checking request context cancellation, causing the HTTP server shutdown to hang.
- **Catch & Resolution**: The worker identified the hanging goroutine, bounded the test handler wait to 1s with context awareness, and re-verified.

### B. Developer Assertion Inversion on Model Abstention Option
- **Symptom**: `TestClassifyAmbiguousPrefixUsesModel` and `TestClassifyDeduplicatesClassesAndPrintsPlainResult` failed on initial commit `b4b582f`.
- **Cause**: The developer implemented the private `unknown` enum token for GBNF grammar abstention, but the test assertions expected `classes` in the JSON output to omit `unknown`.
- **Catch & Resolution**: The Host Orchestrator ran package tests during milestone review, spotted the assertion failure, and resumed the developer to align test expectations with the abstention schema (`dd778cc`).

### C. Quota-1 Guardrail Conflict with Legacy Test Assertions
- **Symptom**: In `harnez`, adding live `neus classify` prompt auto-inference caused two legacy CLI tests to fail because they expected exact default strings without expecting classifier notice output on stderr.
- **Catch & Resolution**: The developer isolated test suites from external machine environment by injecting an abstaining test classifier in unit tests while preserving live discovery in integration tests.

### D. Subagent Command Syntax Misunderstandings
- **Symptom**: `harnez read -L` was initially invoked by subagents as `harnez read -L <file>` instead of `harnez read -L <start:end> <file>`.
- **Catch & Resolution**: The error feedback immediately engaged the Tool Feedback Protocol (`harnez rate`), prompting self-correction on the subsequent turn.

---

## 4. Quality & Invariants Audit

| Invariant / Dimension | Status | Verification Evidence |
| :--- | :--- | :--- |
| **Module Boundaries** | **PASS** | `harnez` never imports `neus` internal packages; communication is exclusively via the stable CLI contract (`neus classify --json ...`). |
| **Determinism First** | **PASS** | Tab completion remains offline & static; exact role matches and unique prefixes bypass inference entirely (<1 ms). |
| **Zero Spurious Diffs** | **PASS** | Working trees in both `neus` and `harnez` remained clean after each milestone. |
| **Backward Compatibility** | **PASS** | Existing explicit flag invocations (`--role developer`, `--name sess`) behave identically; all existing tests pass. |
| **Test Suite Rigor** | **PASS** | `make test-q1` passing across all packages in both repositories. |

---

## 5. Efficiency & Velocity Assessment

- **Total Milestones Delivered**: 5 milestones across 4 issues (neus 041 M1/M2, harnez 692, 686, 695).
- **Subagent Turns**: Average turn duration was 1.5–3 minutes for planning, and 3–7 minutes for implementation and full Quota-1 test runs.
- **Model Efficiency**: All worker sessions ran on low-cost `codex:luna:med`, keeping token consumption minimal while reserving host capacity for high-level orchestration and review.

---

## 6. Key Learnings & Evergreen Rules

1. **The Sub-50ms Boundary Rule**:
   - *Never put neural model inference on synchronous keystroke / shell completion paths.*
   - Shell completion must always be static, cached, or local prefix/edit-distance matching. Neural reasoning belongs exclusively at command execution time.
2. **Schema-Constrained Classification with Explicit Abstention**:
   - When mapping natural language to CLI actions, always use GBNF grammars or structured schemas, and always include an explicit `unknown` token so the model can safely abstain rather than hallucinate.
3. **Multi-Repo Decoupling via CLI Contracts**:
   - Separate tools should interact via documented JSON CLI interfaces (`neus classify --json`) rather than fragile library imports or shared memory.

---

## 7. Commits Summary

### `neus`
- `9049a3b` `docs: assess neus for CLI completion`
- `f4c8eab` `docs(issues): 041, neus classify CLI`
- `b4b582f` `feat(cli): neus classify (issue 041 M1)`
- `dd778cc` `fix(cli): classify test assertions and deduplication`
- `e87514d` `docs(index): register neus-cli-completion-assessment study`
- `752a068` `docs(classify): document CLI contract`
- `2dc70b3` `docs(issues): close 041, M2: live endpoint wiring and docs`

### `harnez`
- `18a910fe` `feat(subagent): role aliases, descriptions, and resolver (issue 692 M1)`
- `69d9aa2c` `docs(issues): close 692, role descriptions, aliases, ResolveRole`
- `e5e08abc` `docs(issues): 686 (prompt role inference) & 695 (positional role/target)`
- `0190c196` `feat(agent): infer and resolve dynamic roles (issue 686)`
- `f96694eb` `docs(issues): close 686, dynamic role resolution and prompt inference`
- `0a5637e8` `feat(agent): add positional roles and fuzzy resume (issue 695)`
- `d8aef237` `docs(issues): close 695, positional role and name parsing, unique fuzzy resume selectors`
