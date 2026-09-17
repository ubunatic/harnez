# 409 — Provider-adaptive output routing in harnez read and subagent dispatch

**Status**: Closed — implemented and tested in e09ea23
**Priority**: P2 (Medium)
**Severity**: Feature / Optimization
**Category**: Multi-Agent Dispatch / Context Optimization
**Related**: #394, #395, #397, #408

---

## 1. Problem Statement & Motivation

Token economics for Vision Transformer (ViT) image cards differ fundamentally across model providers:

- **OpenAI (Codex / GPT-4o)** ($512\times512\text{px}$ tiles): Achieves **2.0x to 8.5x net compression** across source files and multi-doc cheatsheets.
- **Anthropic (Claude 3.7)** ($750\text{px}^2$ area scaling): Achieves **1.5x to 3.0x net compression**.
- **Google Gemini** ($384\times384\text{px}$ tiles at 258 tokens/tile): On narrow diffs or long single-column files, tile grid ceilings can result in compression ratios $< 1.0\times$ (e.g. $0.33\times$ to $0.65\times$), meaning raw text consumes fewer tokens than the corresponding vision tiles.

Currently, commands like `harnez read -I` and subagent doc modes (`--doc-mode=vision`) generate visual PNG cards statically regardless of target model. While images provide non-token benefits (such as preventing textual self-attention fatigue and giving holistic spatial grasp of diffs/TUIs), agents running on Gemini or local SLMs would benefit from an adaptive mode that automatically chooses between visual PNG cards and token-bounded text streams based on real-time provider compression metrics.

---

## 2. Technical Scope & Architecture

### 2.1 Provider-Adaptive Routing Mode (`--auto`)
1. **In `harnez read`**:
   - Add `--auto` (or `--mode=auto`) routing flag.
   - Query `internal/readcard/tokens.go` for the estimated compression ratio against the active agent provider.
   - **Routing Heuristic**:
     - If active provider compression $\ge 1.0\times$ (e.g. Claude or OpenAI on multi-column bundles/diffs): render and output styled visual PNG card (`-I`).
     - If active provider compression $< 1.0\times$ (e.g. Gemini on narrow linear code without complex tables): output line-bounded text (`-n` / `-L`).
2. **In Subagent Dispatch (`harnez subagent --doc-mode=auto`)**:
   - For Claude / Codex: attach 3-in-1 visual developer card (`docs/cards/bundle.png`).
   - For Gemini / Local SLMs: inject concise lite text rules (`*.lite.md`) or targeted rule slices.

### 2.2 Active Harness Detection
- Resolve active agent identity via `internal/resolve` or environment markers (`HARNEZ_AGENT_HARNESS`, `CLAUDE_CODE`, `GEMINI_CLI`, `CODEX_CLI`).
- Default to conservative provider profiles when running in standalone CLI terminals.

### 2.3 Preserving Explicit Overrides
- Explicit flags (`-I` / `--image`, `--text`, `--raw`, `-n`) must always override adaptive heuristics, allowing operators to force visual cards when visual spatial layout and syntax coloring are explicitly desired.

---

## 3. Acceptance Criteria

- [ ] Implement `--auto` in `harnez read` and `--doc-mode=auto` in subagent dispatch.
- [ ] Adaptive decision engine uses real-time provider token calculations in `internal/readcard/tokens.go`.
- [ ] Explicit flags (`-I`, `--text`, `-n`) strictly override adaptive defaults.
- [ ] Unit tests in `internal/readcard/` and `internal/subagent/` verifying routing decisions across simulated Claude, OpenAI, and Gemini profiles.
- [ ] Documented in `docs/MultimodalContextDelivery.md` and CLI help.
