//go:build ignore

// Command import-bdf-font imports one BDF bitmap into a glyphs.yaml file.
// It intentionally changes only the requested size; run it once per font size.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// bdfFont holds the glyphs plus the font bounding box that anchors the cell:
// ascent is the distance from the baseline to the cell top, xoff the cell's left edge.
type bdfFont struct {
	glyphs       []bdfGlyph
	ascent, xoff int
}

type bdfGlyph struct {
	code, width, height, xoff, yoff int
	rows                            []string
}

func main() {
	input := flag.String("input", "", "input BDF file")
	output := flag.String("output", "", "glyphs.yaml to update")
	size := flag.String("size", "", "target cell size, for example 7x13")
	cellWidth := flag.Int("cell-width", 0, "target matrix width; defaults to the width in -size")
	cellHeight := flag.Int("cell-height", 0, "target matrix height; defaults to the height in -size")
	flag.Parse()
	if *input == "" || *output == "" || *size == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := importBDFWithCellSize(*input, *output, *size, *cellWidth, *cellHeight); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func importBDF(inputPath, outputPath, size string) error {
	return importBDFWithCellSize(inputPath, outputPath, size, 0, 0)
}

func importBDFWithCellSize(inputPath, outputPath, size string, requestedWidth, requestedHeight int) error {
	cellWidth, cellHeight, err := parseSize(size)
	if err != nil {
		return err
	}
	if requestedWidth > 0 {
		cellWidth = requestedWidth
	}
	if requestedHeight > 0 {
		cellHeight = requestedHeight
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("read glyph spec: %w", err)
	}
	// Edit the YAML node tree so untouched glyphs, comments and ordering stay byte-stable.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse glyph spec: %w", err)
	}
	glyphs := mapValue(doc.Content[0], "glyphs")
	if glyphs == nil {
		return errors.New("glyph spec has no glyphs")
	}
	// Import only glyphs the spec already lists: real BDFs hold thousands of
	// code points, and the spec is parsed on every harnez start.
	wanted := make(map[rune]bool)
	if charset := mapValue(doc.Content[0], "charset"); charset != nil {
		for _, r := range charset.Value {
			wanted[r] = true
		}
	}
	for i := 0; i+1 < len(glyphs.Content); i += 2 {
		for _, r := range glyphs.Content[i].Value {
			wanted[r] = true
		}
	}
	font, err := parseBDF(inputPath, cellHeight)
	if err != nil {
		return err
	}
	for _, glyph := range font.glyphs {
		if !wanted[rune(glyph.code)] {
			continue
		}
		rows, err := matrix(glyph, font, cellWidth, cellHeight)
		if err != nil {
			return fmt.Errorf("glyph U+%04X: %w", glyph.code, err)
		}
		entry := mapEntry(glyphs, string(rune(glyph.code)))
		setSequence(entry, size, rows)
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encode glyph spec: %w", err)
	}
	return os.WriteFile(outputPath, out.Bytes(), 0o644)
}

// mapValue returns the value node stored under key in a mapping node, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// mapEntry returns the mapping stored under key, appending an empty one when missing.
func mapEntry(m *yaml.Node, key string) *yaml.Node {
	if v := mapValue(m, key); v != nil {
		return v
	}
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key, Style: yaml.DoubleQuotedStyle}
	m.Content = append(m.Content, k, v)
	return v
}

// setSequence replaces or appends the row sequence stored under size.
func setSequence(m *yaml.Node, size string, rows []string) {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, row := range rows {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: row, Style: yaml.DoubleQuotedStyle})
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == size {
			m.Content[i+1] = seq
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: size}, seq)
}

func parseSize(size string) (int, int, error) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid size %q", size)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid size %q", size)
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil || w < 1 || h < 1 {
		return 0, 0, fmt.Errorf("invalid size %q", size)
	}
	return w, h, nil
}

func parseBDF(path string, cellHeight int) (bdfFont, error) {
	file, err := os.Open(path)
	if err != nil {
		return bdfFont{}, fmt.Errorf("open BDF: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	font := bdfFont{ascent: cellHeight}
	var current *bdfGlyph
	readingBitmap := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "FONTBOUNDINGBOX "):
			fields := strings.Fields(line)
			if len(fields) != 5 {
				return bdfFont{}, fmt.Errorf("invalid FONTBOUNDINGBOX line %q", line)
			}
			height, _ := strconv.Atoi(fields[2])
			font.xoff, _ = strconv.Atoi(fields[3])
			yoff, _ := strconv.Atoi(fields[4])
			font.ascent = height + yoff
		case strings.HasPrefix(line, "STARTCHAR "):
			current = &bdfGlyph{code: -1}
		case strings.HasPrefix(line, "ENCODING ") && current != nil:
			if current.code, err = strconv.Atoi(strings.Fields(line)[1]); err != nil {
				return bdfFont{}, fmt.Errorf("invalid ENCODING line %q", line)
			}
		case strings.HasPrefix(line, "BBX ") && current != nil:
			fields := strings.Fields(line)
			if len(fields) != 5 {
				return bdfFont{}, fmt.Errorf("invalid BBX line %q", line)
			}
			current.width, _ = strconv.Atoi(fields[1])
			current.height, _ = strconv.Atoi(fields[2])
			current.xoff, _ = strconv.Atoi(fields[3])
			current.yoff, _ = strconv.Atoi(fields[4])
		case line == "BITMAP" && current != nil:
			readingBitmap = true
		case line == "ENDCHAR" && current != nil:
			if current.code >= 0 {
				font.glyphs = append(font.glyphs, *current)
			}
			current = nil
			readingBitmap = false
		case readingBitmap && current != nil:
			current.rows = append(current.rows, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return bdfFont{}, fmt.Errorf("read BDF: %w", err)
	}
	return font, nil
}

func matrix(glyph bdfGlyph, font bdfFont, width, height int) ([]string, error) {
	if glyph.width < 0 || glyph.width > 8 || glyph.height < 0 || len(glyph.rows) != glyph.height {
		return nil, errors.New("invalid BBX bitmap")
	}
	rows := make([][]byte, height)
	for y := range rows {
		rows[y] = []byte(strings.Repeat(" ", width))
	}
	top := font.ascent - (glyph.yoff + glyph.height)
	for y, encoded := range glyph.rows {
		bits, err := strconv.ParseUint(encoded, 16, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid bitmap row %q", encoded)
		}
		for x := 0; x < glyph.width; x++ {
			if bits&(1<<uint(8-1-x)) == 0 {
				continue
			}
			dx, dy := glyph.xoff-font.xoff+x, top+y
			if dx >= 0 && dx < width && dy >= 0 && dy < height {
				rows[dy][dx] = '1'
			}
		}
	}
	result := make([]string, len(rows))
	for i := range rows {
		result[i] = string(rows[i])
	}
	return result, nil
}
