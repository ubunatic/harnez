package readcard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// RenderOptions configures visual image card generation.
type RenderOptions struct {
	Columns         int    // Number of columns (1, 2, 3, 4, or 0 for auto)
	FontName        string // Font name: "pixel", "retro", "5x8", "3x5", "standard", "8x16", "7x13" (default: "pixel")
	FontSize        int    // Font size in pixels (default: 8 or 11)
	Theme           string // "dark" (default) or "light"
	Wrap            string // "soft" (default) or "truncate"
	MaxDimension    int    // Max width/height constraint (default: 1568)
	ShowLineNumbers bool   // Print line numbers in gutter (default: true)
	OutputPath      string // Custom output PNG path (or directory)
	SourceTokens    int    // Source text token count used in generated default names
	Title           string // Card title / filename
	StartLine       int    // Starting line number (1-indexed, default 1)
	LineNumbers     string // all, off, or positive cadence; overrides ShowLineNumbers when set
	SourceLines     []int  // Optional original source anchors after compression
	MeasureOnly     bool   // Compute exact page geometry and costs without creating PNGs
}

// RenderResult contains the generated image paths and token statistics.
type RenderResult struct {
	Files              []string       `json:"files"`
	Width              int            `json:"width"`
	Height             int            `json:"height"`
	Columns            int            `json:"columns"`
	TotalLines         int            `json:"total_lines"`
	TotalPages         int            `json:"total_pages"`
	TokenStats         TokenStats     `json:"token_stats"`
	PrimaryPath        string         `json:"primary_path"`
	Pages              []PageGeometry `json:"pages"`
	OriginalTokenStats *TokenStats    `json:"original_token_stats,omitempty"`
}

// PageGeometry describes a single tightly cropped page.
type PageGeometry struct {
	Width   int `json:"width"`
	Height  int `json:"height"`
	Columns int `json:"columns"`
}

// ColorTheme defines the palette for syntax highlighting and card canvas.
type ColorTheme struct {
	Bg            color.RGBA
	HeaderBg      color.RGBA
	HeaderFg      color.RGBA
	Border        color.RGBA
	GutterBg      color.RGBA
	GutterFg      color.RGBA
	GutterBorder  color.RGBA
	ColumnSep     color.RGBA
	BadgeBg       color.RGBA
	BadgeFg       color.RGBA
	Text          color.RGBA
	Keyword       color.RGBA
	Type          color.RGBA
	String        color.RGBA
	Comment       color.RGBA
	Number        color.RGBA
	Operator      color.RGBA
	Punctuation   color.RGBA
	HeaderKeyword color.RGBA
}

var DarkTheme = ColorTheme{
	Bg:            color.RGBA{R: 0x0f, G: 0x11, B: 0x17, A: 0xff}, // Deep slate #0f1117
	HeaderBg:      color.RGBA{R: 0x16, G: 0x1b, B: 0x22, A: 0xff}, // Slate header #161b22
	HeaderFg:      color.RGBA{R: 0xf0, G: 0xf6, B: 0xfc, A: 0xff}, // Bright white #f0f6fc
	Border:        color.RGBA{R: 0x30, G: 0x36, B: 0x3d, A: 0xff}, // Border #30363d
	GutterBg:      color.RGBA{R: 0x12, G: 0x15, B: 0x1c, A: 0xff}, // Gutter #12151c
	GutterFg:      color.RGBA{R: 0x6e, G: 0x76, B: 0x81, A: 0xff}, // Gutter text #6e7681
	GutterBorder:  color.RGBA{R: 0x21, G: 0x26, B: 0x2d, A: 0xff}, // Gutter border #21262d
	ColumnSep:     color.RGBA{R: 0x27, G: 0x2c, B: 0x36, A: 0xff}, // Column divider #272c36
	BadgeBg:       color.RGBA{R: 0x1f, G: 0x29, B: 0x3d, A: 0xff}, // Badge background
	BadgeFg:       color.RGBA{R: 0x38, G: 0xbd, B: 0xf8, A: 0xff}, // Badge text #38bdf8
	Text:          color.RGBA{R: 0xe6, G: 0xed, B: 0xf3, A: 0xff}, // #e6edf3
	Keyword:       color.RGBA{R: 0x38, G: 0xbd, B: 0xf8, A: 0xff}, // #38bdf8 (cyan/blue)
	Type:          color.RGBA{R: 0x34, G: 0xd3, B: 0x99, A: 0xff}, // #34d399 (emerald)
	String:        color.RGBA{R: 0x4a, G: 0xde, B: 0x80, A: 0xff}, // #4ade80 (bright green)
	Comment:       color.RGBA{R: 0x8b, G: 0x94, B: 0x9e, A: 0xff}, // #8b949e (slate comment)
	Number:        color.RGBA{R: 0xfb, G: 0xbf, B: 0x24, A: 0xff}, // #fbbf24 (amber)
	Operator:      color.RGBA{R: 0xf4, G: 0x72, B: 0xb6, A: 0xff}, // #f472b6 (rose)
	Punctuation:   color.RGBA{R: 0x94, G: 0xa3, B: 0xb8, A: 0xff}, // #94a3b8
	HeaderKeyword: color.RGBA{R: 0x22, G: 0xd3, B: 0xee, A: 0xff}, // #22d3ee
}

