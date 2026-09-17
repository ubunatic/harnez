package readcard

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// IssueCardItem represents an issue to be rendered in an issues overview / matrix card.
type IssueCardItem struct {
	Number     string
	RawStatus  string
	PlainTitle string
	Path       string
}

// IssueMatrixOptions configures issue matrix card generation.
type IssueMatrixOptions struct {
	Title        string
	Badge        string
	FontName     string
	FontSize     int
	Theme        string
	MaxDimension int
	OutputPath   string
	Columns      int
}

// RenderIssuesMatrixCard renders a list of issues into a dense, multi-column visual overview card.
func RenderIssuesMatrixCard(items []IssueCardItem, opts IssueMatrixOptions) (*RenderResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no issues provided for matrix card")
	}

	if opts.FontSize <= 0 {
		opts.FontSize = 11
	}
	if opts.MaxDimension <= 0 {
		opts.MaxDimension = 1568
	}
	if opts.Title == "" {
		opts.Title = "Issue Tracker Discovery"
	}

	theme := DarkTheme
	if strings.ToLower(opts.Theme) == "light" {
		theme = LightTheme
	}

	font := ResolveFont(opts.FontName, opts.FontSize)
	cw := font.CharWidth
	ch := font.CharHeight
	lineHeight := ch + 2

	numItems := len(items)
	cols := opts.Columns
	if cols <= 0 {
		if numItems <= 5 {
			cols = 1
		} else if numItems <= 15 {
			cols = 2
		} else {
			cols = 3
		}
	}
	if cols > 4 {
		cols = 4
	}
	if cols > numItems {
		cols = numItems
	}

	headerHeight := 36
	paddingX := 16
	paddingY := 12
	colGap := 16
	itemHeaderHeight := 20

	itemsPerCol := (numItems + cols - 1) / cols

	// Measure maximum line lengths across all items
	maxLineLen := 45
	for _, item := range items {
		titleLine := fmt.Sprintf("# %s — %s", item.Number, item.PlainTitle)
		if len(titleLine) > maxLineLen {
			maxLineLen = len(titleLine)
		}
		pathLine := fmt.Sprintf("Path: %s", item.Path)
		if len(pathLine) > maxLineLen {
			maxLineLen = len(pathLine)
		}
	}
	if maxLineLen > 80 {
		maxLineLen = 80
	}

	cardWidth := (cols * ((maxLineLen * cw) + 20)) + ((cols - 1) * colGap) + (paddingX * 2)
	if cardWidth > opts.MaxDimension {
		cardWidth = opts.MaxDimension
	}
	if cardWidth < 700 {
		cardWidth = 700
	}
	colWidth := (cardWidth - (paddingX * 2) - ((cols - 1) * colGap)) / cols

	itemBoxHeight := itemHeaderHeight + (2 * lineHeight) + 12
	colContentHeight := itemsPerCol * (itemBoxHeight + 8)

	cardHeight := headerHeight + (paddingY * 2) + colContentHeight + 8
	if cardHeight > opts.MaxDimension {
		cardHeight = opts.MaxDimension
	}
	if cardHeight < 200 {
		cardHeight = 200
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
		badgeText = fmt.Sprintf("%d issues | %d col | %dx%d px", numItems, cols, cardWidth, cardHeight)
	}
	badgeX := cardWidth - paddingX - (len(badgeText) * cw)
	if badgeX > paddingX+(len(opts.Title)*cw)+10 {
		font.DrawString(img, badgeText, badgeX, 10, theme.BadgeFg)
	}

	var allTextBuilder strings.Builder
	allTextBuilder.WriteString(opts.Title)
	allTextBuilder.WriteString("\n")

	// Render items into columns
	for i, item := range items {
		colIdx := i / itemsPerCol
		rowIdx := i % itemsPerCol
		if colIdx >= cols {
			break
		}

		colX := paddingX + colIdx*(colWidth+colGap)
		itemY := headerHeight + paddingY + rowIdx*(itemBoxHeight+8)

		if itemY+itemBoxHeight > cardHeight-paddingY {
			continue
		}

		// Draw item background box
		drawRect(img, colX, itemY, colWidth, itemBoxHeight, theme.HeaderBg)
		drawStrokeRect(img, colX, itemY, colX+colWidth-1, itemY+itemBoxHeight-1, theme.Border)

		// Header line in box: "# <number> <status>"
		statusCol := theme.Keyword
		rawLower := strings.ToLower(item.RawStatus)
		if strings.Contains(rawLower, "closed") || strings.Contains(rawLower, "resolved") {
			statusCol = theme.String
		} else if strings.Contains(rawLower, "blocked") {
			statusCol = theme.Operator
		} else if strings.Contains(rawLower, "progress") {
			statusCol = theme.Number
		}

		headerLine := fmt.Sprintf("#%s [%s]", item.Number, item.RawStatus)
		font.DrawString(img, headerLine, colX+6, itemY+4, statusCol)

		// Title Line
		titleLine := item.PlainTitle
		maxChars := (colWidth - 12) / cw
		if maxChars > 0 && len(titleLine) > maxChars {
			titleLine = titleLine[:maxChars-1] + "…"
		}
		font.DrawString(img, titleLine, colX+6, itemY+itemHeaderHeight+2, theme.Text)

		// Path line
		pathLine := item.Path
		if maxChars > 0 && len(pathLine) > maxChars {
			pathLine = pathLine[:maxChars-1] + "…"
		}
		font.DrawString(img, pathLine, colX+6, itemY+itemHeaderHeight+lineHeight+4, theme.Comment)

		allTextBuilder.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", item.Number, item.RawStatus, item.PlainTitle, item.Path))
	}

	outPath := opts.OutputPath
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), "harnez_find_issues.png")
	}

	if dir := filepath.Dir(outPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
	}

	f, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create issues matrix png %s: %w", outPath, err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return nil, fmt.Errorf("encode issues matrix png: %w", err)
	}
	f.Close()

	textStats := ComputeTextTokens(allTextBuilder.String())
	imageStats := ComputeImageTokens(textStats.TextTokens, textStats.TextBytes, cardWidth, cardHeight, 1)

	return &RenderResult{
		Files:       []string{outPath},
		Width:       cardWidth,
		Height:      cardHeight,
		Columns:     cols,
		TotalLines:  numItems,
		TotalPages:  1,
		TokenStats:  imageStats,
		PrimaryPath: outPath,
	}, nil
}
