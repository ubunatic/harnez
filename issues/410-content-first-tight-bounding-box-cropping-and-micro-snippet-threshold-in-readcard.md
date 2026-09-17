# 410 — Content-first tight bounding box cropping and micro-snippet threshold in readcard

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Multimodal Context & Readcard
**Related**: #406, #407, #408, #409

---

## 1. Problem Statement & Motivation

Two related inefficiencies currently impact visual card generation in `internal/readcard/`:

### 1.1 Premature Geometry Allocation & Empty Column Voids
In `internal/readcard/render.go`, canvas geometry is calculated before the content is tokenized and wrapped into actual rows. If a user or script invokes multi-column rendering on short inputs (e.g. `echo "123" | harnez read -I --columns=3` or a 15-line snippet with `--columns=3`), the engine allocates all 3 columns and sets `cardWidth = 850px`, leaving columns 2 and 3 as completely empty black space.

Canvas dimensions should be computed **content-first**:
1. Wrap and slice content into actual rendered rows.
2. Determine `usedCols` for the page — if all rows fit in 1 column, `usedCols = 1`, and columns 2/3 are never allocated or drawn.
3. Compute the exact bounding box from `actualMaxLineLen` of the rendered rows.
4. Only expand `cardWidth` to the minimum width required by the header title and badges.

### 1.2 Micro-Snippet Threshold Policy (Images vs. Text)
For micro-snippets (1–5 lines, e.g. `echo "123"`, single error traces, 3-line configs), transmitting text as an image card incurs an irreducible floor:
- Base header + border + 1 ViT tile = **~136 vision tokens on Claude, ~774 on Gemini**.
- Transmitting a 1-token or 10-token snippet as 774 vision tokens represents a 77x token inflation penalty.
- Visual cards excel when packing **50 to 500 lines** into 1–2 ViT tiles. For micro-snippets, text output (`harnez read -n`) is orders of magnitude more efficient.

---

## 2. Technical Scope & Architecture

### 2.1 Content-First Bounding Box Engine (`internal/readcard/render.go`)
- **Row-First Pipeline**:
  - Build `allRows []renderRow` (including soft-wrapped lines) before determining canvas geometry.
- **Dynamic Active Column Pruning**:
  ```go
  usedCols := (len(pageRows) + linesPerCol - 1) / linesPerCol
  if usedCols < 1 {
      usedCols = 1
  }
  if usedCols > cols {
      usedCols = cols
  }
  ```
- **Content Width Calculation**:
  - Calculate `actualMaxLineLen` strictly from the rows appearing on that page.
  - `colWidth := gutterWidth + (actualMaxLineLen * cw) + 16`
  - `contentWidth := (usedCols * colWidth) + ((usedCols - 1) * colGap) + (paddingX * 2)`
- **Header Fit Constraint**:
  - `headerMinWidth := (len(titleText) + len(badgeText)) * cw + (paddingX * 2) + 32`
  - `cardWidth := max(contentWidth, headerMinWidth)` (clamped to `MaxDimension`).

### 2.2 Micro-Snippet Threshold in `--auto` Routing (Issue 409 Integration)
- Define thresholds:
  - `MicroSnippetLineThreshold = 5`
  - `MicroSnippetTokenThreshold = 100`
- When `--auto` (or `--doc-mode=auto`) is active:
  - If `totalLines <= 5` and `rawTokens < 100`: route directly to text stream (`harnez read -n`) rather than rendering an image.
  - If `-I` is explicitly passed by the user/agent, continue rendering the tightly-cropped image.

---

## 3. Acceptance Criteria

- [ ] Unused columns are automatically pruned (`usedCols = 1` for 1 row on a 3-column request).
- [ ] Card width is tightly cropped to the actual content bounding box + header minimum.
- [ ] Running `echo "123" | harnez read -I --columns=3` generates a compact ~300–350px card with zero empty column gaps.
- [ ] Micro-snippet threshold policy defined for integration with `--auto` provider routing.
- [ ] Unit tests in `internal/readcard/read_test.go` verifying active column pruning and tight width cropping.