var LightTheme = ColorTheme{
	Bg:            color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	HeaderBg:      color.RGBA{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff},
	HeaderFg:      color.RGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff},
	Border:        color.RGBA{R: 0xd0, G: 0xd7, B: 0xde, A: 0xff},
	GutterBg:      color.RGBA{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff},
	GutterFg:      color.RGBA{R: 0x65, G: 0x6d, B: 0x76, A: 0xff},
	GutterBorder:  color.RGBA{R: 0xe1, G: 0xe4, B: 0xe8, A: 0xff},
	ColumnSep:     color.RGBA{R: 0xd0, G: 0xd7, B: 0xde, A: 0xff},
	BadgeBg:       color.RGBA{R: 0xdd, G: 0xf4, B: 0xff, A: 0xff},
	BadgeFg:       color.RGBA{R: 0x09, G: 0x69, B: 0xda, A: 0xff},
	Text:          color.RGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff},
	Keyword:       color.RGBA{R: 0x09, G: 0x69, B: 0xda, A: 0xff},
	Type:          color.RGBA{R: 0x05, G: 0x50, B: 0xae, A: 0xff},
	String:        color.RGBA{R: 0x1a, G: 0x7f, B: 0x37, A: 0xff},
	Comment:       color.RGBA{R: 0x59, G: 0x63, B: 0x6e, A: 0xff},
	Number:        color.RGBA{R: 0x95, G: 0x38, B: 0x00, A: 0xff},
	Operator:      color.RGBA{R: 0x82, G: 0x50, B: 0xdf, A: 0xff},
	Punctuation:   color.RGBA{R: 0x65, G: 0x6d, B: 0x76, A: 0xff},
	HeaderKeyword: color.RGBA{R: 0x05, G: 0x50, B: 0xae, A: 0xff},
}

