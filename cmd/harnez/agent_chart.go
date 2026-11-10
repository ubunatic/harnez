package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"
	"ubunatic.com/harnez/internal/braillecanvas"
	"ubunatic.com/harnez/internal/subagent"
)

type chartPoint struct {
	spec     string
	provider string
	alias    string
	tier     string
	cost     float64
	rawCost  float64
	rating   float64
	source   string
	x        int
	y        int
}

type modelSeries struct {
	provider string
	alias    string
	points   []chartPoint
}

func renderModelsChart(out io.Writer, entries []subagent.ModelEntry, width int, costMode string, colorEnabled bool) error {
	const minWidth = 60
	if width < minWidth {
		return fmt.Errorf("terminal width %d is too narrow for --chart (minimum %d columns)", width, minWidth)
	}

	scores, err := subagent.LoadModelScores()
	if err != nil {
		return fmt.Errorf("load model scores: %w", err)
	}

	var plotted []chartPoint
	var unplotted []string

	seriesMap := make(map[string]*modelSeries)
	var seriesOrder []string

	for _, e := range entries {
		alias := e.Model.Name
		parts := strings.Split(e.Spec, ":")
		if len(parts) >= 2 {
			alias = parts[1]
		}

		rating, source, ok := scores.Rating(e.Model.Provider, alias, e.Model.Tier)
		if !ok {
			unplotted = append(unplotted, fmt.Sprintf("%s (no rating)", e.Spec))
			continue
		}

		rawCost := float64(e.Cost)
		cost := rawCost
		if costMode == "effective" {
			cost = rawCost * scores.Multiplier(e.Model.Provider)
		}

		if cost <= 0 {
			unplotted = append(unplotted, fmt.Sprintf("%s (cost <= 0)", e.Spec))
			continue
		}

		pt := chartPoint{
			spec:     e.Spec,
			provider: e.Model.Provider,
			alias:    alias,
			tier:     e.Model.Tier,
			cost:     cost,
			rawCost:  rawCost,
			rating:   rating,
			source:   source,
		}
		plotted = append(plotted, pt)

		key := e.Model.Provider + ":" + alias
		s, exists := seriesMap[key]
		if !exists {
			s = &modelSeries{
				provider: e.Model.Provider,
				alias:    alias,
			}
			seriesMap[key] = s
			seriesOrder = append(seriesOrder, key)
		}
		s.points = append(s.points, pt)
	}

	if len(plotted) == 0 {
		return fmt.Errorf("no models with valid cost and capability rating to plot")
	}

	// Calculate Pareto frontier
	pareto := computeParetoFrontier(plotted)
	paretoSpecs := make(map[string]bool, len(pareto))
	for _, p := range pareto {
		paretoSpecs[p.spec] = true
	}

	// Determine cost bounds
	minCost := plotted[0].cost
	maxCost := plotted[0].cost
	for _, p := range plotted {
		if p.cost < minCost {
			minCost = p.cost
		}
		if p.cost > maxCost {
			maxCost = p.cost
		}
	}
	if maxCost <= minCost {
		maxCost = minCost * 10
	}

	logMin := math.Log10(minCost)
	logMax := math.Log10(maxCost)
	if logMax <= logMin {
		logMax = logMin + 1
	}

	const yAxisMargin = 6
	canvasWidth := width - yAxisMargin - 2
	if canvasWidth < 20 {
		canvasWidth = 20
	}
	const canvasHeight = 12
	canvas := braillecanvas.New(canvasWidth, canvasHeight)

	scaleMin := scores.Scale.Min
	scaleMax := scores.Scale.Max
	if scaleMax <= scaleMin {
		scaleMin = 1.0
		scaleMax = 5.0
	}

	mapX := func(cost float64) int {
		if cost <= 0 {
			return 0
		}
		norm := (math.Log10(cost) - logMin) / (logMax - logMin)
		if norm < 0 {
			norm = 0
		}
		if norm > 1 {
			norm = 1
		}
		return int(math.Round(norm * float64(canvas.WidthDots()-1)))
	}

	mapY := func(rating float64) int {
		norm := (rating - scaleMin) / (scaleMax - scaleMin)
		if norm < 0 {
			norm = 0
		}
		if norm > 1 {
			norm = 1
		}
		return int(math.Round(norm * float64(canvas.HeightDots()-1)))
	}

	// Map all points and draw series
	for _, key := range seriesOrder {
		s := seriesMap[key]
		sort.Slice(s.points, func(i, j int) bool {
			return s.points[i].cost < s.points[j].cost
		})

		colorCode := scores.ProviderColor(s.provider)
		for i := range s.points {
			s.points[i].x = mapX(s.points[i].cost)
			s.points[i].y = mapY(s.points[i].rating)
		}

		// Connect points across effort tiers with Braille lines
		for i := 0; i < len(s.points)-1; i++ {
			p0 := s.points[i]
			p1 := s.points[i+1]
			canvas.Line(p0.x, p0.y, p1.x, p1.y, colorCode)
		}

		// Set point dots
		for _, p := range s.points {
			canvas.Set(p.x, p.y, colorCode)
			// Small cross/dot to make points visible
			if p.x > 0 {
				canvas.Set(p.x-1, p.y, colorCode)
			}
			if p.x < canvas.WidthDots()-1 {
				canvas.Set(p.x+1, p.y, colorCode)
			}
			if p.y > 0 {
				canvas.Set(p.x, p.y-1, colorCode)
			}
			if p.y < canvas.HeightDots()-1 {
				canvas.Set(p.x, p.y+1, colorCode)
			}
		}

		// Label series near highest cost point
		last := s.points[len(s.points)-1]
		labelCellX := (last.x / 2) + 1
		labelCellY := (canvas.HeightCells() - 1) - (last.y / 4)
		if labelCellX+len(s.alias) >= canvas.WidthCells() {
			labelCellX = (last.x / 2) - len(s.alias) - 1
		}
		if canvas.IsOccupied(labelCellX, labelCellY, len(s.alias)) {
			if labelCellY > 0 && !canvas.IsOccupied(labelCellX, labelCellY-1, len(s.alias)) {
				labelCellY--
			} else if labelCellY < canvas.HeightCells()-1 && !canvas.IsOccupied(labelCellX, labelCellY+1, len(s.alias)) {
				labelCellY++
			}
		}
		if labelCellX >= 0 && labelCellX < canvas.WidthCells() {
			canvas.Text(labelCellX, labelCellY, s.alias, colorCode)
		}
	}

	// Draw Pareto frontier line
	if len(pareto) > 1 {
		for i := 0; i < len(pareto)-1; i++ {
			px0 := mapX(pareto[i].cost)
			py0 := mapY(pareto[i].rating)
			px1 := mapX(pareto[i+1].cost)
			py1 := mapY(pareto[i+1].rating)
			canvas.Line(px0, py0, px1, py1, "1;37") // bold white line
		}
	}

	// Title
	basisLabel := "raw API cost"
	if costMode == "effective" {
		basisLabel = "effective cost (subscription-adjusted)"
	}
	fmt.Fprintf(out, "Cost vs Capability Chart (%s, Log Scale)\n", basisLabel)
	fmt.Fprintf(out, "Y: Capability Rating (1-5 stars) · X: Relative Cost · Pareto Frontier Marked (bold)\n\n")

	// Render Y axis and canvas rows
	canvasLines := canvas.RenderLines(colorEnabled)
	for row := 0; row < canvas.HeightCells(); row++ {
		var yLabel string
		switch row {
		case 0:
			yLabel = "5.0 ┤"
		case 3:
			yLabel = "4.0 ┤"
		case 6:
			yLabel = "3.0 ┤"
		case 8:
			yLabel = "2.0 ┤"
		case 11:
			yLabel = "1.0 ┤"
		default:
			yLabel = "    │"
		}
		fmt.Fprintf(out, "%s %s\n", yLabel, canvasLines[row])
	}

	// Bottom axis line
	var axisLine strings.Builder
	axisLine.WriteString("    └")

	ticks := generateLogTicks(minCost, maxCost)
	tickCols := make(map[int]string)
	for _, t := range ticks {
		norm := (math.Log10(t) - logMin) / (logMax - logMin)
		col := int(math.Round(norm * float64(canvasWidth-1)))
		if col >= 0 && col < canvasWidth {
			label := formatTickVal(t)
			tickCols[col] = label
		}
	}

	for col := 0; col < canvasWidth; col++ {
		if _, ok := tickCols[col]; ok {
			axisLine.WriteRune('┬')
		} else {
			axisLine.WriteRune('─')
		}
	}
	fmt.Fprintln(out, axisLine.String())

	// Tick labels line
	tickLine := make([]rune, canvasWidth+10)
	for i := range tickLine {
		tickLine[i] = ' '
	}
	for col, label := range tickCols {
		start := col - (len(label) / 2)
		if start < 0 {
			start = 0
		}
		if start+len(label) < len(tickLine) {
			copy(tickLine[start:], []rune(label))
		}
	}
	fmt.Fprintf(out, "     %s\n", strings.TrimRight(string(tickLine), " "))

	// X-axis label
	xLabel := "Relative Cost (raw API, codex:luna:low = 1)"
	if costMode == "effective" {
		xLabel = "Effective Cost (subscription-adjusted, codex:luna:low = 1)"
	}
	fmt.Fprintf(out, "     %s\n\n", xLabel)

	// Pareto frontier summary
	var paretoSummaries []string
	for _, p := range pareto {
		paretoSummaries = append(paretoSummaries, fmt.Sprintf("%s (cost %.2g, ★%.1f)", p.spec, p.cost, p.rating))
	}
	fmt.Fprintf(out, "Pareto frontier: %s\n\n", strings.Join(paretoSummaries, " → "))

	// Legend & sources
	fmt.Fprint(out, "Providers: ")
	provList := []string{"codex", "claude", "agy"}
	var provBadges []string
	for _, p := range provList {
		color := scores.ProviderColor(p)
		if colorEnabled {
			provBadges = append(provBadges, fmt.Sprintf("\x1b[%sm■ %s\x1b[0m", color, p))
		} else {
			provBadges = append(provBadges, fmt.Sprintf("[%s]", p))
		}
	}
	fmt.Fprintln(out, strings.Join(provBadges, "  "))
	fmt.Fprintf(out, "Rating source: spec/model_scores.yaml (%s)\n", scores.Models["codex:luna"].Source)
	fmt.Fprintf(out, "Cost source: spec/agent.yaml cost matrix (%s)\n", basisLabel)

	if len(unplotted) > 0 {
		sort.Strings(unplotted)
		fmt.Fprintf(out, "\nModels not plotted (%d): %s\n", len(unplotted), strings.Join(unplotted, ", "))
	}

	return nil
}

