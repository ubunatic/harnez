---
title: Multimodal Context Delivery Architecture & Context Optimization Roadmap
weight: 35
---

# Multimodal Context Delivery Architecture & Context Optimization Roadmap

## 1. Executive Summary

As multi-agent systems and nested subagent loops become standard in daily development, context window consumption and token economics dominate both latency and monetary cost. Traditionally, repository instruction documents (`docs/practices/AgenticLoop.md`, `docs/lang/*.md`, `AGENTS.md`) were inlined as raw text or eagerly dumped into context, consuming 15,000–30,000 tokens on every agent turn.

With the convergence of five architectural milestones:
1. **Zero Global Doc Inlining** (Issue 386 & Issue 389) making global doc installation strictly opt-in,
2. **Unified Cross-Harness Dynamic Skills** (Issue 388) across `claude`, `agy`, `codex`, and `prime`,
3. **Multi-Column Visual Cheatsheets & ViT Bounded Cards** (Issue 387, Issue 391, Issue 392) offering 2x–8.5x token compression,
4. **Isolated Subagent Dispatch & Mechanical Lint Canary** (Issue 393) proving 100% mechanical instruction compliance with zero text rules,
5. **Native 1-Bit Retro Pixel Font Engine** (Issue 398 & Issue 400) eliminating anti-aliasing gray-bleed and unlocking up to 10.5x compression on micro-canvases,

Harnez adopts a formal **4-Tier Context Delivery Architecture**.

```mermaid
flowchart TD
    subgraph Tiers["Harnez 4-Tier Context Delivery Architecture"]
        T1["Tier 1: Global Behavioral Glue<br/>(CLAUDE.md / AGENTS.md, Zero Doc Inlining, 90%+ Prefix Cache)"]
        T2["Tier 2: Universal Autonomous Capabilities<br/>(~/.&lt;harness&gt;/skills/ with Progressive Disclosure Metadata)"]
        T3["Tier 3: Bounded Visual Cheatsheets<br/>(Rendered Image Cards, Retro Pixel Fonts, Subagents)"]
        T4["Tier 4: Project-Local Evergreen Rules<br/>(./AGENTS.md & ./docs/ via harnez init, Bounded Grep/Range Reads)"]
    end

    T1 -->|Static Prefix Cache| Agent[Active Agent / Subagent Context]
    T2 -->|On-Demand Skill Invocation| Agent
    T3 -->|Multimodal Image Dispatch| Agent
    T4 -->|Targeted Range-Bounded Ingestion| Agent
```

---

## 2. The 4-Tier Context Delivery Architecture

### Tier 1: Global Behavioral Glue (Prefix Cached & Minimal)
- **Scope**: User global config (`~/.claude/CLAUDE.md`, `~/.codex/`, `~/.gemini/antigravity-cli/`, `~/.prime/`).
- **Payload**: Minimal, immutable operating invariants (e.g. Zero Zombie Guarantee, Quota-1 single-test boundary, single default branch rule).
- **Rule**: **Zero Doc Inlining**. No language or practice markdown files are eagerly concatenated into global context.
- **Economic Objective**: Maximize prompt prefix cache hits (>90% cache efficiency). Any mutation in Tier 1 invalidates the global prefix cache across all sessions.

### Tier 2: Universal Autonomous Capabilities (Dynamic Skills)
- **Scope**: Cross-harness skill definitions (`~/.claude/skills/`, `~/.gemini/skills/`, `~/.codex/skills/`, `~/.prime/agent/skills/`).
- **Payload**: Standardized skill directories containing `SKILL.md`, tools, and reference scripts.
- **Progressive Disclosure Pipeline**:
  1. *Level 1 (Discovery)*: Light description and intent trigger in the tool index (~20–40 tokens).
  2. *Level 2 (Activation)*: Targeted `SKILL.md` instructions ingested into context only when triggered.
  3. *Level 3 (Execution)*: Auxiliary scripts and tools execute in background sub-processes without dumping raw code into context.