// RenderFileToCards renders source code lines into styled PNG card(s) bounded within max dimension.
func RenderFileToCards(lines []string, filename string, opts RenderOptions) (*RenderResult, error) {
	cadence, err := ParseLineNumbers(opts.LineNumbers)
	if err != nil {
		return nil, err
	}
	if opts.LineNumbers != "" {
		opts.ShowLineNumbers = cadence != 0
	}
	if opts.FontSize <= 0 {
		opts.FontSize = 11
	}
	if opts.MaxDimension <= 0 {
		opts.MaxDimension = 1568
	}
	if opts.MaxDimension < 160 {
		return nil, fmt.Errorf("max dimension must be at least 160")
	}
	if len(opts.SourceLines) != 0 && len(opts.SourceLines) != len(lines) {
		return nil, fmt.Errorf("source line mapping length does not match input")
	}
	if opts.StartLine <= 0 {
		opts.StartLine = 1
	}
	if opts.Title == "" {
		opts.Title = filename
	}

	theme := DarkTheme
	if strings.ToLower(opts.Theme) == "light" {
		theme = LightTheme
	}

	font := ResolveFont(opts.FontName, opts.FontSize)
	cw := font.CharWidth
	ch := font.CharHeight
	lineHeight := ch + 2

	totalLines := len(lines)
	if totalLines == 0 {
		lines = []string{"(empty file)"}
		totalLines = 1
	}

	// Calculate raw longest line
	rawMaxLineLen := 0
	for _, l := range lines {
		l = strings.ReplaceAll(l, "\t", "    ")
		rCount := len([]rune(l))
		if rCount > rawMaxLineLen {
			rawMaxLineLen = rCount
		}
	}

	// Gutter width: digits of (startLine + totalLines)
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
		gutterWidth = (digits+1)*cw + 12
	}

	// Layout geometry
	headerHeight := 36
	paddingX := 16
	paddingY := 12
	colGap := 16

	availableHeight := opts.MaxDimension - headerHeight - (paddingY * 2) - 4
	if availableHeight < lineHeight {
		return nil, fmt.Errorf("max dimension too small for selected font")
	}
	linesPerCol := availableHeight / lineHeight
	if linesPerCol < 1 {
		linesPerCol = 1
	}

	maxColChars1Col := (opts.MaxDimension - (paddingX * 2) - gutterWidth - 16) / cw
	if maxColChars1Col < 35 {
		maxColChars1Col = 35
	}

	// Determine column count
	cols := opts.Columns
	if cols <= 0 {
		cols = 3
	}
	if cols > 4 {
		cols = 4
	}
	// A column must fit a gutter and at least three code glyphs, including
	// continuation markers. Reduce columns before wrapping, never clip rows.
	for cols > 1 && (opts.MaxDimension-2*paddingX-(cols-1)*colGap)/cols < gutterWidth+16+3*cw {
		cols--
	}

	maxLineLen := rawMaxLineLen
	if maxLineLen < 35 {
		maxLineLen = 35
	}
	if cols == 1 {
		// In 1-column mode, expand up to available canvas width (~240 chars at 1568px)
		if maxLineLen > maxColChars1Col {
			maxLineLen = maxColChars1Col
		}
	} else {
		maxColCharsMulti := ((opts.MaxDimension-(paddingX*2)-((cols-1)*colGap))/cols - gutterWidth - 16) / cw
		if maxColCharsMulti < 35 {
			maxColCharsMulti = 35
		}
		if maxLineLen > maxColCharsMulti {
			maxLineLen = maxColCharsMulti
		}
	}

	colWidth := gutterWidth + (maxLineLen * cw) + 16
	totalContentWidth := (cols * colWidth) + ((cols - 1) * colGap)
	cardWidth := totalContentWidth + (paddingX * 2)
	if cardWidth > opts.MaxDimension {
		cardWidth = opts.MaxDimension
		colWidth = (cardWidth - (paddingX * 2) - ((cols - 1) * colGap)) / cols
	}

	// Available character capacity in each column
	availCodeChars := (colWidth - gutterWidth - 16) / cw
	if availCodeChars < 3 {
		return nil, fmt.Errorf("max dimension too small for selected font and gutter")
	}

	wrapMode := strings.ToLower(strings.TrimSpace(opts.Wrap))
	isTruncate := wrapMode == "truncate"

	ext := filepath.Ext(filename)
	lang := DetectLanguage(filename)
	var inMultiComment bool
	var allRows []renderRow

	for i, rawLine := range lines {
		actualLineNum := opts.StartLine + i
		if len(opts.SourceLines) > 0 {
			actualLineNum = opts.SourceLines[i]
		}
		expandedLine := strings.ReplaceAll(rawLine, "\t", "    ")
		tokens := HighlightLine(expandedLine, ext, &inMultiComment)

		if isTruncate {
			totalRunes := 0
			for _, tok := range tokens {
				totalRunes += len([]rune(tok.Text))
			}
			if totalRunes > availCodeChars {
				headTokens, _ := splitTokensByLength(tokens, availCodeChars-1)
				headTokens = append(headTokens, Token{Type: TokenComment, Text: "…"})
				allRows = append(allRows, renderRow{
					lineNum:        actualLineNum,
					isContinuation: false,
					tokens:         headTokens,
				})
			} else {
				allRows = append(allRows, renderRow{
					lineNum:        actualLineNum,
					isContinuation: false,
					tokens:         tokens,
				})
			}
		} else {
			// Soft-wrap mode (default)
			remainingTokens := tokens
			totalRunes := 0
			for _, tok := range remainingTokens {
				totalRunes += len([]rune(tok.Text))
			}

			if totalRunes <= availCodeChars {
				allRows = append(allRows, renderRow{
					lineNum:        actualLineNum,
					isContinuation: false,
					tokens:         tokens,
				})
			} else {
				head, rest := splitTokensByLength(remainingTokens, availCodeChars)
				allRows = append(allRows, renderRow{
					lineNum:        actualLineNum,
					isContinuation: false,
					tokens:         head,
				})
				remainingTokens = rest

				contBudget := availCodeChars - 2
				for len(remainingTokens) > 0 {
					contHead, contRest := splitTokensByLength(remainingTokens, contBudget)
					chunkTokens := append([]Token{{Type: TokenComment, Text: "↳ "}}, contHead...)
					allRows = append(allRows, renderRow{
						lineNum:        0, // blank gutter for continuation
						isContinuation: true,
						tokens:         chunkTokens,
					})
					remainingTokens = contRest
				}
			}
		}
	}

	linesPerPage := linesPerCol * cols
	totalPages := (len(allRows) + linesPerPage - 1) / linesPerPage
	if totalPages == 0 {
		totalPages = 1
	}

	var outputPaths []string
	firstCardHeight := 0
	firstCardWidth, firstColumns := 0, 0
	var pages []PageGeometry
	imageStats := ComputeTextTokens(strings.Join(lines, "\n"))

	for page := 0; page < totalPages; page++ {
		pageStartIdx := page * linesPerPage
		pageEndIdx := pageStartIdx + linesPerPage
		if pageEndIdx > len(allRows) {
			pageEndIdx = len(allRows)
		}
		pageRows := allRows[pageStartIdx:pageEndIdx]

		// Calculate dynamic card height for this page
		pageRowsCount := len(pageRows)
		// Short pages need one column; larger pages are balanced across up to
		// three default columns. Twenty rows is a density target, not a limit.
		usedCols := max((pageRowsCount+linesPerCol-1)/linesPerCol, min(cols, max(1, (pageRowsCount+19)/20)))
		pageColLines := (pageRowsCount + usedCols - 1) / usedCols
		actualMax := 0
		for _, row := range pageRows {
			width := 0
			for _, tok := range row.tokens {
				width += len([]rune(tok.Text))
			}
			actualMax = max(actualMax, width)
		}
		colWidth := gutterWidth + actualMax*cw + 16
		titleText := opts.Title
		if totalPages > 1 {
			titleText = fmt.Sprintf("%s (Page %d/%d)", opts.Title, page+1, totalPages)
		}
		badgeText := fmt.Sprintf("[%s] %d lines | %d col | page %d/%d", lang, totalLines, usedCols, page+1, totalPages)
		headerMin := (len([]rune(titleText))+len([]rune(badgeText)))*cw + 2*paddingX + 32
		cardWidth := min(opts.MaxDimension, max(usedCols*colWidth+(usedCols-1)*colGap+2*paddingX, headerMin))
		cardHeight := headerHeight + (paddingY * 2) + (pageColLines * lineHeight) + 8
		if cardHeight > opts.MaxDimension {
			cardHeight = opts.MaxDimension
		}
		if cardHeight < 120 {
			cardHeight = 120
		}
		if page == 0 {
			firstCardHeight = cardHeight
			firstCardWidth, firstColumns = cardWidth, usedCols
		}
		pages = append(pages, PageGeometry{Width: cardWidth, Height: cardHeight, Columns: usedCols})
		pageStats := ComputeImageTokens(0, 0, cardWidth, cardHeight, 1)
		imageStats.ClaudeTokens += pageStats.ClaudeTokens
		imageStats.OpenAITokens += pageStats.OpenAITokens
		imageStats.GeminiTokens += pageStats.GeminiTokens
		if opts.MeasureOnly {
			continue
		}

		img := image.NewRGBA(image.Rect(0, 0, cardWidth, cardHeight))
		drawRect(img, 0, 0, cardWidth, cardHeight, theme.Bg)

		// Outer card border
		drawStrokeRect(img, 0, 0, cardWidth-1, cardHeight-1, theme.Border)

		// Header Bar
		drawRect(img, 1, 1, cardWidth-2, headerHeight, theme.HeaderBg)
		drawHorizontalLine(img, 1, cardWidth-2, headerHeight, theme.Border)

		// Header Title
		font.DrawStringBounded(img, titleText, paddingX, 10, cardWidth-paddingX, theme.HeaderFg)

		// Header Badges
		badgeX := cardWidth - paddingX - (len([]rune(badgeText)) * cw)
		if badgeX > paddingX+(len([]rune(titleText))*cw)+10 {
			font.DrawString(img, badgeText, badgeX, 10, theme.BadgeFg)
		}

		// Render Columns
		stride := pageColLines
		if stride < 1 {
			stride = 1
		}
		for c := 0; c < usedCols; c++ {
			cStartIdx := c * stride
			if cStartIdx >= len(pageRows) {
				break
			}
			cEndIdx := cStartIdx + stride
			if cEndIdx > len(pageRows) {
				cEndIdx = len(pageRows)
			}
			colRows := pageRows[cStartIdx:cEndIdx]

			colX := paddingX + c*(colWidth+colGap)
			colY := headerHeight + paddingY

			// Column Separator
			if c > 0 {
				sepX := colX - (colGap / 2)
				drawVerticalLine(img, sepX, colY, colY+(pageColLines*lineHeight), theme.ColumnSep)
			}

			// Gutter Background
			if opts.ShowLineNumbers && gutterWidth > 0 {
				drawRect(img, colX, colY, gutterWidth-4, len(colRows)*lineHeight, theme.GutterBg)
				drawVerticalLine(img, colX+gutterWidth-4, colY, colY+(len(colRows)*lineHeight), theme.GutterBorder)
			}

			// Render lines in column
			for rowIdx, rrow := range colRows {
				curY := colY + (rowIdx * lineHeight) + 2

				// Draw Line Number if not continuation
				if opts.ShowLineNumbers && !rrow.isContinuation && rrow.lineNum > 0 {
					numStr := fmt.Sprintf("%*d", digits, rrow.lineNum)
					if cadence > 1 && rrow.lineNum != opts.StartLine && rrow.lineNum%cadence != 0 {
						numStr = fmt.Sprintf("%*s", digits, ".")
					}
					font.DrawString(img, numStr, colX+4, curY, theme.GutterFg)
				}

				// Draw Code
				codeX := colX + gutterWidth
				tokenX := codeX
				maxColX := colX + colWidth - 4
				if c == usedCols-1 {
					maxColX = cardWidth - paddingX
				}
				for _, tok := range rrow.tokens {
					tokCol := tokenColor(tok.Type, theme)
					tokenX += font.DrawStringBounded(img, tok.Text, tokenX, curY, maxColX, tokCol)
					if tokenX >= maxColX {
						break // visually wrap/clip at column edge
					}
				}
			}
		}

		// Determine target file path
		outPath, err := resolveOutPath(opts.OutputPath, filename, opts.SourceTokens, page, totalPages)
		if err != nil {
			return nil, fmt.Errorf("resolve output path: %w", err)
		}

		f, err := os.Create(outPath)
		if err != nil {
			return nil, fmt.Errorf("create image file %s: %w", outPath, err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return nil, fmt.Errorf("encode png to %s: %w", outPath, err)
		}
		f.Close()

		outputPaths = append(outputPaths, outPath)
	}

	// Compute token stats
	imageStats.ImageWidth, imageStats.ImageHeight, imageStats.TotalPages = firstCardWidth, firstCardHeight, totalPages
	if imageStats.ClaudeTokens > 0 {
		imageStats.ClaudeRatio = round2(float64(imageStats.TextTokens) / float64(imageStats.ClaudeTokens))
	}
	if imageStats.OpenAITokens > 0 {
		imageStats.OpenAIRatio = round2(float64(imageStats.TextTokens) / float64(imageStats.OpenAITokens))
	}
	if imageStats.GeminiTokens > 0 {
		imageStats.GeminiRatio = round2(float64(imageStats.TextTokens) / float64(imageStats.GeminiTokens))
	}

	primary := ""
	if len(outputPaths) > 0 {
		primary = outputPaths[0]
	}

	return &RenderResult{
		Files:       outputPaths,
		Width:       firstCardWidth,
		Height:      firstCardHeight,
		Columns:     firstColumns,
		TotalLines:  totalLines,
		TotalPages:  totalPages,
		TokenStats:  imageStats,
		PrimaryPath: primary,
		Pages:       pages,
	}, nil
}

