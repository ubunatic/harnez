# 041 — Sibling Projects Audit: Real-World Shapes, Drift Causes & Test Suite Extensions

**Status**: Closed  
**Category**: Quality Assurance / Test Coverage / Multi-Project Ergonomics  
**Related**: [009 — Diff Clean No Makefile Targets](009-diff-clean-no-makefile-targets.md), [011 — Auto-detect Non-deterministic Order](011-autodetect-nondeterministic-order.md), [018 — Mark Bundled Docs in Frontmatter](018-mark-bundled-docs-in-frontmatter.md), [039 — Agentic Loop Practices](039-agentic-loop-practices-and-sprint-command.md), `docs/CLIDesign.md`

---

## 1. Context & Motivation

An audit of 27 sibling repositories located in the workspace (`~/projects/`) was performed to investigate their actual filesystem setups, harness states (`AGENTS.md`, `CLAUDE.md`, `./docs/`, `Makefile`), and determine:
1. Why recent `harnez` updates (such as `docs/practices/AgenticLoop.md`, the `/sprint` command, and context discipline rules) have not automatically reached sibling projects.
2. How real-world repositories differ from the idealized fixtures currently tested in `internal/claude/*_test.go` and `internal/markdown/*_test.go`.
3. Concrete, high-value proposals for extending the `harnez` test suite to cover real-world variations without overblowing test complexity.

---

## 2. Sibling Repository Audit & Landscape

### 2.1 Repository Inventory & State Matrix

