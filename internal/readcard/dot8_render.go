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
// Dots 7 and 8 use accent colors for better readability. Dot8Colors can opt
// into red/white checkerboard coloring for the 2x4 dot matrix.
func dot8RenderFileToCards(lines []string, filename string, opts RenderOptions) (*RenderResult, error) {
	if err := validateStyle(opts); err != nil {
		return nil, err
	}
	if opts.Dot8Colors != "" && opts.Dot8Colors != "red-white" {
		return nil, fmt.Errorf("invalid dot8 colors %q (use red-white)", opts.Dot8Colors)
	}
	if opts.Dot8Pitch != 0 && opts.Dot8Pitch != 3 && opts.Dot8Pitch != 4 {
		return nil, fmt.Errorf("invalid dot8 pitch %d (use 3 or 4)", opts.Dot8Pitch)
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

	// Dot8 uses 3px cells for both Braille and text (dedicated 3x6 Dot8 font)
	cellWidth := opts.Dot8Pitch
	if cellWidth == 0 {
		cellWidth = 3
	}
	brailleCellHeight := 4
	textFont := FontDot8
	if cellWidth == 4 {
		textFont = Font3x5
	}

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
	if opts.Chrome == ChromeNone {
		headerHeight, legendHeight, paddingX, paddingY, colGap = 0, 0, 8, 4, 10
	} else if opts.Chrome == ChromeSlim {
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

	linesPerPage := linesPerCol * cols
	totalPages := (totalLines + linesPerPage - 1) / linesPerPage
	if totalPages == 0 {
		totalPages = 1
	}

	var outputPaths []string
	var pages []PageGeometry
	firstCardHeight := 0
	firstCardWidth := 0

	for page := 0; page < totalPages; page++ {
		pageStartIdx := page * linesPerPage
		pageEndIdx := pageStartIdx + linesPerPage
		if pageEndIdx > totalLines {
			pageEndIdx = totalLines
		}
		pageLines := lines[pageStartIdx:pageEndIdx]

		actualLinesPerCol := (len(pageLines) + cols - 1) / cols
		contentHeight := actualLinesPerCol * lineHeight

		cardHeight := headerHeight + legendHeight + paddingY*2 + contentHeight
		if cardHeight > opts.MaxDimension {
			cardHeight = opts.MaxDimension
		}

		if page == 0 {
			firstCardHeight = cardHeight
			firstCardWidth = cardWidth
		}
		pages = append(pages, PageGeometry{Width: cardWidth, Height: cardHeight, Columns: cols})

		if opts.MeasureOnly {
			continue
		}

		// Create header with legend
		headerImg := image.NewRGBA(image.Rect(0, 0, cardWidth, headerHeight+legendHeight))
		draw.Draw(headerImg, headerImg.Bounds(), image.NewUniform(theme.HeaderBg), image.Point{}, draw.Src)

		// Draw title in header
		if opts.Chrome != ChromeNone {
			titleFont := ResolveFont("pixel", 8)
			x := paddingX
			titleText := opts.Title
			if totalPages > 1 {
				titleText = fmt.Sprintf("%s (Page %d/%d)", opts.Title, page+1, totalPages)
			}
			for _, r := range titleText {
				titleFont.DrawRune(headerImg, r, x, 8, theme.HeaderFg)
				x += titleFont.CharWidth
			}
		}

		if legendHeight > 0 {
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
				for _, r := range sample.label {
					legendFont.DrawRune(headerImg, r, legendX, legendY, theme.Punctuation)
					legendX += legendFont.CharWidth
				}
				legendFont.DrawRune(headerImg, '=', legendX, legendY, theme.Punctuation)
				legendX += legendFont.CharWidth
				drawDot8BraillCell(headerImg, sample.cell, legendX, legendY, theme.Text, theme.Keyword, theme.Type, opts.Dot8Colors, cellWidth)
				legendX += 3 + 2 // cell width + gap
			}

			docsText := "docs/BrailleDot8.md"
			for _, r := range docsText {
				legendFont.DrawRune(headerImg, r, legendX, legendY, theme.Comment)
				legendX += legendFont.CharWidth
			}
		}

		// Create main content image
		contentImg := image.NewRGBA(image.Rect(0, 0, cardWidth, paddingY+contentHeight+paddingY))
		draw.Draw(contentImg, contentImg.Bounds(), image.NewUniform(theme.Bg), image.Point{}, draw.Src)

		// Render lines in columns
		y := paddingY
		lineIdx := 0

		for col := 0; col < cols && lineIdx < len(pageLines); col++ {
			x := paddingX + col*(colWidth+colGap)
			colY := y

			for row := 0; row < actualLinesPerCol && lineIdx < len(pageLines); row++ {
				line := pageLines[lineIdx]
				lineNum := opts.StartLine + pageStartIdx + lineIdx
				if len(opts.SourceLines) > pageStartIdx+lineIdx {
					lineNum = opts.SourceLines[pageStartIdx+lineIdx]
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
				for cellIndex, r := range line {
					if r >= 0x2800 && r <= 0x2800+0xFF {
						drawDot8BraillCell(contentImg, r, contentX, colY, theme.Text, theme.Keyword, theme.Type, opts.Dot8Colors, cellWidth)
					} else {
						textColor := theme.Keyword
						if cellIndex%2 == 1 {
							textColor = theme.Type
						}
						textFont.DrawRune(contentImg, r, contentX, colY, textColor)
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
		outPath, err := resolveOutPath(opts.OutputPath, filename, opts.SourceTokens, page, totalPages)
		if err != nil {
			return nil, fmt.Errorf("resolve output path: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return nil, fmt.Errorf("create output directory: %w", err)
		}

		f, err := os.Create(outPath)
		if err != nil {
			return nil, fmt.Errorf("create output file: %w", err)
		}

		if err := png.Encode(f, fullImg); err != nil {
			f.Close()
			return nil, fmt.Errorf("encode PNG: %w", err)
		}
		f.Close()
		outputPaths = append(outputPaths, outPath)
	}

	// Return result
	fullText := strings.Join(lines, "\n")
	stats := ComputeTextTokens(fullText)
	primary := ""
	if len(outputPaths) > 0 {
		primary = outputPaths[0]
	}

	return &RenderResult{
		Files:       outputPaths,
		Width:       firstCardWidth,
		Height:      firstCardHeight,
		Columns:     cols,
		TotalLines:  totalLines,
		TotalPages:  totalPages,
		TokenStats:  stats,
		PrimaryPath: primary,
		Pages:       pages,
	}, nil
}

// drawDot8BraillCell draws a Braille cell as 3x4 dots with special colors for dots 7 and 8.
// Dots 1-6 are drawn in the base color, dot 7 (uppercase) in dot7Color, dot 8 (digit prefix) in dot8Color.
// Layout: column 0 (dots 1,2,3), gap, column 2 (dots 4,5,6), row 3 (dots 7,8 combined).
// Each dot is 1 pixel.
func drawDot8BraillCell(img *image.RGBA, r rune, x, y int, baseColor, dot7Color, dot8Color color.RGBA, colorMode string, cellWidth int) {
	if r == 0x2800 {
		return // Empty cell
	}

	bits := byte(r - 0x2800)

	// Dot positions: col 0 for dots 1,2,3; col 2 for dots 4,5,6; row 3 for dots 7,8.
	// A 4px text pitch adds one trailing pixel; Braille geometry remains 3px.
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
			if colorMode == "red-white" && pos.dot <= 6 {
				// Use logical Braille columns (left=0, right=1). The
				// physical columns are 0 and 2 to preserve the 3px cell.
				if (pos.row+(pos.col/2))%2 == 0 {
					col = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
				} else {
					col = color.RGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff}
				}
			} else if pos.dot == 7 {
				col = dot7Color
			} else if pos.dot == 8 {
				col = dot8Color
			}
			img.SetRGBA(px, py, col)
		}
	}
}
