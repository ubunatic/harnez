# 986 — Per-project machine-readable glossary and harnez assess --glossary wording check

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 985 (context usage / context window terminology, first consumer), 057 (`harnez assess`), `docs/Spec.md` (spec + JSON schema conventions)

---

/goal Each harnez-managed project can keep a machine-readable glossary (terms, synonyms, forbidden terms, and
more), harnez ships its own glossary starting with the "context" terms, and a harnez check finds wording
anti-patterns in the repo and proposes rewording. Settle the discovery and research questions below first; stop
and report when the scope decision needs the owner. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## 1. Problem & Motivation

Ambiguous terms lead agents to wrong decisions. In 985, "context size", "context length" and "context used" were <!-- harnez-lint:ignore glossary -- problem statement quote -->
used for both the tokens a session currently uses and the model's maximum, which led to wrong compaction and
restart calls. `agent list` still shows `CONTEXT_USED` next to `CONTEXT_SIZE`. 985 adds the rule as prose in one <!-- harnez-lint:ignore glossary -- problem statement quote -->
ticket, but prose rules spread through tickets and docs are not checkable and do not carry over between projects.

The owner prefers checks in tool output over more instructions. A glossary that tools can read turns wording
rules into a check.

## 2. Architecture & Design (Refined 2026-10-27)

