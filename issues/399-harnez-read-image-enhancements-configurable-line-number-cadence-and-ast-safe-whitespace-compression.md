# 399 — harnez read image enhancements: configurable line-number cadence and AST-safe whitespace compression

**Status**: Closed — implemented and tested: line-numbers cadence and ws/ast compression verified manually and via TestCompressionPreservesSyntaxAndAnchors, TestCadenceOriginalLines
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: CLI & Tooling / Context Optimization
**Related**: [[395-harnez-read-command-with-i-image-flag-for-rendering-and-injecting-whole-files-as-visual-context]], [[394-multimodal-doc-delivery-architecture-and-context-optimization-roadmap]]

---

## 1. Problem & Motivation

When reading and rendering source files to visual image cards with `harnez read -I`:
1. **Line Number Overhead & Visual Density**: Rendering a full 4-digit line number on every single line consumes horizontal width in narrow multi-column layouts. For quick orientation, agents often only need cadence markers (e.g. every 10 or 20 lines, or milestone anchors) rather than every consecutive line.
2. **Whitespace Inefficiency**: Source files frequently contain empty lines, excessive formatting padding, or redundant indentations. Compressing whitespace naively via regex can break indentation-sensitive languages (Python, YAML) or string literals. AST-safe compression can eliminate non-semantic whitespace while strictly preserving syntax trees.

---

## 2. Proposed CLI Extensions

```bash
# Line number cadence options:
harnez read -I --line-numbers=all internal/lint/lint.go      # Every line (default)
harnez read -I --line-numbers=10 internal/lint/lint.go       # Markers at 1, 10, 20, 30...
harnez read -I --line-numbers=none internal/lint/lint.go     # No line numbers (max density)

# AST-safe whitespace compression:
harnez read -I --compress=ws internal/lint/lint.go           # Safe AST-level whitespace & blank-line stripping
harnez read -I --compress=ast internal/lint/lint.go          # AST minification preserving identifiers & semantics
```

### Key Technical Requirements:
1. **Line Number Modes**:
   - `all`: Full gutter with every line numbered.
   - `every:N` (e.g. `10`, `20`): Display line number only every $N$ lines, rendering a subtle tick mark or dot on intermediate lines.
   - `none` / `off`: Suppress line numbers completely for maximum horizontal text width.
2. **AST-Safe Compression Engine**:
   - **Go**: Use `go/parser` / `go/format` or token-stream traversal to safely compact whitespace and strip superfluous newlines without changing semantics or breaking multi-line comments.
   - **JSON / YAML**: Real AST/token parser to minify structure.
   - **Shell / Python**: Parse token boundaries to preserve string literal quotes and indentation invariants.
3. **Card Dimension Packing**: Measure line compaction gains and ViT token savings achieved by AST compression.

---

## 3. Acceptance Criteria

- [ ] Support `--line-numbers=all|off|10|20|N` in `harnez read` and `internal/readcard`.
- [ ] Implement AST-safe whitespace compression (`--compress=ws|ast`) for Go, JSON, and Shell in `internal/readcard`.
- [ ] Automated unit and CLI integration tests in `cmd/harnez/read_test.go` and `internal/readcard/`.
- [ ] Documented in CLI help and `docs/CLIDesign.md`.
