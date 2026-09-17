# 406 — Fix multi-column line distribution and crop whitespace in harnez read image rendering

**Status**: Closed
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug / Multimodal Context & Readcard
**Related**: #395, #396, #398, #400, #401

---

## 1. Problem Statement & Motivation

A visual inspection of multiple generated visual PNG context cards in `/tmp/` (`/tmp/harnez_read_stdin_efbb8f27.png`, `/tmp/harnez_read_343_*.png`, `/tmp/harnez_read_tokens_5ff2f93f.png`) revealed a severe layout defect and data truncation bug in `internal/readcard/render.go`:

### 1.1 Column Line Distribution & Data Clipping Bug
In `RenderFileToCards`:
- The card height for a page is dynamically calculated from `pageColLines = (pageLinesCount + cols - 1) / cols` (e.g., 43 lines for an 85-line diff in 2 columns $\to$ canvas height 498px).
- However, the column rendering loop slices lines using the **theoretical maximum capacity** `linesPerCol` (150 lines at 1568px max dimension) rather than the actual `pageColLines`:
  ```go
  // Bug in internal/readcard/render.go:
  for c := 0; c < cols; c++ {
      cStartIdx := c * linesPerCol // linesPerCol = 150!
      ...
  }
  ```
- **Catastrophic Impact**:
  - **Column 0**: Begins at line 0 and attempts to render all 85 lines, but the canvas is only 43 lines high. Lines 44–85 are rendered below the bottom of the canvas and completely lost / clipped!
  - **Column 1**: Begins at index 150, which is $> 85$, breaks immediately, and renders as a **100% empty black void** covering 50% of the image.

### 1.2 Uncropped Canvas Width
When files with shorter line lengths or fewer columns are rendered, `colWidth` is padded assuming 120 characters and `cardWidth` expands to `1568px`, leaving large expanses of unused black margin on the right.

---

## 2. Technical Scope & Implementation Plan

### 2.1 Fix Column Slicing in `internal/readcard/render.go`
- In `RenderFileToCards`, calculate the line stride per column for the current page as:
  ```go
  stride := pageColLines
  if stride < 1 {
      stride = 1
  }
  for c := 0; c < cols; c++ {
      cStartIdx := c * stride
      if cStartIdx >= len(pageLines) {
          break
      }
      cEndIdx := cStartIdx + stride
      if cEndIdx > len(pageLines) {
          cEndIdx = len(pageLines)
      }
      colLines := pageLines[cStartIdx:cEndIdx]
      ...
  }
  ```

### 2.2 Dynamic Content Width & Margin Cropping
- Calculate `maxLineLen` from the actual lines present on the page (or file), with reasonable min bounds (e.g. 35 chars).
- Compute `cardWidth` strictly as `(cols * colWidth) + ((cols - 1) * colGap) + (paddingX * 2)` without clamping to `MaxDimension` unless it exceeds it.

### 2.3 Unit Testing & Visual Verification
- Add unit tests in `internal/readcard/read_test.go`:
  - Test multi-column distribution for 85-line, 133-line, and 250-line inputs.
  - Verify that lines in column 1 (e.g. lines 44–85) contain non-background pixels and that text is rendered in both columns.
  - Verify that single-column and narrow diffs produce tightly cropped widths.

---

## 3. Acceptance Criteria

- [ ] Multi-column rendering evenly balances lines across all active columns on each page.
- [ ] No lines are clipped or drawn off the bottom of the canvas.
- [ ] Width is tightly cropped to actual line length and active column count.
- [ ] Unit tests pass in `internal/readcard/` confirming non-empty second column rendering and pixel assertions.
- [ ] Visual verification of `/tmp/harnez_read_stdin_*.png` and `/tmp/harnez_read_tokens_*.png` confirms 2-column balanced layout.