### 2.1 File Locations & Layering
- **JSON Schema**: `spec/schemas/glossary.schema.json` (Draft-07, `additionalProperties: false`, embedded via `harnez.DefaultFS`).
- **Base Glossary**: `spec/glossary.yaml` (embedded in the `harnez` binary; ships the canonical context terms from #985).
- **Per-Project Override**: `.harnez/glossary.yaml` (optional per repo). When present, `internal/glossary.LoadForRepo(dir)` validates it against `spec/schemas/glossary.schema.json` and merges it over `spec/glossary.yaml` by normalized `term` key:
  - A project entry with the same `term` overrides the base entry (or disables it via `enabled: false`).
  - `include_base: false` at the top of `.harnez/glossary.yaml` opts out of the embedded base glossary entirely.

### 2.2 Schema & Entry Structure
Each entry in `terms:` is concept-centric (ubiquitous-language style) and binds prose definitions, code identifiers, and substitution rules:
- `term` (required, string): Canonical preferred term (e.g. `"context usage"`).
- `status` (optional, enum `preferred` | `deprecated`, default `preferred`).
- `definition` (required, string): Concise 1-line explanation for humans and agents.
- `synonyms` (optional, `[]string`): Allowed alternative phrasings.
- `code_names` (optional, `[]string`): Canonical Go identifiers, config keys, or CLI column headers that map to this term (e.g. `ContextTokens`, `CONTEXT_USAGE`).
- `forbidden` (optional, `[]ForbiddenTerm`):
  - `phrase` (required, string): Disallowed wording or identifier (e.g. `"context size"`, `"CONTEXT_SIZE"`). <!-- harnez-lint:ignore glossary -- schema example of forbidden terms -->
  - `replacement` (required, string): Suggested rewording (e.g. `"context usage"` or `"CONTEXT_WINDOW"`).
  - `reason` (optional, string): Why the phrase is forbidden.
  - `case_sensitive` (optional, bool, default `false`): When `false`, matches case-insensitively on word boundaries; when `true`, matches exact casing on token boundaries.
- `scope` (optional, `{include: []string, exclude: []string}`): Path globs overriding top-level `default_scope`.
- `examples` (optional, `[]string`) and `see_also` (optional, `[]string`).

### 2.3 Scanning & Matching Rules
- **Default scope**: Scans `AGENTS.md`, `.harnez/rules/*.md`, `docs/**/*.md`, active `issues/*.md`, `spec/**/*.yaml`, `cmd/**/*.go`, and `internal/**/*.go`.
- **Default exclusions**: Skips `issues/CLOSED.md`, `issues/BACKLOG.md`, `issues/*.md` with `**Status**: Closed`, `docs/studies/**`, `docs/feedback/**`, `**/*_test.go`, `spec/glossary.yaml`, and `.harnez/glossary.yaml`.
- **Language-aware extraction**:
  - `.go`: Scans only comments and string literals via `go/scanner` (never Go syntax/identifiers).
  - `.md`: Skips fenced code blocks (` ``` ` / `~~~`) for case-insensitive prose phrases; honors `<!-- harnez-lint:ignore glossary -- reason -->` and `# harnez-lint:ignore glossary -- reason`.
- **Shared forbidden phrase deduplication**: When multiple terms forbid the same phrase (e.g. `"context size"` in both `context usage` and `context window`), the matcher emits a single finding per `file:line:col` with the combined replacement suggestion. <!-- harnez-lint:ignore glossary -- describes the glossary's own forbidden term -->

### 2.4 CLI & Agent-Facing Surfaces
- **`harnez assess --glossary [path]`**: Runs the glossary check on `path` (default `.`), printing active glossary sources, term count, and any findings (`file:line:col: forbidden term "<match>" — use <replacement> (<reason>) [glossary]`), plus `--json` support. Report-only (exit 0) by default.
- **`harnez lint` (`glossary` rule)**: Wired into `internal/lint` (`DefaultLinter` / `runLint`) so `harnez lint --check` (run by `make lint`) fails on unsuppressed forbidden terms in tracked files.
- **Agent rules delivery**: `config.yaml` / `harnez init` includes a compact 4-line terminology block in `.harnez/rules/Output.md` so agents receive the 1-line definitions and forbidden terms via the existing startup read with zero new rule files.

## 3. Discovery Resolution: Scope Decision

**Chosen scope**: Domain glossary + ubiquitous-language mapping (`terms`, `definition`, `synonyms`, `code_names`, `forbidden` with replacement/reason, `scope`), surfaced in both `harnez assess --glossary` (inspection report) and `harnez lint` (CI gate).

- **Why not a broader "doc rules" spec?** Structural doc checks already have dedicated mechanisms (`ValidateCopyableDocCatalog`, `TestRootDocCopiesMatchSources`, `MarkerIntegrityRule` in `internal/lint/marker.go`, `harnez issues lint`). Mixing prose style rules or filename conventions into the glossary schema would couple unrelated concerns (see `docs/ReviewLearnings.md` on thin vertical slices).
- **What stays out**: General English style linting (passive voice, weasel words), filename/diagram formatting rules, automatic multi-file `--fix` rewriting, and scanning closed historical tickets or case studies.

## 4. Research Resolution: Prior Art & Format Choice

- **Prose linters**:
  - **Vale** ([vale.sh](https://vale.sh/docs/topics/vocab/)) uses `accept.txt`/`reject.txt` word lists plus `extends: substitution` YAML rules (`swap: {observed: expected}`, `ignorecase: true`). Great for flat phrase swaps, but separates definitions from substitutions and has no concept of code identifiers (`code_names`).
  - **textlint + `prh`** ([textlint-rule-prh](https://github.com/textlint-rule/textlint-rule-prh)) uses a single YAML file (`expected`, `pattern`, inline `specs` test cases, and `imports`), proving the value of a single self-contained YAML dictionary with layering.
- **Agent harnesses & DDD/SKOS**:
  - `CLAUDE.md`, `AGENTS.md`, `.cursor/rules/*.mdc`, and `llms.txt` rely on uncheckable Markdown prose.
  - DDD Ubiquitous Language and SKOS (`prefLabel`, `altLabel`, `hiddenLabel`, `definition`, `notation`) organize rules around the **canonical concept** rather than isolated regexes.
- **Decision**: Author a native Harnez YAML schema (`spec/schemas/glossary.schema.json`) that combines SKOS's concept-centric structure (`term`, `definition`, `synonyms`, `code_names`) with Vale/`prh` substitution mechanics (`forbidden[].phrase` -> `replacement`), executed natively in Go without external linter dependencies.

## 5. Milestones & Verification

- **M1 — Schema, Base Glossary & Loader (`internal/glossary`)**:
  - Add `spec/schemas/glossary.schema.json` and `spec/glossary.yaml` (with the 4 terms from #985: `context usage`, `context window`, `compaction limit`, `cached re-read`).
  - Implement `internal/glossary` loader, JSON Schema validation, and `.harnez/glossary.yaml` override merging.
  - Unit tests in `internal/glossary/glossary_test.go` for schema conformance, override precedence, `include_base: false`, and invalid YAML rejection.
- **M2 — Matcher & Language-Aware Scanner (`internal/glossary` + `internal/lint`)**:
  - Implement word-boundary and `case_sensitive` matching, shared forbidden-phrase deduplication, `scope` glob filtering, closed-issue skipping (`**Status**: Closed`), Markdown fence skipping, Go comment/string-literal scanning (`go/scanner`), and `harnez-lint:ignore glossary` suppression.
  - Table tests in `internal/glossary/check_test.go`.
- **M3 — CLI Wiring (`harnez assess --glossary` + `harnez lint`) & Repo Alignment**:
  - Wire `harnez assess --glossary [--json] [path]` in `cmd/harnez/assess_cmd.go` and register the `glossary` rule in `internal/lint`.
  - Align active Harnez docs and Go comments (`docs/PiAgent.md`, `docs/AgentStatusLines.md`, `docs/JevCompaction.md`, `docs/HarnezAgentArchitecture.md`, `docs/CodexEvents.md`, `internal/subagent/claude.go`, `internal/subagent/session.go`, plus `cmd/harnez/agent_manage_cmds.go` with #985) so `harnez lint --check` passes.
- **M4 — Agent Rules Delivery & Docs**:
  - Add the compact terminology summary to `.harnez/rules/Output.md` in `config.yaml` (and sync via `harnez init -d .`).
  - Update `docs/other/Spec.md` (`docs/Spec.md`) and `docs/CLIDesign.md`.
  - Verify `make lint`, `make test-q1`, and `make install`.
