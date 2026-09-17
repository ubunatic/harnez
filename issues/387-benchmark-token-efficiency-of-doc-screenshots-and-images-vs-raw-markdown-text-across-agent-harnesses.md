# 387 — Benchmark token efficiency of doc screenshots and images vs raw markdown text across agent harnesses

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Research & Canary / Context Optimization
**Related**: [[385-tokens-command-to-count-tokens-in-files-and-directories]], [[386-strip-eager-global-doc-includes-from-global-claude-md-template]]

---

## 1. Problem & Motivation

In current multi-agent workflows, instruction and language documentation files (`docs/*.md`, `AGENTS.md`) are fed into LLM context as raw markdown text. Text tokenization scales roughly linearly with character count (~1.3 tokens per word, or ~3.75 chars/token). A rich repository with 5–8 bundled docs can easily consume 15,000–30,000 tokens on every invocation.

However, modern multimodal models (Gemini, GPT-4o/Codex, Claude 3.5/3.7) process images using **tiled patch encoders**:
- **Google Gemini**: ~258 tokens per fixed image/tile grid.
- **OpenAI (GPT-4o / Codex)**: 85 base + 170 tokens per 512×512 tile.
- **Anthropic Claude**: $(W \times H) / 750$ tokens.

Because vision token costs are strictly bounded by image geometry rather than text density, dense information layouts (e.g. 2-column or 3-column newspaper/cheatsheet layouts, compact 10–12px monospace typography, high-contrast syntax themes) could theoretically compress 3,000–5,000 tokens of raw markdown into **258 – 1,100 vision tokens**—yielding a 3x to 15x token reduction.

## 2. Experimental Scope & Canary Design

### 2.1 Canary Doc Selection
Pick a representative, non-ubiquitous doc containing precise technical invariants, Unicode glyphs, and syntax (e.g. `docs/TUIDesign.md` or `docs/MacOSPortability.md`).

### 2.2 Rendering Matrix
Develop a lightweight rendering script (using headless Chromium / Python) to produce variations:
1. **Layout**:
   - Standard 1-column page (1080p / 14px).
   - Compact 2-column cheatsheet grid (11px, high-contrast).
   - Ultra-dense 3-column micro-grid (10px monospace).
2. **Typography**:
   - Monospace (JetBrains Mono / Fira Code) vs Clean Sans (Inter / Roboto).
3. **Image Format & Compression**:
   - Crisp PNG (lossless text edges) vs high-quality JPG / WebP.

### 2.3 Evaluation Matrix across Harnesses
Evaluate across the three active agent CLIs (`agy`, `codex`, `claude`):
1. **Token Consumption**: Measure reported context consumption (`/context` / API usage metadata) of raw markdown vs each image variation.
2. **Fidelity & Instruction Following (OCR Probe)**: Probe agents with 5–10 targeted questions requiring exact string matches, edge-case flags, and Unicode glyph comprehension to verify zero OCR hallucination.
3. **Prompt Caching Economics**: Compare vision token cost vs cached prompt token cost.

## 3. Acceptance Criteria

- [ ] Automated rendering harness script in `scripts/` or `scratch/` generating standard, compact, and dense image variants from markdown.
- [ ] Empirical token benchmark table across `agy`, `codex`, and `claude`.
- [ ] Accuracy/fidelity evaluation report documenting OCR error rates and edge-case syntax retention.
- [ ] Synthesis report in `docs/studies/` recommending whether vision-based doc injection is viable for specific doc types.
