# 398 — Trial ultra-compact pixel and bitmap fonts from retro games for extreme ViT token compression

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Low
**Category**: Research & Canary / Context Optimization
**Related**: [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[391-vision-transformer-font-resolution-and-ocr-breakdown-threshold-probe]], [[392-multi-doc-bounded-developer-cheatsheet-card-3-in-1-packed-below-1568px-ceiling]], [[394-multimodal-doc-delivery-architecture-and-context-optimization-roadmap]]

---

## 1. Problem & Motivation

Our prior font resolution probe (Issue 391) demonstrated that vector monospace fonts (e.g. JetBrains Mono, Fira Code) lose OCR accuracy below $9\text{px}$ due to **anti-aliasing fuzziness, subpixel hinting blur, and gray-bleed**, which softens critical punctuation like `{`, `[`, `:=`, and `!=`.

In retro games (PICO-8, Game Boy, Commodore 64, ZX Spectrum, micro-emulators), **pixel/bitmap fonts** (e.g., $3\times5$, $4\times6$, $5\times8$, $6\times8$ grids) are engineered specifically for crisp legibility on $256\times256\text{px}$ or smaller viewports with **pure binary contrast (1-bit on/off pixels, zero anti-aliasing blur)**.

If rendered with crisp nearest-neighbor rasterization or aligned directly with ViT patch grids (e.g., $14\times14\text{px}$ ViT patches in CLIP/SigLIP/Gemini/Claude), pixel fonts could potentially allow packing massive text corpora into a tiny $256\times256\text{px}$ or $512\times512\text{px}$ image canvas, unlocking **15x–30x multimodal token compression**.

---

## 2. Research Hypothesis & Target Fonts

### Target Pixel & Micro-Bitmap Fonts:
- **PICO-8 Font** ($3\times5$ / $4\times6$ pixel glyphs)
- **Tom-Thumb** ($3\times5$ extreme micro-font)
- **Spleen 5x8 / 6x12** (ultra-dense BSD console bitmap font)
- **Cozette** ($6\times13$ bitmap font with broad symbol/Nerd font coverage)
- **Unscii** ($8\times8$ classic pixel art font)
- **Proggy Tiny / Micro** ($5\times7$ programming pixel font)

### Research Questions:
1. **Zero-AA Legibility**: Does pure 1-bit crisp contrast without anti-aliasing allow ViT vision encoders to resolve $5\text{px}$–$7\text{px}$ characters that fail under vector rendering?
2. **ViT Patch Alignment**: Does integer-scaling a pixel font (e.g., $2\times$ or $3\times$ nearest-neighbor scaling) yield better OCR accuracy per token than fractional vector downscaling?
3. **Punctuation Disambiguation**: Can micro-pixel fonts reliably distinguish code syntax tokens (`{` vs `[`, `:=` vs `!=`, `_` vs `-`, `1` vs `l` vs `|`) across Claude, Gemini, and OpenAI vision models?
4. **Canvas Compression Ceiling**: Can an entire multi-file codebase or 5,000-word documentation manual fit into a single $256\times256\text{px}$ ($1 \times 1$ tile = 85–258 tokens) or $512\times512\text{px}$ image with 100% mechanical recovery?

---

## 3. Planned Canary Architecture

1. Create test harness `scripts/canary-pixel-fonts/` with:
   - Pixel font rasterizer embedding target bitmap TTF/BDF/PNG font atlases.
   - Nearest-neighbor / `image-rendering: pixelated` renderer.
   - Font probe test cards testing code snippets, syntax glyphs, and multi-line markdown.
2. Run OCR recovery probes across OpenAI (Codex / GPT-4o), Google Gemini 2.0 / Flash, and Anthropic Claude 3.7.
3. Compute exact token savings and error rates for $256\times256\text{px}$, $384\times384\text{px}$, and $512\times512\text{px}$ canvases.
4. Record findings and recommendations in `docs/studies/`.

---

## 4. Acceptance Criteria

- [ ] Canary test scripts in `scripts/canary-pixel-fonts/` evaluating candidate retro pixel fonts.
- [ ] OCR accuracy and syntax glyph benchmark across Claude, Gemini, and OpenAI at micro resolutions ($256\times256\text{px}$ to $512\times512\text{px}$).
- [ ] Documented study report in `docs/studies/` with compression ratios, visual samples, and recommendations.
