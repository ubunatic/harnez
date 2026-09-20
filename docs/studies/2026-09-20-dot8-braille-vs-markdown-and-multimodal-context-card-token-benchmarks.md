# Dot8 Braille vs. Markdown and Multimodal Context Card Token Benchmarks

**Date**: 2026-09-20  
**Status**: Completed  
**Harness Evaluated**: Google Antigravity (`agy`), Harnez Telemetry Hooks (`tool_catalog.sqlite`), `harnez read -I` Vision Engine  

---

## 1. Executive Summary

This study documents empirical benchmarks comparing token efficiency, latency, and representation tradeoffs across four distinct document delivery formats:
1. **Plain Markdown Text** (`.md`)
2. **Braille 8 Plain Text** (`.dot8` / `.braille.md`)
3. **Visual PNG Context Cards — Markdown** (`harnez read -I .md`)
4. **Visual PNG Context Cards — Braille 8** (`harnez read -I .dot8`)

Three real project documentation files of varying length and structural density were measured under live harness telemetry:
- [`docs/ConciseMode.md`](../ConciseMode.md) (64 lines, prose-heavy)
- [`docs/Canary.md`](../Canary.md) (192 lines, mixed prose and code blocks)
- [`docs/CodexHooks.md`](../CodexHooks.md) (150 lines, dense JSON schemas and code syntax)

---

## 2. Empirical Benchmark Data

All operations were executed sequentially in live agent turns and captured via the Harnez Antigravity hook into `~/.harnez/tool_catalog.sqlite`.

### 2.1 Complete Token Comparison Matrix

| Target Document | Lines / Words | 1. Plain Markdown Text (`view_file .md`) | 2. Braille 8 Text (`view_file .dot8`) | 3. Visual PNG Card: Markdown (`harnez read -I .md`) | 4. Visual PNG Card: Braille 8 (`harnez read -I .dot8`) |
|---|---|---|---|---|---|
| **`ConciseMode`** | 64 lines / 330 w | **47 tokens** | **694 tokens** | **752 tokens** *(1552×348 px)* | **1,916 tokens** *(1552×348 px)* |
| **`Canary`** | 192 lines / 995 w | **958 tokens** | **2,126 tokens** | **2,077 tokens** *(1552×828 px)* | **5,226 tokens** *(1552×838 px)* |
| **`CodexHooks`** | 150 lines / 635 w | **2,444 tokens** | **5,597 tokens** | **1,346 tokens** *(1552×628 px)* | **3,323 tokens** *(1552×638 px)* |
| **Total (3 Files)** | **406 lines** | **3,449 tokens** | **8,417 tokens** | **4,175 tokens** | **10,465 tokens** |

---

## 3. Telemetry & Execution Findings

### 3.1 Live Tool Call Latency & Output Bytes

| Call ID | Tool | Format | Target | Duration | Output Payload | Recorded `actual_tokens` |
|---|---|---|---|---|---|---|
| **25073** | `view_file` | Text | `ConciseMode.braille.md` | 65 ms | 2,599 bytes | 694 |
| **25074** | `view_file` | Text | `Canary.braille.md` | 43 ms | 7,972 bytes | 2,126 |
| **25075** | `view_file` | Text | `CodexHooks.braille.md` | 37 ms | 20,986 bytes | 5,597 |
| **25076** | `view_file` | Text | `ConciseMode.md` | 45 ms | 176 bytes | 47 |
| **25077** | `view_file` | Text | `Canary.md` | 43 ms | 3,591 bytes | 958 |
| **25078** | `view_file` | Text | `CodexHooks.md` | 39 ms | 9,163 bytes | 2,444 |

Average raw read duration across all calls was **45.3 ms**, with zero RPC failures.

---

## 4. Architectural Analysis & Tokenizer Mechanics

### 4.1 Why Braille 8 Text Expands Token Volume (~2.44×)
- **BPE Subword Merges**: Modern LLM tokenizers (tiktoken, SentencePiece, Gemini vocabulary) contain tens of thousands of common English word and subword merges (e.g. `development`, `mechanism`, `PreToolUse`, `permissionDecision`).
- **Braille Unicode Disjointness**: Unicode Braille patterns (`U+2800`–`U+28FF`) are absent from primary subword merges. Each 8-dot Braille character encodes as a 3-byte UTF-8 sequence and is split into individual single-character or byte-fallback tokens.
- **Outcome**: A file containing 1,000 English words expands from ~958 tokens to 2,126 tokens when encoded into Braille 8.

### 4.2 Visual PNG Cards: Code Density vs. Prose Density
- **Syntax/JSON-Dense Documents** ([`CodexHooks.md`](../CodexHooks.md)):
  - Raw Text: **2,444 tokens** (JSON brackets, quotes, indentation, field keys).
  - Visual PNG Card: **1,346 vision tokens** (rendered in 3 columns at 1552×628 px).
  - **Net Savings: 44.9% token reduction** through spatial 2D packing.
- **Short Prose Documents** ([`ConciseMode.md`](../ConciseMode.md)):
  - Raw Text: **47 tokens**.
  - Visual PNG Card: **752 vision tokens** (minimum tile grid threshold).
  - **Takeaway**: Short prose files (<100 lines) are cheaper in text, while dense structured specifications and multi-file listings save significant tokens as visual PNG cards.

### 4.3 Visual Braille Rendering
- Visual Braille cards rendered cleanly using the 5x8 retro pixel font engine.
- However, because Braille dot patterns require discrete dot spacing to remain legible, rendered Braille cards required ~**2.5× more vision tokens** (10,465 vs. 4,175 tokens) than visual Markdown cards.

---

## 5. Conclusions & Recommendations

1. **Retain Braille 8 for Lossless Obfuscation & Compact Binary Encoding**:
   - Braille 8 (`.dot8`) provides strict lossless 1:1 round-trip encoding and compact visual representation, but should not be used as the primary LLM input text format due to tokenizer character-splitting penalties.
2. **Use `harnez read -I` on Structured/Large Files**:
   - For complex schemas, multi-file ASTs, and large docs (>100 lines), visual PNG cards provide significant token compression (~40–50% savings on code-heavy content) and preserve 2D layout semantics.
3. **Keep Short Prose in Direct Text / Concise Mode**:
   - Short markdown docs (<100 lines) operate at peak token efficiency as plain text under ConciseMode.
