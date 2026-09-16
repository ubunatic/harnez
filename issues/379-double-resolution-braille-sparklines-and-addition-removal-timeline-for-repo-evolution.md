# 379 — Double-resolution braille sparklines and addition/removal timeline for repo evolution

**Status**: In Progress
**Priority**: P2 (Medium)  
**Severity**: Minor  
**Category**: Feature  

**Related**: [201](201-double-timeseries-resolution-and-add-btop-style-braille-sparklines.md), [376](376-multi-track-git-history-evolution-sparks-across-code-tests-docs-skills-and-issues.md), [377](377-project-level-token-attribution-and-generative-cost-of-change-metrics.md), `../loom/graph`  

---

## 1. Summary & Motivation

Currently, `harnez dochistory --tracks` and `harnez repo-history` use single-column 8-step block runes (` ▂▃▄▅▆▇█`) for sparkline rendering. In a 10-character terminal box, this fits 10 time sample points.

By adopting **Braille Unicode (`0x2800` – `0x28FF`)** (referencing `../loom/graph` and `internal/rograph`):
1. **Double Horizontal Resolution (2x Points in Same Width)**:
   - Each Braille cell encodes **two independent vertical columns** (left column: dots 1, 2, 3, 7; right column: dots 4, 5, 6, 8).
   - In a 10-character box, we can render **20 discrete time points** with 4 vertical levels per column.
   - Baseline zero is `⣀` (dots 7 & 8 at the bottom), and maximum height is `⣿` (full 8 dots).
2. **Additions / Removals Velocity Timeline with ANSI Color Coding**:
   - For commit-by-commit diffs and churn:
     - **Green (`\x1b[32m`)**: Net positive growth / additions dominant.
     - **Red (`\x1b[31m`)**: Net reduction / pruning dominant.
     - **Yellow (`\x1b[33m`)**: Mixed / refactor churn (both additions and removals).

---

## 2. Visual Specification

### Braille Sparkline Format

```text
Baseline (0, 0):         [⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀]  (dots 7 & 8 only)
Rising Trend:            [⣀⣠⣤⣦⣶⣷⣿⣿⣿⣿]
Churn / Fluctuation:     [⣀⣤⣠⣠⣀⣤⣤⣄⣄⣀]
```

### Repo Evolution with Braille & Color Coding

```text
Repo Evolution (harnez · 1147 commits · 120d)
Code:    [⣀⣀⣀⣠⣤⣤⣶⣶⣿] 43.5k LOC (400k tokens)
Tests:   [⣀⣀⣀⣀⣤⣤⣶⣶⣿] 30.5k LOC (278k tokens) · 0.70 test/code ratio
Docs:    [⣀⣠⣠⣤⣦⣶⣶⣿⣿] 278k tokens (146 files)
Skills:  [⣀⣀⣀⣤⣶⣶⣶⣿⣿] 19 skills (17.1k tokens)
Issues:  [⣀⣠⣤⣦⣶⣿⣿⣿⣿] 379 tickets (149 open · 230 closed)
```

---

## 3. Technical Architecture

1. **Braille Mapping Function**:
   - `brailleGlyph(leftLevel, rightLevel int) rune`
   - Maps values $0 \dots 4$ to Braille dots:
     - Left column dots: `[0, 0x40, 0x44, 0x46, 0x47]` (dots: none, 7, 7+3, 7+3+2, 7+3+2+1)
     - Right column dots: `[0, 0x80, 0xA0, 0xB0, 0xB8]` (dots: none, 8, 8+6, 8+6+5, 8+6+5+4)
     - Default baseline at zero: `0x2800 | 0x40 | 0x80` = `\u28C0` (`⣀`).
2. **Diff / Color Classifier**:
   - For each 2-sample cell $(v_i, v_{i+1})$:
     - $\Delta = v_{i+1} - v_i$
     - If $\Delta > 0 \rightarrow$ Green
     - If $\Delta < 0 \rightarrow$ Red
     - If $\Delta == 0 \rightarrow$ Muted / Default
3. **Integration**:
   - Update `internal/assess/tracks.go` and `internal/assess/dochistory.go` to use the Braille renderer.
   - Support toggle via `--braille` or make Braille the standard high-resolution default.

---

## 4. Acceptance Criteria

- [ ] `internal/assess/` implements 2-samples-per-cell Braille sparkline generation (`0x2800` mapping).
- [ ] Baseline zero renders as `⣀` (`\u28C0`), not empty whitespace or mid-blocks.
- [ ] Multi-track evolution charts (`harnez dochistory --tracks`) render 20-sample Braille sparklines across 10-cell width.
- [ ] Addition/removal color tinting (green for additions, red for removals, yellow for churn) supported when ANSI color is enabled.
- [ ] Unit tests verify Braille dot math, baseline rendering, and 20-sample resampling.