func tokenColor(t TokenType, theme ColorTheme) color.RGBA {
	switch t {
	case TokenKeyword:
		return theme.Keyword
	case TokenTypeIdent:
		return theme.Type
	case TokenString:
		return theme.String
	case TokenComment:
		return theme.Comment
	case TokenNumber:
		return theme.Number
	case TokenOperator:
		return theme.Operator
	case TokenPunctuation:
		return theme.Punctuation
	case TokenHeader:
		return theme.HeaderKeyword
	case TokenList:
		return theme.Operator
	default:
		return theme.Text
	}
}

func resolveOutPath(customOut string, sourceFile string, sourceTokens, page, totalPages int) (string, error) {
	if customOut != "" {
		if fi, err := os.Stat(customOut); err == nil && fi.IsDir() {
			base := filepath.Base(sourceFile)
			stem := strings.TrimSuffix(base, filepath.Ext(base))
			if totalPages > 1 {
				return filepath.Join(customOut, fmt.Sprintf("%s_page%d.png", stem, page+1)), nil
			}
			return filepath.Join(customOut, fmt.Sprintf("%s.png", stem)), nil
		}
		if totalPages > 1 {
			ext := filepath.Ext(customOut)
			stem := strings.TrimSuffix(customOut, ext)
			if ext == "" {
				ext = ".png"
			}
			return fmt.Sprintf("%s_page%d%s", stem, page+1, ext), nil
		}
		return customOut, nil
	}

	// Default to OS Temp Dir with deterministic / readable name
	base := filepath.Base(sourceFile)
	h := sha256.Sum256([]byte(sourceFile))
	shortHash := hex.EncodeToString(h[:4])

	tmpDir := os.TempDir()
	if totalPages > 1 {
		return filepath.Join(tmpDir, fmt.Sprintf("harnez_read_%s_%d-tokens_%s_p%d.png", base, sourceTokens, shortHash, page+1)), nil
	}
	return filepath.Join(tmpDir, fmt.Sprintf("harnez_read_%s_%d-tokens_%s.png", base, sourceTokens, shortHash)), nil
}

