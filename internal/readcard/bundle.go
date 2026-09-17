package readcard

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// CardSection represents one documentation file or text block within a multi-doc bundle card.
type CardSection struct {
	Title    string
	Filename string
	Lines    []string
}

// BundleOptions configures multi-doc card bundling.
type BundleOptions struct {
	Title        string
	Badge        string
	Columns      int    // Number of columns across the card (usually equal to len(Sections), or 2/3)
	FontName     string // Font name: "pixel", "retro", "5x8", "3x5", "standard", "8x16", "7x13" (default: "pixel")
	FontSize     int    // Font size in pixels (default: 8 or 11)
	Theme        string // "dark" (default) or "light"
	MaxDimension int    // Max width/height constraint (default: 1568)
	OutputPath   string // Output PNG file path
}

// RenderBundleCard renders multiple documentation sections into a single bounded multi-column visual card.
func RenderBundleCard(sections []CardSection, opts BundleOptions) (*RenderResult, error) {
	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections provided for bundle card")
	}

	if opts.FontSize <= 0 {
		opts.FontSize = 11
	}
	if opts.MaxDimension <= 0 {
		opts.MaxDimension = 1568
	}
	if opts.Title == "" {
		opts.Title = "Developer Cheatsheet"
	}

	theme := DarkTheme
	if strings.ToLower(opts.Theme) == "light" {
		theme = LightTheme
	}

	font := ResolveFont(opts.FontName, opts.FontSize)
	cw := font.CharWidth
	ch := font.CharHeight
	lineHeight := ch + 2

	cols := opts.Columns
	if cols <= 0 {
		cols = len(sections)
	}
	if cols > 4 {
		cols = 4
	}

	headerHeight := 36
	paddingX := 16
	paddingY := 12
	colGap := 16
	secHeaderHeight := 24

	// Determine max lines per column across all sections to bound height
	maxSecLines := 0
	totalLines := 0
	allTextBuilder := strings.Builder{}

	for _, sec := range sections {
		totalLines += len(sec.Lines)
		if len(sec.Lines) > maxSecLines {
			maxSecLines = len(sec.Lines)
		}
		for _, l := range sec.Lines {
			allTextBuilder.WriteString(l)
			allTextBuilder.WriteString("\n")
		}
	}

	// Calculate max column width
	maxLineLen := 45
	for _, sec := range sections {
		for _, l := range sec.Lines {
			l = strings.ReplaceAll(l, "\t", "    ")
			if len(l) > maxLineLen {
				maxLineLen = len(l)
			}
		}
	}
	if maxLineLen > 80 {
		maxLineLen = 80
	}

	// Layout dimensions
	cardWidth := (cols * ((maxLineLen * cw) + 20)) + ((cols - 1) * colGap) + (paddingX * 2)
	if cardWidth > opts.MaxDimension {
		cardWidth = opts.MaxDimension
	}
	if cardWidth < 900 {
		cardWidth = 900
	}
	colWidth := (cardWidth - (paddingX * 2) - ((cols - 1) * colGap)) / cols

	cardHeight := headerHeight + (paddingY * 2) + secHeaderHeight + (maxSecLines * lineHeight) + 16
	if cardHeight > opts.MaxDimension {
		cardHeight = opts.MaxDimension
	}
	if cardHeight < 300 {
		cardHeight = 300
	}

	img := image.NewRGBA(image.Rect(0, 0, cardWidth, cardHeight))
	drawRect(img, 0, 0, cardWidth, cardHeight, theme.Bg)
	drawStrokeRect(img, 0, 0, cardWidth-1, cardHeight-1, theme.Border)

	// Header Bar
	drawRect(img, 1, 1, cardWidth-2, headerHeight, theme.HeaderBg)
	drawHorizontalLine(img, 1, cardWidth-2, headerHeight, theme.Border)

	// Header Title
	font.DrawString(img, opts.Title, paddingX, 10, theme.HeaderFg)

	// Header Badge
	badgeText := opts.Badge
	if badgeText == "" {
		badgeText = fmt.Sprintf("%d sections | %d col | %dx%d px", len(sections), cols, cardWidth, cardHeight)
	}
	badgeX := cardWidth - paddingX - (len(badgeText) * cw)
	if badgeX > paddingX+(len(opts.Title)*cw)+10 {
		font.DrawString(img, badgeText, badgeX, 10, theme.BadgeFg)
	}

	availableSecLines := (cardHeight - headerHeight - (paddingY * 2) - secHeaderHeight - 8) / lineHeight
	if availableSecLines < 5 {
		availableSecLines = 5
	}

	// Render each column section
	for c := 0; c < cols && c < len(sections); c++ {
		sec := sections[c]
		colX := paddingX + c*(colWidth+colGap)
		colY := headerHeight + paddingY

		// Column separator
		if c > 0 {
			sepX := colX - (colGap / 2)
			drawVerticalLine(img, sepX, colY, cardHeight-paddingY, theme.ColumnSep)
		}

		// Section Header
		drawRect(img, colX, colY, colWidth, secHeaderHeight, theme.HeaderBg)
		drawStrokeRect(img, colX, colY, colX+colWidth-1, colY+secHeaderHeight-1, theme.Border)
		secTitle := sec.Title
		if secTitle == "" {
			secTitle = sec.Filename
		}
		font.DrawString(img, secTitle, colX+8, colY+5, theme.Keyword)

		contentY := colY + secHeaderHeight + 6
		ext := filepath.Ext(sec.Filename)
		var inMultiComment bool

		linesToRender := sec.Lines
		if len(linesToRender) > availableSecLines {
			linesToRender = linesToRender[:availableSecLines]
		}

		for lineIdx, rawLine := range linesToRender {
			curY := contentY + (lineIdx * lineHeight)
			tokens := HighlightLine(rawLine, ext, &inMultiComment)
			tokenX := colX + 4
			maxSecX := colX + colWidth - 4
			for _, tok := range tokens {
				tokCol := tokenColor(tok.Type, theme)
				tokenX += font.DrawStringBounded(img, tok.Text, tokenX, curY, maxSecX, tokCol)
				if tokenX >= maxSecX {
					break
				}
			}
		}
	}

	outPath := opts.OutputPath
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), "harnez_bundle_card.png")
	}

	if dir := filepath.Dir(outPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create bundle png %s: %w", outPath, err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return nil, fmt.Errorf("encode bundle png: %w", err)
	}
	f.Close()

	textStats := ComputeTextTokens(allTextBuilder.String())
	imageStats := ComputeImageTokens(textStats.TextTokens, textStats.TextBytes, cardWidth, cardHeight, 1)

	return &RenderResult{
		Files:       []string{outPath},
		Width:       cardWidth,
		Height:      cardHeight,
		Columns:     cols,
		TotalLines:  totalLines,
		TotalPages:  1,
		TokenStats:  imageStats,
		PrimaryPath: outPath,
	}, nil
}