### Tier 3: Bounded Visual Cheatsheets (Multimodal Compression)
- **Scope**: High-density rendered image cards (`docs/vision/`, `harnez read -I`, `harnez docs cards`).
- **Payload**: Styled 2-column or 3-column cheatsheets and 3-in-1 developer cards (`Bash` + `Make` + `Git`) rendered using 1-bit crisp bitmap fonts (`Font5x8` default, `Font3x5` micro).
- **ViT Invariants**:
  - Max dimension kept $\le 1568\text{px}$ (preventing downscaling blur on Claude and OpenAI).
  - Tiled ViT patch encoders compress raw markdown from **6,473 text tokens** down to **765–1,032 vision tokens** (**6.3x to 8.5x net reduction**).
  - Used for isolated subagent dispatches, `harnez read -I` file inspections, and on-demand reference lookups.

### Tier 4: Project-Local Evergreen Rules (Repository-Scoped)
- **Scope**: Repository root (`./AGENTS.md`, `./CLAUDE.md`, `./docs/`).
- **Payload**: Project-specific architectural invariants, domain rules, and local overlays (`AGENTS.local.md`).
- **Ingestion Discipline**:
  - Invariant 6 (Range-Bounded Ingestion): No whole-file dumping of `AGENTS.md` or large architecture specs.
  - Subagents and orchestrators query docs via targeted tools (`view_file` slices, `grep_search`, `harnez find`, `harnez read`).

---

## 3. Provider ViT Economics & Resolution Matrix

| Model / Harness | Vision Token Formula | 3-in-1 Card Tokens *(1440×1400px)* | Typical File (70–150 lines, 3-col) | Raw Text Equiv | Compression Ratio | Downsampling Bound |
| :--- | :--- | :---: | :---: | :---: | :---: | :--- |
| **OpenAI / Codex** | $85 + 170 \times \lceil W/512 \rceil \times \lceil H/512 \rceil$ | **765** *(3×3 tiles)* | **765** *(3×1 tiles, H ≤ 512px)* | ~1,514 | **~2.0x – 8.5x** | Max 2048px |
| **Google Gemini** | $258 \times \lceil W/384 \rceil \times \lceil H/384 \rceil$ | **1,032** *(4×4 tiles)* | **2,580** *(5×2 tiles)* | ~1,514 | Flat per tile | Patch 384px |
| **Anthropic Claude**| $\approx (W \times H) / 750$ | **2,688** | **1,010** *(1520×498px)* | ~1,514 | **~1.5x – 2.4x** | Max 1568px |

### 3.1 Provider Architectural Nuances & Routing
- **OpenAI Step-Function Tiles ($512\times 512\text{px}$)**: OpenAI costs jump in discrete $170$-token increments per $512\times 512$ tile. Packing height below $512\text{px}$ via 3 columns (`--columns=3`) keeps the vertical tile count at 1, halving the token cost compared to tall 1-column cards ($H > 512\text{px}$).
- **Anthropic Claude Continuous Area Integration ($(W \times H)/750$)**: Claude tokenization scales smoothly with total pixel area. Dynamic canvas cropping and tight column bounding boxes directly reduce Claude token burn.
- **Google Gemini Grid Tiles ($384\times 384\text{px}$)**: Gemini evaluates $258$ tokens per $384\text{px}$ patch. For short micro-snippets (<50 lines), compact 1-column layouts or raw text routing are preferred over wide multi-column canvases.

### 3.2 ViT Resolution & Rasterization Rules
- **Vector Anti-Aliased Fonts**: Below 9px, anti-aliasing gray-bleed degrades `{` vs `[`, `:=` vs `!=`.
- **Pure 1-Bit Retro Pixel Fonts (`Font5x8`, `Font3x5`)**: Pure binary contrast eliminates gray-bleed entirely, delivering 100% OCR fidelity down to 7px/8px cells.
- **Canvas Boundaries**:
  - $256 \times 256\text{px}$: 87 tokens on Claude, 255 on OpenAI $\to$ **$7.6\times$ compression** in $3\times5$ font.
  - $512 \times 512\text{px}$: 255 tokens on OpenAI, 258 on Gemini $\to$ **$10.5\times$ compression** in $3\times5$ font.
  - $1564 \times 1568\text{px}$: Multi-column card fitting full language or practice documentation bundles.

---

## 4. Visual Layout Engine & Geometry (`internal/readcard`)

