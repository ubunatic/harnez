package readcard

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// dot8RenderFileToCards renders Dot8-encoded text as native 3x4 Braille dot cards.
// Braille cells are drawn as 1px dots in columns 0 and 2 (with gap in column 1).
// Non-Braille characters use the 3x5 Tom Thumb font.
// Dots 7 and 8 use accent colors for better readability.
func dot8RenderFileToCards(lines []string, filename string, opts RenderOptions) (*RenderResult, error) {
	if err := validateStyle(opts); err != nil {
		return nil, err
	}

	// Validate and set defaults
	if opts.MaxDimension <= 0 {
		opts.MaxDimension = 1568
	}
	if opts.MaxDimension < 160 {
		return nil, fmt.Errorf("max dimension must be at least 160")
	}
	if opts.StartLine <= 0 {
		opts.StartLine = 1
	}
	if opts.Title == "" {
		opts.Title = filename
	}

	cadence, _ := ParseLineNumbers(opts.LineNumbers)
	if opts.LineNumbers != "" {
		opts.ShowLineNumbers = cadence != 0
	}

	// Select theme
	theme := DarkTheme
	if strings.ToLower(opts.Theme) == "light" {
		theme = LightTheme
	}

	// Dot8 uses 3px cells for both Braille and text (3x5 font character width is 3px)
	cellWidth := 3
	brailleCellHeight := 4
	textFont := Font3x5

	// Line height to fit content
	lineHeight := brailleCellHeight + 2

	totalLines := len(lines)
	if totalLines == 0 {
		lines = []string{"(empty file)"}
		totalLines = 1
	}

	// Calculate gutter width for line numbers
	maxLineNum := opts.StartLine + totalLines - 1
	if len(opts.SourceLines) > 0 {
		maxLineNum = opts.SourceLines[len(opts.SourceLines)-1]
	}
	digits := len(fmt.Sprintf("%d", maxLineNum))
	if digits < 3 {
		digits = 3
	}
	gutterWidth := 0
	if opts.ShowLineNumbers {
		gutterWidth = digits*textFont.CharWidth + 8
	}

	// Layout constants (match default card)
	headerHeight := 36
	legendHeight := 8 // height of legend strip
	paddingX := 16
	paddingY := 12
	colGap := 16
	if opts.Chrome == ChromeSlim || opts.Chrome == ChromeNone {
		headerHeight, paddingX, paddingY, colGap = 14, 8, 4, 10
	}

	// Column layout
	cols := opts.Columns
	if cols <= 0 {
		cols = 3
	}
	if cols > 4 {
		cols = 4
	}

	availableHeight := opts.MaxDimension - headerHeight - legendHeight - (paddingY * 2) - 4
	linesPerCol := availableHeight / lineHeight
	if linesPerCol < 1 {
		linesPerCol = 1
	}

	// Calculate max line width (in characters, all 3px wide)
	maxLineLen := 0
	for _, l := range lines {
		l = strings.ReplaceAll(l, "\t", "    ")
		rCount := len([]rune(l))
		if rCount > maxLineLen {
			maxLineLen = rCount
		}
	}
	if maxLineLen < 35 {
		maxLineLen = 35
	}

	// Reduce columns if needed
	for cols > 1 && (opts.MaxDimension-2*paddingX-(cols-1)*colGap)/cols < gutterWidth+16+3*cellWidth {
		cols--
	}

	// Column width in pixels
	colWidth := gutterWidth + (maxLineLen * cellWidth) + 16
	totalContentWidth := (cols * colWidth) + ((cols - 1) * colGap)
	cardWidth := totalContentWidth + (paddingX * 2)
	if cardWidth > opts.MaxDimension {
		cardWidth = opts.MaxDimension
		colWidth = (cardWidth - (paddingX * 2) - ((cols - 1) * colGap)) / cols
	}

	// Calculate actual content height (don't over-allocate)
	// With cols columns, we need ceil(totalLines / cols) rows per column
	actualLinesPerCol := (totalLines + cols - 1) / cols
	contentHeight := actualLinesPerCol * lineHeight

	cardHeight := headerHeight + legendHeight + paddingY*2 + contentHeight

	// Create header with legend
	headerImg := image.NewRGBA(image.Rect(0, 0, cardWidth, headerHeight+legendHeight))
	draw.Draw(headerImg, headerImg.Bounds(), image.NewUniform(theme.HeaderBg), image.Point{}, draw.Src)

	// Draw title in header
	if opts.Chrome != ChromeNone {
		titleFont := ResolveFont("pixel", 8)
		x := paddingX
		for _, r := range opts.Title {
			titleFont.DrawRune(headerImg, r, x, 8, theme.HeaderFg)
			x += titleFont.CharWidth
		}
	}

	// Draw legend strip in header: "a=⠁ b=⠃ ... z=⠵ A=⡁ 1=⢀⠁ docs/BrailleDot8.md"
	legendX := paddingX
	legendY := headerHeight + 1

	// Draw sample Braille legend
	legendFont := ResolveFont("pixel", 6)
	legendSamples := []struct {
		label string
		cell  rune
	}{
		{"a", rune(0x2800 + 0b000001)},
		{"z", rune(0x2800 + 0b110101)},
		{"A", rune(0x2800 + 0b000001 + (1 << 6))},
		{"1", rune(0x2800 + (1 << 7))}, // dot 8 prefix
	}

	for _, sample := range legendSamples {
		// Draw label
		for _, r := range sample.label {
			legendFont.DrawRune(headerImg, r, legendX, legendY, theme.Punctuation)
			legendX += legendFont.CharWidth
		}
		// Draw "="
		legendFont.DrawRune(headerImg, '=', legendX, legendY, theme.Punctuation)
		legendX += legendFont.CharWidth
		// Draw cell (as 3x4 dots)
		drawDot8BraillCell(headerImg, sample.cell, legendX, legendY, theme.Text, theme.Keyword, theme.Type)
		legendX += 3 + 2 // cell width + gap
	}

	// Draw docs pointer
	docsText := "docs/BrailleDot8.md"
	for _, r := range docsText {
		legendFont.DrawRune(headerImg, r, legendX, legendY, theme.Comment)
		legendX += legendFont.CharWidth
	}

	// Create main content image
	contentImg := image.NewRGBA(image.Rect(0, 0, cardWidth, paddingY+contentHeight+paddingY))
	draw.Draw(contentImg, contentImg.Bounds(), image.NewUniform(theme.Bg), image.Point{}, draw.Src)

	// Render lines in columns
	y := paddingY
	lineIdx := 0

	for col := 0; col < cols && lineIdx < len(lines); col++ {
		x := paddingX + col*(colWidth+colGap)
		colY := y

		for row := 0; row < linesPerCol && lineIdx < len(lines); row++ {
			line := lines[lineIdx]
			lineNum := opts.StartLine + lineIdx
			if len(opts.SourceLines) > lineIdx {
				lineNum = opts.SourceLines[lineIdx]
			}
			lineIdx++

			// Draw line number
			if opts.ShowLineNumbers {
				numStr := fmt.Sprintf("%d", lineNum)
				numX := x
				for _, r := range numStr {
					textFont.DrawRune(contentImg, r, numX, colY, theme.GutterFg)
					numX += textFont.CharWidth
				}
			}

			// Draw content
			contentX := x + gutterWidth
			for _, r := range line {
				if r >= 0x2800 && r <= 0x2800+0xFF {
					// Braille cell: draw as 3x4 dots with special colors for dots 7 and 8
					drawDot8BraillCell(contentImg, r, contentX, colY, theme.Text, theme.Keyword, theme.Type)
				} else {
					// Regular character
					textFont.DrawRune(contentImg, r, contentX, colY, theme.Text)
				}
				contentX += cellWidth
			}

			colY += lineHeight
		}
	}

	// Combine header and content
	fullImg := image.NewRGBA(image.Rect(0, 0, cardWidth, cardHeight))
	draw.Draw(fullImg, image.Rect(0, 0, cardWidth, headerHeight+legendHeight), headerImg, image.Point{}, draw.Src)
	draw.Draw(fullImg, image.Rect(0, headerHeight+legendHeight, cardWidth, cardHeight), contentImg, image.Point{}, draw.Src)

	// Save image
	outPath := opts.OutputPath
	if outPath == "" {
		outPath = fmt.Sprintf("harnez_read_%s_%d-tokens_%s.png",
			strings.TrimSuffix(filepath.Base(opts.Title), filepath.Ext(opts.Title)),
			opts.SourceTokens,
			"dot8card")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, fullImg); err != nil {
		return nil, fmt.Errorf("encode PNG: %w", err)
	}

	// Return result
	fullText := strings.Join(lines, "\n")
	stats := ComputeTextTokens(fullText)

	return &RenderResult{
		Files:      []string{outPath},
		Width:      cardWidth,
		Height:     cardHeight,
		Columns:    cols,
		TotalLines: totalLines,
		TotalPages: 1,
		TokenStats: stats,
		PrimaryPath: outPath,
		Pages: []PageGeometry{
			{
				Width:   cardWidth,
				Height:  cardHeight,
				Columns: cols,
			},
		},
	}, nil
}