func computeParetoFrontier(points []chartPoint) []chartPoint {
	if len(points) == 0 {
		return nil
	}
	// Sort primarily by cost ascending, secondarily by rating descending
	sorted := make([]chartPoint, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].cost != sorted[j].cost {
			return sorted[i].cost < sorted[j].cost
		}
		return sorted[i].rating > sorted[j].rating
	})

	var frontier []chartPoint
	maxRating := -1.0
	for _, pt := range sorted {
		if pt.rating > maxRating {
			frontier = append(frontier, pt)
			maxRating = pt.rating
		}
	}
	return frontier
}

func generateLogTicks(minCost, maxCost float64) []float64 {
	bases := []float64{0.1, 0.25, 0.5, 1, 2, 4, 10, 25, 50, 100, 200, 400}
	var ticks []float64
	for _, b := range bases {
		if b >= minCost*0.95 && b <= maxCost*1.05 {
			ticks = append(ticks, b)
		}
	}
	if len(ticks) == 0 {
		ticks = []float64{minCost, maxCost}
	}
	return ticks
}

func formatTickVal(val float64) string {
	if val >= 1 && val == math.Floor(val) {
		return fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%.2g", val)
}

func terminalWidth(out io.Writer) int {
	if file, ok := out.(*os.File); ok {
		if w, _, err := term.GetSize(int(file.Fd())); err == nil && w > 0 {
			return w
		}
	}
	return 80
}
