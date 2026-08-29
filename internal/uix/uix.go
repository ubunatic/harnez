// Package uix provides small terminal box layout and rendering helpers.
package uix

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Box describes a terminal panel before layout.
type Box struct {
	ID        string
	Title     string
	Lines     []string
	MinWidth  int
	PrefWidth int
	MaxWidth  int
	Stretch   bool
	Priority  int
	Order     int
	Enabled   bool
}

// PlannedBox is a visible box with an assigned width.
type PlannedBox struct {
	Box
	Width int
}

// Row is one left-to-right layout row.
type Row struct {
	Boxes []PlannedBox
}

// Plan is the complete layout result for a terminal width.
type Plan struct {
	Width int
	Gap   int
	Rows  []Row
}

// Options configures layout planning.
type Options struct {
	Width int
	Gap   int
}

// Layout plans enabled boxes into wrapped rows.
func Layout(boxes []Box, opts Options) Plan {
	width := opts.Width
	gap := opts.Gap
	if gap < 0 {
		gap = 0
	}
	if width < 1 {
		width = 1
	}

	visible := make([]Box, 0, len(boxes))
	for i, box := range boxes {
		if !box.Enabled {
			continue
		}
		if box.Order == 0 {
			box.Order = i
		}
		visible = append(visible, normalizeBox(box))
	}
	sort.SliceStable(visible, func(i, j int) bool {
		if visible[i].Priority != visible[j].Priority {
			return visible[i].Priority < visible[j].Priority
		}
		return visible[i].Order < visible[j].Order
	})

	plan := Plan{Width: width, Gap: gap}
	var current []PlannedBox
	currentWidth := 0
	for _, box := range visible {
		boxWidth := usefulWidth(box, width)
		nextWidth := boxWidth
		if len(current) > 0 {
			nextWidth = currentWidth + gap + boxWidth
		}
		if len(current) > 0 && nextWidth > width {
			plan.Rows = append(plan.Rows, Row{Boxes: stretchRow(current, width, gap)})
			current = nil
			currentWidth = 0
		}
		if boxWidth > width {
			boxWidth = width
		}
		current = append(current, PlannedBox{Box: box, Width: boxWidth})
		if len(current) == 1 {
			currentWidth = boxWidth
		} else {
			currentWidth += gap + boxWidth
		}
	}
	if len(current) > 0 {
		plan.Rows = append(plan.Rows, Row{Boxes: stretchRow(current, width, gap)})
	}
	return plan
}

// Render renders a plan as ANSI-free ASCII bordered boxes.
func Render(plan Plan) string {
	var out []string
	for rowIndex, row := range plan.Rows {
		if rowIndex > 0 {
			out = append(out, "")
		}
		out = append(out, renderRow(row, plan.Gap)...)
	}
	return strings.Join(out, "\n")
}

func normalizeBox(box Box) Box {
	if box.MinWidth < 4 {
		box.MinWidth = 4
	}
	if box.PrefWidth < box.MinWidth {
		box.PrefWidth = box.MinWidth
	}
	if box.MaxWidth > 0 && box.MaxWidth < box.MinWidth {
		box.MaxWidth = box.MinWidth
	}
	if box.MaxWidth > 0 && box.PrefWidth > box.MaxWidth {
		box.PrefWidth = box.MaxWidth
	}
	return box
}

func usefulWidth(box Box, available int) int {
	width := box.PrefWidth
	if width < box.MinWidth {
		width = box.MinWidth
	}
	if box.MaxWidth > 0 && width > box.MaxWidth {
		width = box.MaxWidth
	}
	if width > available && box.MinWidth <= available {
		width = available
	}
	return width
}

func stretchRow(boxes []PlannedBox, available, gap int) []PlannedBox {
	result := append([]PlannedBox(nil), boxes...)
	if available <= 0 {
		return result
	}
	used := rowWidth(result, gap)
	extra := available - used
	if extra <= 0 {
		return result
	}

	for extra > 0 {
		changed := false
		for i := range result {
			if !result[i].Stretch {
				continue
			}
			if result[i].MaxWidth > 0 && result[i].Width >= result[i].MaxWidth {
				continue
			}
			result[i].Width++
			extra--
			changed = true
			if extra == 0 {
				break
			}
		}
		if !changed {
			break
		}
	}
	return result
}

func rowWidth(boxes []PlannedBox, gap int) int {
	width := 0
	for i, box := range boxes {
		if i > 0 {
			width += gap
		}
		width += box.Width
	}
	return width
}

func renderRow(row Row, gap int) []string {
	rendered := make([][]string, len(row.Boxes))
	maxHeight := 0
	for i, box := range row.Boxes {
		rendered[i] = renderBox(box)
		if len(rendered[i]) > maxHeight {
			maxHeight = len(rendered[i])
		}
	}

	gutter := strings.Repeat(" ", gap)
	lines := make([]string, 0, maxHeight)
	for lineIndex := 0; lineIndex < maxHeight; lineIndex++ {
		var line strings.Builder
		for boxIndex, boxLines := range rendered {
			if boxIndex > 0 {
				line.WriteString(gutter)
			}
			if lineIndex < len(boxLines) {
				line.WriteString(boxLines[lineIndex])
			} else {
				line.WriteString(strings.Repeat(" ", row.Boxes[boxIndex].Width))
			}
		}
		lines = append(lines, line.String())
	}
	return lines
}

func renderBox(box PlannedBox) []string {
	width := box.Width
	if width < 4 {
		width = 4
	}
	contentWidth := width - 4
	title := truncate(box.Title, width-4)
	topLabel := " " + title + " "
	topFill := width - 2 - runeLen(topLabel)
	if topFill < 0 {
		topFill = 0
	}

	lines := []string{"+" + topLabel + strings.Repeat("-", topFill) + "+"}
	for _, content := range box.Lines {
		content = truncate(content, contentWidth)
		lines = append(lines, "| "+content+strings.Repeat(" ", contentWidth-runeLen(content))+" |")
	}
	lines = append(lines, "+"+strings.Repeat("-", width-2)+"+")
	return lines
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runeLen(s) <= width {
		return s
	}
	if width <= 3 {
		return string([]rune(s)[:width])
	}
	runes := []rune(s)
	return string(runes[:width-3]) + "..."
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}