func drawRect(img *image.RGBA, x, y, w, h int, col color.RGBA) {
	bounds := img.Bounds()
	for py := y; py < y+h && py < bounds.Max.Y; py++ {
		if py < bounds.Min.Y {
			continue
		}
		for px := x; px < x+w && px < bounds.Max.X; px++ {
			if px < bounds.Min.X {
				continue
			}
			img.SetRGBA(px, py, col)
		}
	}
}

func drawStrokeRect(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA) {
	drawHorizontalLine(img, x0, x1, y0, col)
	drawHorizontalLine(img, x0, x1, y1, col)
	drawVerticalLine(img, x0, y0, y1, col)
	drawVerticalLine(img, x1, y0, y1, col)
}

func drawHorizontalLine(img *image.RGBA, x0, x1, y int, col color.RGBA) {
	bounds := img.Bounds()
	if y < bounds.Min.Y || y >= bounds.Max.Y {
		return
	}
	for px := x0; px <= x1; px++ {
		if px >= bounds.Min.X && px < bounds.Max.X {
			img.SetRGBA(px, y, col)
		}
	}
}

func drawVerticalLine(img *image.RGBA, x, y0, y1 int, col color.RGBA) {
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X {
		return
	}
	for py := y0; py <= y1; py++ {
		if py >= bounds.Min.Y && py < bounds.Max.Y {
			img.SetRGBA(x, py, col)
		}
	}
}

type renderRow struct {
	lineNum        int
	isContinuation bool
	tokens         []Token
}

func splitTokensByLength(tokens []Token, maxLen int) (head []Token, tail []Token) {
	if maxLen <= 0 {
		return nil, tokens
	}
	curLen := 0
	for i, tok := range tokens {
		tokRunes := []rune(tok.Text)
		tokLen := len(tokRunes)
		if curLen+tokLen <= maxLen {
			head = append(head, tok)
			curLen += tokLen
			if curLen == maxLen {
				if i+1 < len(tokens) {
					tail = tokens[i+1:]
				}
				return head, tail
			}
		} else {
			needed := maxLen - curLen
			headTok := Token{
				Type: tok.Type,
				Text: string(tokRunes[:needed]),
			}
			head = append(head, headTok)

			tailTok := Token{
				Type: tok.Type,
				Text: string(tokRunes[needed:]),
			}
			tail = append([]Token{tailTok}, tokens[i+1:]...)
			return head, tail
		}
	}
	return head, nil
}