// drawDot8BraillCell draws a Braille cell as 3x4 dots with special colors for dots 7 and 8.
// Dots 1-6 are drawn in the base color, dot 7 (uppercase) in dot7Color, dot 8 (digit prefix) in dot8Color.
// Layout: column 0 (dots 1,2,3), gap, column 2 (dots 4,5,6), row 3 (dots 7,8 combined).
// Each dot is 1 pixel.
func drawDot8BraillCell(img *image.RGBA, r rune, x, y int, baseColor, dot7Color, dot8Color color.RGBA) {
	if r == 0x2800 {
		return // Empty cell
	}

	bits := byte(r - 0x2800)

	// Dot positions: col 0 for dots 1,2,3; col 2 for dots 4,5,6; row 3 for dots 7,8
	dotPositions := []struct {
		dot int
		col int
		row int
	}{
		{1, 0, 0}, {2, 0, 1}, {3, 0, 2},
		{4, 2, 0}, {5, 2, 1}, {6, 2, 2},
		{7, 0, 3}, {8, 2, 3},
	}

	for _, pos := range dotPositions {
		if (bits & (1 << (pos.dot - 1))) != 0 {
			px := x + pos.col
			py := y + pos.row
			col := baseColor
			if pos.dot == 7 {
				col = dot7Color
			} else if pos.dot == 8 {
				col = dot8Color
			}
			img.SetRGBA(px, py, col)
		}
	}
}
