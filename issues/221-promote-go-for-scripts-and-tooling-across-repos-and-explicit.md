# 221 — Promote Go for Scripts and Tooling Across Repos and Explicitly Demote/Disallow Ad-Hoc Python

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics / Practices
**Related**: [docs/practices/AgenticLoop.md](docs/practices/AgenticLoop.md), [docs/practices/Make.md](docs/practices/Make.md), harnez feedback entry `e7e1c940`

---

## 1. Problem & Motivation

Autonomous coding agents across managed workspaces frequently default to writing ad-hoc Python scripts (e.g. `scripts/audit.py`, `scripts/extract_foo.py`) when asked to perform data extraction, audit tasks, or write helper utilities.

In Go-centric projects (like `ubunatic.com`, `harnez`, `uman`, `lmcoder`, `voxi`), this creates recurring friction:
1. **Toolchain Fragmentation**: Python scripts introduce unmanaged virtual environments, dependency drift (`pip`, `requests`), formatting/linter divergence, and shebang/path portability bugs across developer workstations.
2. **Ecosystem Inconsistency**: The rest of the workspace and tool harnesses are pure Go with zero runtime dependencies.
3. **Loss of Static Safety & Verification**: Go scripts (`scripts/<tool>/main.go` invoked via `go run ./scripts/<tool>`) benefit from static compile-time checking, standard `go test` coverage, strict `gofmt`, and REUSE compliance without extra interpreter setup.

## 2. Proposed Rule & Guidance Updates

1. **Update Shared Harness Guidance (`docs/practices/AgenticLoop.md` / `docs/practices/Languages.md`)**:
   - Add an explicit **Language Hierarchy for Scripts & Tooling**:
     - **Primary Language for Scripts & Tools**: **Go** (`scripts/<name>/main.go` executed with `cd scripts && go run ./<name>`).
     - **Explicit Demotion of Python**: Disallow agents from creating `.py` scripts unless the user or project spec explicitly requires Python (e.g. PyTorch/ML pipelines or Python C-extension bindings).
2. **Standardize Makefile Invocations**:
   - Ensure `Makefile` conventions document `cd scripts && go run ./<tool>` as the standard pattern for repository helper tooling.
3. **Propagate via `harnez apply`**:
   - Bundle this convention into the default agent rules injected across projects during `harnez apply`.

## 3. Implementation & Verification Plan

1. Update `docs/practices/` evergreen files in `harnez` with the Go-first / Python-demoted rule.
2. Run `harnez index` to ensure all issue and doc indices remain consistent.
3. Verify that `harnez apply` propagates the rule into target workspaces.

---

## Implementation Plan

### Approach

Do **not** create a new `docs/practices/Languages.md`. Issue 130 already pushed
back on adding `default: true` docs that get baked into every project's baseline
`AGENTS.md`; a new copyable doc that nobody opts into wouldn't reach agents at
all, and one that defaults to true bloats every project. Instead put the rule in
the two places that already reach the right audiences:

- `docs/practices/AgenticLoop.md` — bundled `default: true`, so its summary
  is already in every managed project's `AGENTS.md`. A short invariant here is
  the propagation mechanism the ticket's step 3 asks for; no new config wiring.
- `docs/lang/Go.md` — the detailed "how" (layout, invocation, testing), loaded
  in Go projects, which are where the friction actually happens.

### Steps

1. **`docs/practices/AgenticLoop.md`** — add a new numbered invariant under
   `## 1. Core Philosophy & Invariants` (currently 1–6), e.g.
   *"7. Script in the Project's Own Language"*: repo-helper scripts and one-off
   audit/extraction tools are written in the project's primary language; in
   Go repos that means `scripts/<name>/main.go` run with
   `go run ./scripts/<name>`. Do not create `.py` scripts unless the user or a
   project spec explicitly asks for Python (ML/PyTorch pipelines, CPython
   bindings). Keep it to 4–6 lines — this text is summarized into every
   project's AGENTS.md, so length has a per-project token cost.
2. **`docs/practices/AgenticLoop.md`** — add one line to
   `### Anti-Patterns to Avoid`: *"❌ Ad-hoc Python in a Go repo: a throwaway
   `scripts/*.py` drags in an unmanaged interpreter, venv, and lint/format
   divergence for a task `go run ./scripts/<name>` does with static checking."*
3. **`config.yaml`** — update the `agentic-loop` doc entry's `hint:` so the
   AGENTS.md summary line mentions the Go-first scripts rule; otherwise the new
   invariant lands in the doc but not in the summary agents actually see first.
   Verify by diffing a regenerated AGENTS.md (`harnez init --dir <scratch>`)
   rather than assuming.
4. **`docs/lang/Go.md`** — add a `## Scripts & Repo Tooling` section after
   `## Project Layout`: `scripts/<tool>/main.go` layout, `go run ./scripts/<tool>`
   invocation, when a script graduates to `cmd/` or a `make` target, and the
   `go test` expectation for anything that outlives one session.
5. **`docs/lang/Make.md`** — in the existing recipe section, show the canonical
   Make invocation (`go run ./scripts/<tool> $(ARGS)`) alongside the existing
   `scripts/deploy.sh` examples so both shapes are represented.
6. **`docs/README.md`** — update the AgenticLoop and Go.md table rows' summary
   text to mention the scripts-language rule (the table is the discovery index).
7. Verify propagation: `harnez apply --docs agentic-loop` into a scratch
   `HOME`, and `harnez init --dir <tmpdir>` for a Go project, then confirm the
   rendered AGENTS.md contains the new hint line. Prefer an assertion in
   `internal/claude/init_test.go` over a manual check if an existing test
   already snapshots the Language Conventions block.
8. Run `go test ./...`, `make check`, `make install`. Then `harnez index`
   (deliberately left to the human/orchestrator — it rewrites `issues/README.md`).

### Design decisions / tradeoffs

- **No new doc, no new `--docs` name.** Adding one costs a `config.yaml` entry,
  a `docs/README.md` row, target/local paths, and a default-visibility decision,
  for content that is ~10 lines. Premature.
- **Two homes, not one.** The universal rule (don't reach for Python) belongs in
  the always-bundled practices doc; the Go-specific mechanics belong in the Go
  doc, where they don't cost non-Go projects anything.
- **Wording is "demote", not "forbid".** An absolute ban will be violated the
  first time a genuinely Python-shaped task appears and then be ignored
  generally. The escape hatch ("unless the user or project spec asks") keeps the
  rule credible.

### Risks / open questions

- Does the rule apply to *non-Go* repos too (e.g. a Zig or C++ project — Go
  script, or that project's language)? Proposed answer: "the project's primary
  language, Go as the workspace default when there isn't one." Confirm with the
  user; it changes the invariant's wording.
- AgenticLoop.md is already long and its summary is in every AGENTS.md; each
  added line has a fleet-wide token cost. Keep step 1 tight.
- Harnez feedback entry `e7e1c940` is the originating evidence — re-read it
  before wording the rule to make sure the failure mode described matches.

### Scope

**Small** — docs plus one `config.yaml` hint; no Go logic changes beyond a
possible test assertion.
