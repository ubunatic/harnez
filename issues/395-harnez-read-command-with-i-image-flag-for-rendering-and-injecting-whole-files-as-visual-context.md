# 395 — harnez read command with -I/--image flag for rendering and injecting whole files as visual context

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: CLI / Context Optimization
**Related**: [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[393-end-to-end-visual-cheatsheet-subagent-dispatch-and-mechanical-lint-compliance-canary]], [[394-multimodal-doc-delivery-architecture-and-context-optimization-roadmap]]

---

## 1. Problem & Motivation

When agents need to inspect large source files, specifications, or logs, standard text file reading tools (`view_file`, `cat`) dump entire text buffers directly into the conversational context. For large files (e.g. 500–2,000 lines), this consumes 5,000–20,000 text tokens on every subsequent turn.

With multimodal ViT encoders achieving 2.4x–8.5x token compression and 100% OCR fidelity at $\ge 9.5\text{px}$ monospace:
A dedicated `harnez read [--image|-I] <files...>` CLI command can render input files into dense, multi-column syntax-highlighted PNG images on the fly and output the image path (or pipe directly to multimodal agent tools), allowing agents to read full-file context at a fraction of the token cost.

---

## 2. Proposed CLI Interface & Behavior

```bash
# Render a source file as a temporary visual image card and print the image path / attachment:
harnez read -I internal/lint/lint.go

# Support multiple files rendered into a multi-column card or image bundle:
harnez read --image cmd/harnez/main.go cmd/harnez/apply.go

# Configure layout and resolution bounds:
harnez read -I --font-size=10 --columns=2 pkg/telemetry/stats.go

# Output to a specific destination:
harnez read -I -o /tmp/lint_view.png internal/lint/lint.go
```

### Key Technical Requirements:
1. **Syntax Highlighting & Formatting**: Clean dark/light theme rendering with line numbers, monospace font ($\ge 9.5\text{px}$), and gutter markers.
2. **Dimension Bounding**: Automatic pagination or multi-column packing keeping each image bounded within $1568\text{px} \times 1568\text{px}$ to prevent provider downsampling cliffs.
3. **Token Accounting Summary**: Output optional metadata displaying pixel geometry and estimated token cost across OpenAI, Gemini, and Claude.
4. **Fallback Mode**: Without `-I` / `--image`, act as a clean, token-aware file reader with line-range bounding.

---

## 3. Acceptance Criteria

- [ ] `harnez read` CLI command implemented under `cmd/harnez/read.go`.
- [ ] `-I` / `--image` flag renders target file(s) into styled, syntax-highlighted PNG(s).
- [ ] Multi-column layout wrapping for tall files to stay below the $1568\text{px}$ downscaling ceiling.
- [ ] Automated tests in `cmd/harnez/read_test.go` verifying image generation, exit codes, and flag handling.
- [ ] Documentation updated in `docs/CLIDesign.md` and CLI help.