| Project | Primary Stack / Languages | `AGENTS.md` State | `CLAUDE.md` | `./docs/` Bundled Copies | Build / Makefile Setup |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`cati`** | Go, HTML/JS, mdBook | `harnez:begin` (Language Conventions, Project Summary, Spec & Pixel Art) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`voxi`** | Rust, Go, Python/GTK4, Shell | `harnez:begin` (Project Summary, Language Conventions, Repo Setup: solo) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`uman`** | Go | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`goha`** | Go, Bash, Cgo | `harnez:begin` (Heavy custom working agreement + Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`mdview`** | Go | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`proctop`** | Go | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`psync`** | Go, Shell | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`wayreel`** | Go, Wayland/Shell | `harnez:begin` (Version-gated canary rules, Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`ziggo`** | Zig, Go | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Zig.md`, `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`uzu`** | Zig, Go | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Zig.md`, `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`zterm`** | Zig | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Zig.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | **No Makefile** (`build.zig` only) |
| **`vkfusion`** | C++, Vulkan, Go | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` & extra `.PHONY` lines |
| **`conreel`** | Rust | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Rust.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`emojig`** | Go, Zig, Rust, C, Shell | `harnez:begin` (Extensive preamble, Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Rust.md`, `Zig.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`fontwidth`** | Go, C, Zig, Python | `harnez:begin` (Server port rules, Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`comfyconf`** | Python, Bash | `harnez:begin` (Project Summary, Language Conventions) | Symlink -> `AGENTS.md` | `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`pdf-doctor`**| Python, Bash | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`spriteview`**| Go (ebiten) | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`ubunatic.com`**| Hugo, Web, JS | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`vimconf`** | Vimscript, Lua | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Go.md`, `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙ 🤖` (text glyph variant) |
| **`books`** | Markdown, mdBook, Shell | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Make.md`, `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`diagnose`** | Shell | `harnez:begin` (Language Conventions) | Symlink -> `AGENTS.md` | `Bash.md`, `Markdown.md`, `Git.md`, `Canary.md`, `Spec.md` | `Makefile` with `.PHONY: ⚙️ 🤖` |
| **`homeserver`**| Go, Taskfile, Shell | **Unmanaged custom AGENTS.md** (No markers) | Symlink -> `AGENTS.md` | Custom `docs/coding/*`, `docs/agents/*` | `Taskfile.yml` + `Makefile` (`.PHONY: ⚙️` only) |
| **`mindstore`** | Notes only | **No AGENTS.md** (only `HUMAN.md`) | None | None | No Makefile |
| **`rocmctl`** | Bare directory | **No AGENTS.md** | None | Only single investigation note | No Makefile |
| **`archive/qrscan`**| Archived project | **Legacy markers** (`<!-- claudeconfig:begin ... -->`) | Regular / link | Legacy docs | Makefile |

---

## 3. Analysis: Why Recent Changes Haven't Been Applied

1. **Architectural Separation of Concerns (`apply` vs `init`)**:
   - As designed in `docs/CLIDesign.md`, `harnez apply` operates exclusively on **global harness environments** (`~/.claude`, `~/.prime/agent`, `~/.gemini/skills`, `~/.codex/skills`).
   - Sibling repositories are intentionally isolated and never mutated by `apply`.
   - Propagating new docs (e.g. `AgenticLoop.md`, `Spec.md`, `Canary.md`), updated markers, or new convention snippets requires running `harnez init` within each project directory.

2. **Lagging Propagation of New Docs & Commands**:
   - `AgenticLoop.md` is bundled into global paths on `harnez apply`, but currently **0 of 27 sibling projects** have received `AgenticLoop.md` in their local `./docs/` or their local `Language Conventions` block.
   - The `/sprint` command is available to agents globally, but local project `AGENTS.md` files do not mention it yet (only `harnez/AGENTS.md` does).

3. **Absence of Workspace Drift Visibility**:
   - `harnez status` currently only inspects the global targets (`~/.claude`, `~/.prime/agent`, etc.).
   - While `make init-siblings` exists in `harnez/Makefile` and `uman harness-sync` exists in root `~/.uman.toml`, there is no dry-run drift reporter (e.g. `harnez status --workspace`) to notify developers when sibling projects have fallen behind the central `harnez` spec.

---

## 4. Real-World Discrepancies vs. Current Test Fixtures

Current tests in `internal/claude/*_test.go` and `internal/markdown/*_test.go` use simplified, clean temp directories. Comparing them against real sibling repositories reveals significant gaps:

| Real-World Scenario | Observed in Sibling Repos | Gap in Current Test Suite |
| :--- | :--- | :--- |
| **Unmanaged Custom `AGENTS.md`** | `homeserver` has a completely bespoke, handwritten `AGENTS.md` with no section markers. | Tests only check files that are missing or already have valid markers. Tests do not verify how `init` injects markers into an unmanaged file without overwriting custom prose. |
| **Polyglot Multi-Language Stacks** | `emojig`, `fontwidth`, `voxi` have Go + Zig + Rust + C + Python + Shell simultaneously. | `docs_test.go` tests single detectors in isolation (`detectDoc(dir, "zig")`). It does not assert multi-language combinations, ordering stability, or de-duplication across 4+ simultaneous matchers. |
| **Projects Without Makefiles** | `zterm` uses `build.zig` exclusively; `homeserver` uses `Taskfile.yml`. | Tests assume Makefile presence or template generation. Tests should guarantee that `init` does not unexpectedly inject a `Makefile` when another build tool is used. |
| **Makefile Phony Sentinel Variants** | `vimconf` uses `⚙` (U+2699); `vkfusion` has multiple `.PHONY` lines; `homeserver` has `.PHONY: ⚙️` only. | Tests in `maketargets.go` test basic insertion, but do not test normalization of alternate Unicode glyph variants or multi-line `.PHONY` declarations. |
| **Legacy Markers with Rich Custom Content** | `archive/qrscan` and older projects have `claudeconfig` markers surrounded by custom team agreements. | Tests test `markdown.Apply` on clean strings, but do not verify that `migrateLegacyMarkers` + `buildLangConventions` preserves preceding and trailing custom markdown without formatting regressions. |
| **Bare / Non-Git Directories** | `mindstore`, `rocmctl` reside in `~/projects/` but lack git/harness files. | Batch sync scripts (`init-siblings`) are not tested against bare directories to ensure graceful skips. |

---

## 5. Proposals for Extending the Test Suite

To capture these real-world scenarios while maintaining fast execution (< 0.5s) and zero dependency overhead, the following table-driven test additions are proposed:

### 5.1 Polyglot Auto-Detection & Ordering Matrix (`internal/claude/docs_test.go`)
Add a table-driven test fixture verifying multi-stack detection and deterministic sorting:
- **Fixture 1**: `go.mod` + `build.zig` + `main.c` + `scripts/run.sh` + `Makefile` -> `[golang, bash, make, zig, cpp, markdown, git, canary, spec, agentic-loop]`
- **Fixture 2**: `Cargo.toml` + `build.zig.zon` -> `[rust, zig, markdown, git, canary, spec, agentic-loop]`
- **Fixture 3**: `requirements.txt` + `setup.py` + `Taskfile.yml` -> `[markdown, git, canary, spec, agentic-loop]`

### 5.2 Legacy Marker Migration & Pre-existing Content Preservation (`internal/claude/init_test.go`)
Test `RunInit` against an `AGENTS.md` containing:
- Custom human working agreements and role definitions (e.g. modeling `goha` and `cati`).
- Legacy `<!-- claudeconfig:begin Language Conventions -->` markers.
- Custom downstream sections (e.g. `## Pixel Art Guidelines`).
- **Assertion**: Running `RunInit` migrates markers to `<!-- harnez:begin ... -->`, updates the conventions list cleanly, and preserves 100% of surrounding custom sections byte-for-byte.

### 5.3 Non-Makefile Project Safety Test (`internal/claude/init_test.go`)
- **Setup**: Temp directory with `build.zig` and `src/main.zig` (modeling `zterm`).
- **Execution**: Run `RunInit` with auto-detection.
- **Assertion**: `AGENTS.md` and `docs/Zig.md` are initialized, but **no** `Makefile` is created unless explicitly configured.

### 5.4 Makefile Reconciliation Matrix (`internal/claude/maketargets_test.go`)
Test `ReconcileMakeTargets` against:
1. Makefile with text presentation glyph `.PHONY: ⚙ 🤖` (modeling `vimconf`).
2. Makefile with multiple separate `.PHONY` lines (modeling `vkfusion`).
3. Makefile with existing user-defined `help:` target.
- **Assertion**: Proper sentinel normalization according to `MakeConfig.PhonyFix`, idempotent injection of the `help: 🤖` target, and zero duplication of user targets.

### 5.5 Multi-Project Batch Initialization Test (`internal/claude/integration_test.go`)
- **Setup**: A sandbox containing mock subdirectories for a Go service, a Zig CLI, a Rust library, an unmanaged custom repo, and a bare non-git folder.
- **Execution**: Run project initialization across all subdirectories.
- **Assertion**:
  - Valid projects are upgraded and receive auto-detected docs cleanly.
  - Custom unmanaged markdown in `homeserver`-style setups is preserved.
  - Bare non-git folders are safely skipped without error.
  - A consecutive run produces zero changes (idempotency).

---

## 6. Architecture & Scope Principle: Zero-Feature-Bloat Guarantee

A critical architectural finding from this audit: **Extending the test suite does not require or imply adding new features, dependencies, or complexity.**

1. **The Core Engine Already Implements the Correct Logic**:
   - `autoDetectDocs` already processes language detectors in deterministic priority order from `config.yaml`.
   - `markdown.Apply` already replaces bounded marker blocks without disturbing outer custom prose.
   - `init` already guards Makefile scaffolding to ensure non-Make projects (e.g. Zig, Python) remain untouched unless requested.
2. **Tests as Pure Safety Nets & Characterization**:
   - The proposed table-driven test cases serve purely to characterize and lock down existing behavior against realistic permutations.
   - They run in milliseconds, require zero mocks or external daemons, and prevent subtle regressions when modifying templates or marker logic in the future.
3. **Preserving Minimalist Design**:
   - We explicitly avoid heavy AST parsers, LSP tooling, or background multi-repo sync daemons.
   - Sibling project independence is preserved: `apply` remains global-only, `init` remains project-local, and simple string/marker operations remain the source of truth.

