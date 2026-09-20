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

	// Dot8 uses 3x4 cells for Braille and 3x5 for text
	brailleCellWidth := 3
	brailleCellHeight := 4
	textFont := Font3x5
	textCellWidth := textFont.CharWidth

	// Line height must accommodate both (max height + spacing)
	lineHeight := brailleCellHeight + 2

	totalLines := len(lines)
	if totalLines == 0 {
		lines = []string{"(empty file)"}
		totalLines = 1
	}

	// Calculate gutter width
	maxLineNum := opts.StartLine + totalLines - 1
	if len(opts.SourceLines) > 0 {
		maxLineNum = opts.SourceLines[len(opts.SourceLines)-1]
	}
	digits := len(fmt.Sprintf("%d", maxLineNum))
	if digits < 3 {
		digits = 3
	}
	gutterWidth := digits*textCellWidth + 8

	// Layout constants
	headerHeight := 36
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

	availableHeight := opts.MaxDimension - headerHeight - (paddingY * 2) - 4
	linesPerCol := availableHeight / lineHeight
	if linesPerCol < 1 {
		linesPerCol = 1
	}

	// Calculate max line width (in Braille cells)
	// Each Braille cell is 3px wide, non-Braille chars are 3px (Font3x5)
	maxLineLen := 0
	for _, l := range lines {
		rCount := len([]rune(strings.ReplaceAll(l, "\t", "    ")))
		if rCount > maxLineLen {
			maxLineLen = rCount
		}
	}
	if maxLineLen < 35 {
		maxLineLen = 35
	}

	// Reduce columns if needed
	for cols > 1 && (opts.MaxDimension-2*paddingX-(cols-1)*colGap)/cols < gutterWidth+16+3*brailleCellWidth {
		cols--
	}

	// Column width in pixels
	colWidth := gutterWidth + (maxLineLen * brailleCellWidth) + 16
	totalContentWidth := (cols * colWidth) + ((cols - 1) * colGap)
	cardWidth := totalContentWidth + (paddingX * 2)
	if cardWidth > opts.MaxDimension {
		cardWidth = opts.MaxDimension
		colWidth = (cardWidth - (paddingX * 2) - ((cols - 1) * colGap)) / cols
	}

	// Create header
	headerImg := image.NewRGBA(image.Rect(0, 0, cardWidth, headerHeight))
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

	// Create main content image
	contentHeight := paddingY + linesPerCol*lineHeight + paddingY
	contentImg := image.NewRGBA(image.Rect(0, 0, cardWidth, contentHeight))
	draw.Draw(contentImg, contentImg.Bounds(), image.NewUniform(theme.Bg), image.Point{}, draw.Src)

	// Render lines
	y := paddingY
	for i := 0; i < linesPerCol && i < len(lines); i++ {
		line := lines[i]
		lineNum := opts.StartLine + i
		if len(opts.SourceLines) > i {
			lineNum = opts.SourceLines[i]
		}

		// Draw line number
		numStr := fmt.Sprintf("%d", lineNum)
		if opts.ShowLineNumbers {
			x := paddingX
			for _, r := range numStr {
				textFont.DrawRune(contentImg, r, x, y, theme.GutterFg)
				x += textFont.CharWidth
			}
		}

		// Draw content
		x := paddingX + gutterWidth
		for _, r := range line {
			if r >= 0x2800 && r <= 0x2800+0xFF {
				// Braille cell: draw as 3x4 dots with special colors for dots 7 and 8
				drawDot8BraillCell(contentImg, r, x, y, theme.Text, theme.Keyword, theme.Type)
			} else {
				// Regular character
				textFont.DrawRune(contentImg, r, x, y, theme.Text)
			}
			x += brailleCellWidth
		}

		y += lineHeight
	}

	// Combine header and content
	fullImg := image.NewRGBA(image.Rect(0, 0, cardWidth, headerHeight+contentHeight))
	draw.Draw(fullImg, image.Rect(0, 0, cardWidth, headerHeight), headerImg, image.Point{}, draw.Src)
	draw.Draw(fullImg, image.Rect(0, headerHeight, cardWidth, headerHeight+contentHeight), contentImg, image.Point{}, draw.Src)

	// Save image
	filepath.Base(opts.OutputPath)
	outPath := opts.OutputPath
	if outPath == "" {
		outPath = fmt.Sprintf("read_%s_%dx%d.png", strings.TrimSuffix(filepath.Base(opts.Title), filepath.Ext(opts.Title)), cardWidth, headerHeight+contentHeight)
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
		Height:     headerHeight + contentHeight,
		Columns:    cols,
		TotalLines: totalLines,
		TotalPages: 1,
		TokenStats: stats,
		PrimaryPath: outPath,
		Pages: []PageGeometry{
			{
				Width:   cardWidth,
				Height:  headerHeight + contentHeight,
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
