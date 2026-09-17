# 402 — Visual diff context command for git diff with text opt-out

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: CLI & Tooling / Context Optimization
**Related**: [[395-harnez-read-command-with-i-image-flag-for-rendering-and-injecting-whole-files-as-visual-context]], [[400-embed-retro-pixel-font-engine-into-internal-readcard-as-default-across-read-docs-cards-init-and-apply]], [[401-default-visual-bounded-card-output-in-harnez-find-and-harnez-issues-with-raw-opt-out]]

---

## 1. Problem & Motivation

During development sprints and pre-commit review gates (Phase 3 of AgenticLoop), reviewer subagents and orchestrators frequently inspect `git diff` or `git show` outputs. When diffs span multiple files or hundreds of lines, printing raw text diffs consumes huge amounts of token quota, risks truncation, and accelerates context window fatigue.

With the retro-pixel multimodal engine in `internal/readcard`, diffs can be rendered as crisp, 2-column or 3-column syntax-highlighted visual cards (with green additions, red deletions, and context lines). This compresses large diffs by 2x to 8x while preserving complete line and symbol fidelity.

We need to establish the canonical place/verb in the Harnez CLI for agents to invoke visual git diffs, defaulting to visual/bounded presentation with clean text opt-outs.

---

## 2. Verb & Placement Tradeoff Analysis

| Option | Invocations | Pros | Cons |
| :--- | :--- | :--- | :--- |
| **A. `harnez read --diff` / `-D`** | `harnez read -D`<br>`harnez read --diff=HEAD~1`<br>`harnez read --diff --cached` | Directly leverages `harnez read`'s existing flags (`-I`, `--columns`, `--lines`, `--font`, `--tokens`). Natural mental model for "reading a diff". | Overloads `read` flag surface slightly. |
| **B. Pipeline `git diff \| harnez read -I`** | `git diff \| harnez read -I`<br>`git diff --cached \| harnez read -I` | Zero new CLI verbs needed; pure Unix composability. Works with any git command (`git show`, `git log -p`). | Requires two sub-processes in shell instead of a single atomic tool call; agents might not remember to pipe. |
| **C. `harnez diff --git`** | `harnez diff --git [ref]`<br>`harnez diff [ref/file]` | Uses intuitive `diff` noun. | Collides with existing `harnez diff` (which checks `apply` config drift against `~/.claude`; see `docs/CLIDesign.md` on keeping global apply/diff separate from repo concerns). |
| **D. `harnez review diff` / `harnez git diff`** | `harnez review diff`<br>`harnez git diff` | Explicit domain namespace for review workflows. | Adds a new subcommand tree. |

**Recommendation**: Support **Option A (`harnez read --diff` / `-D`)** as the primary direct command, while also ensuring **Option B (`git diff | harnez read -I`)** has full unified diff syntax highlighting and auto-detection in `internal/readcard/lexer.go`.

---

## 3. Proposed Behavior & Opt-Outs

1. **Default Presentation**:
   - `harnez read --diff` (or `-D`): Inspects working tree diff against HEAD (supporting `--cached`/`--staged` or specific revisions/files).
   - In interactive / agent sessions with vision enabled, emits a dense, bounded retro-pixel PNG card (`-I` behavior by default).
2. **Opt-Out Mechanics**:
   - `--raw` / `--text`: Output raw unified diff text.
   - `--json`: Output structured JSON metadata with per-file hunks and token stats.
3. **Syntax Highlighting & Formatting**:
   - Add unified diff lexing support (`+` green, `-` red, `@@` cyan/magenta headers) to `internal/readcard/lexer.go`.
4. **Pipe / Non-TTY Auto-Fallback**:
   - When piped to a file or downstream command, default to standard text output.

---

## 4. Acceptance Criteria

- [ ] Add unified diff lexer/highlighter to `internal/readcard/lexer.go`.
- [ ] Add `--diff` / `-D` flag (and `--cached` / `--staged` support) to `harnez read`.
- [ ] Ensure piped stdin (`git diff | harnez read -I`) correctly detects diff syntax and renders visual cards.
- [ ] Provide `--raw`, `--text`, and `--json` opt-outs.
- [ ] Add unit and CLI tests in `internal/readcard/` and `cmd/harnez/read_test.go`.
- [ ] Update `AGENTS.md` and `docs/practices/AgenticLoop.md` Phase 3 (Pre-Commit Review Gate) with guidance to use visual diffs during reviews.
