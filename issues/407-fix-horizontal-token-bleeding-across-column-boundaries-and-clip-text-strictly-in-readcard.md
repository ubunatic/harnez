# 407 — Fix horizontal token bleeding across column boundaries and clip text strictly in readcard

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug / Multimodal Context & Readcard
**Related**: #406, #395, #398, #400

---

## 1. Problem Statement & Motivation

Following the multi-column line distribution fix in Issue 406, visual inspection of rendered multi-column and single-column diff cards revealed two horizontal rendering defects:

1. **Horizontal Token Bleeding Across Column Boundaries**:
   In `internal/readcard/font.go`, `DrawString` rasterizes runes sequentially without enforcing a maximum horizontal boundary (`maxX`). In `internal/readcard/render.go`, the line rendering loop only checked `if tokenX >= colX+colWidth-cw` *after* drawing the entire token. If a token crossed the column boundary, its trailing characters physically spilled over the column separator and collided directly into the line numbers of the next column (or over the right canvas border).
2. **Auto Column Selection on Wide Lines**:
   The auto column selector in `RenderFileToCards` chose `cols = 2` solely based on line count without evaluating line width. When lines are wide (>85 chars), 2 columns on a 1568px canvas force columns to be narrower than the content, causing avoidable truncation.

---

## 2. Technical Scope & Implementation Plan

### 2.1 Add Bounded String Drawing to `MonospaceFont`
- In `internal/readcard/font.go`, add:
  ```go
  func (f *MonospaceFont) DrawStringBounded(img *image.RGBA, s string, x, y, maxX int, col color.RGBA) int
  ```
  - Before rasterizing each rune, check if `curX + f.CharWidth > maxX`. If so, stop rendering immediately.
  - Return the rendered pixel advance.

### 2.2 Strict Column Clipping in `internal/readcard/render.go`
- Compute `maxColX := colX + colWidth - 4` (or `cardWidth - paddingX` for the last column).
- Use `font.DrawStringBounded(img, tok.Text, tokenX, curY, maxColX, tokCol)` when rendering code tokens.

### 2.3 Intelligent Auto Column Selection
- In auto mode (`opts.Columns <= 0`):
  - If `maxLineLen > 85`, keep `cols = 1` unless the user explicitly requested `--columns=2` or higher.
  - For shorter lines ($\le 85$ chars), allow multi-column layout for $\ge 65$ lines.

### 2.4 Verification
- Add unit tests in `internal/readcard/read_test.go` asserting that no pixels are drawn past `maxX` or into adjacent column gutters.
- Verify `make test-q1`.

---

## 3. Acceptance Criteria

- [ ] `DrawStringBounded` stops drawing runes strictly before `maxX`.
- [ ] Multi-column rendering never spills tokens across the column separator or over the right border.
- [ ] Auto column selection retains `cols = 1` for wide lines (>85 chars) to prevent unnecessary truncation.
- [ ] Unit tests pass in `internal/readcard/` verifying bounded rendering.