### 4.1 Multi-Column Line Balancing & Height Optimization
To minimize vertical tile count, lines are distributed across $N$ columns using ceiling division:
$$\text{linesPerCol} = \left\lceil \frac{\text{totalLines}}{N} \right\rceil$$
Distributing lines evenly ensures the canvas height stays under the critical $512\text{px}$ ViT threshold for typical file lengths (70–150 lines), preventing single-column vertical ballooning.

### 4.2 Horizontal Boundary Safety (`DrawStringBounded`)
When rendering code lines into column slots, text must never spill across column gutters. The `DrawStringBounded(img, font, x, y, col, str, minX, maxX)` primitive strictly clips pixel drawing to the column's assigned bounding box:
- Characters exceeding `maxX` are safely suppressed.
- Gutters and adjacent column text remain 100% clean and uncorrupted.

### 4.3 Dynamic Auto-Width Scaling & Continuation Soft-Wrapping
Code lines frequently exceed standard column widths (e.g. 80 chars):
1. **Dynamic Single-Column Expansion**: In single-column mode (`--columns=1`), canvas width dynamically scales with the maximum line length (up to ~240 characters) before wrapping.
2. **Continuation Soft-Wrapping**: Lines exceeding column capacity are cleanly soft-wrapped onto indented continuation rows prefixed with `↳ ` in muted syntax styling.
3. **Ellipsis Truncation Marker**: If a deeply nested line exceeds maximum continuation depth, an ellipsis (`…`) indicates bounded truncation without silent data dropping.

### 4.4 Content-First Dynamic Bounding Box Cropping
Canvases are sized content-first:
- Trailing empty columns (when total lines are fewer than the column count) are actively pruned.
- The bounding box wraps strictly around the populated text columns, gutter, and headers, eliminating empty margin waste.

---

## 5. CLI Implementation & Operational Commands

### 5.1 Visual File Inspection (`harnez read -I`)
Render source files directly into bounded, syntax-highlighted visual cards:
```bash
# Render file with default 5x8 retro pixel font and optimal 3-column packing:
harnez read -I internal/lint/lint.go

# Read specific line ranges with custom column wrapping:
harnez read -I --lines 1:120 --columns 3 cmd/harnez/main.go
```

### 5.2 Automated Cheatsheet Builder (`harnez docs cards`)
Compile repository markdown documentation into bounded visual PNG cards:
```bash
# Compile standard 3-in-1 bundle (Bash + Make + Git) into docs/vision/:
harnez docs cards build --bundle=dev-3in1 --out=docs/vision/

# Validate card dimensions and font resolution against ViT bounds:
harnez docs cards check docs/vision/*.png
```

### 5.3 Subagent Dispatch & Doc Modes
Coordinate context delivery across agent harnesses:
- `harnez mode vision` (or `harnez mode 4`): Activates visual documentation tier.
- Subagent launcher stages `STYLE_GUIDE.png` and provides multimodal context headers with zero text injection.

---

## 6. Pre-Tool Enforcement & Reading Discipline

To prevent token fatigue and preserve rate quotas, harnez provides automated PreToolUse hooks (`harnez hook pre-tool`):
1. **Unconstrained Read Guard**: Native IDE file-view tool calls (`view_file`, `View`, `ReadMultipleFiles`) on files $>100$ lines are blocked by the pre-tool hook with an actionable directive to run `harnez read -I` (visual card) or `harnez read -L <range> -n` (bounded line range).
2. **Binary Media Exemption**: Rendered image artifacts (`.png`, `.jpg`, `.svg`) are exempted, allowing native image viewers to inspect visual cards without restriction.
3. **Session Rate & Activity Feedback**: The hook provides real-time tool rate feedback (`Summary: N tool calls in Xm`), encouraging efficient tool usage.

---

## 7. Mechanical Verification Invariants

All multimodal instruction delivery adheres to three verification principles:
1. **Isolated Scratch Execution**: Canaries and subagent tasks execute in clean scratch workspaces without ambient text rules.
2. **Mechanical Linting**: Compliance is judged objectively by `harnez lint` / `internal/lint` AST and regex checks (`if test`, `set -euo pipefail`, `git -C`, `mktemp`+`trap`, `local` declarations) rather than subjective LLM self-assessment.
3. **Traceable Token Accounting**: All visual dispatches log exact pixel dimensions, ViT tile counts, and baseline text token comparisons.
