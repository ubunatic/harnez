package readcard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// TextOptions configures text-mode file reading.
type TextOptions struct {
	ShowLineNumbers bool   // -n, --number
	LineRange       string // -L, --lines (e.g. 10:50, 10-50, 10..50, :50, 100:)
	Head            int    // --head N
	Tail            int    // --tail N
	ShowStats       bool   // --stats, --tokens
}

// ReadResult holds read text content, sliced lines, and line metadata.
type ReadResult struct {
	SourceFile  string     `json:"source_file"`
	Lines       []string   `json:"lines"`
	StartLine   int        `json:"start_line"`
	TotalLines  int        `json:"total_lines"`
	TokenStats  TokenStats `json:"token_stats"`
	SourceLines []int      `json:"source_lines,omitempty"`
}

// ReadSource reads lines from a file path or reader, applying line-range and head/tail filtering.
func ReadSource(r io.Reader, filename string, opts TextOptions) (*ReadResult, error) {
	scanner := bufio.NewScanner(r)
	// Buffer size up to 10MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}

	totalOriginal := len(allLines)
	start := 1
	end := totalOriginal

	// Parse line range if given
	if opts.LineRange != "" {
		s, e, err := ParseLineRange(opts.LineRange, totalOriginal)
		if err != nil {
			return nil, fmt.Errorf("invalid line range %q: %w", opts.LineRange, err)
		}
		start = s
		end = e
	}

	// Apply start/end slice
	var selected []string
	if totalOriginal > 0 && start <= end && start <= totalOriginal {
		sIdx := start - 1
		if sIdx < 0 {
			sIdx = 0
		}
		eIdx := end
		if eIdx > totalOriginal {
			eIdx = totalOriginal
		}
		selected = allLines[sIdx:eIdx]
	}

	// Apply head/tail if set
	if opts.Head > 0 && len(selected) > opts.Head {
		selected = selected[:opts.Head]
	}
	if opts.Tail > 0 && len(selected) > opts.Tail {
		offset := len(selected) - opts.Tail
		start += offset
		selected = selected[offset:]
	}

	fullText := strings.Join(selected, "\n")
	stats := ComputeTextTokens(fullText)

	return &ReadResult{
		SourceFile: filename,
		Lines:      selected,
		StartLine:  start,
		TotalLines: totalOriginal,
		TokenStats: stats,
	}, nil
}

// ReadFile reads lines from a file path with text options.
func ReadFile(path string, opts TextOptions) (*ReadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadSource(f, path, opts)
}

// FormatText formats lines for stdout, optionally prefixing line numbers.
func FormatText(res *ReadResult, showLineNumbers bool) string {
	mode := "off"
	if showLineNumbers {
		mode = "all"
	}
	text, _ := FormatTextCadence(res, mode)
	return text
}

// FormatTextCadence formats original source anchors with an optional gutter cadence.
func FormatTextCadence(res *ReadResult, mode string) (string, error) {
	cadence, err := ParseLineNumbers(mode)
	if err != nil {
		return "", err
	}
	if cadence == 0 {
		return strings.Join(res.Lines, "\n"), nil
	}

	var sb strings.Builder
	maxLineNum := res.StartLine + len(res.Lines) - 1
	if len(res.SourceLines) > 0 {
		maxLineNum = res.SourceLines[len(res.SourceLines)-1]
	}
	digits := len(fmt.Sprintf("%d", maxLineNum))
	if digits < 3 {
		digits = 3
	}

	for i, l := range res.Lines {
		lineNum := res.StartLine + i
		if len(res.SourceLines) > i {
			lineNum = res.SourceLines[i]
		}
		label := fmt.Sprintf("%d", lineNum)
		if i != 0 && lineNum%cadence != 0 {
			label = "."
		}
		sb.WriteString(fmt.Sprintf("%*s │ %s\n", digits, label, l))
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

// ParseLineRange parses range strings like "10:50", "10-50", "10..50", ":50", "100:", "15".
func ParseLineRange(s string, totalLines int) (int, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 1, totalLines, nil
	}

	sep := ""
	for _, candidate := range []string{":", "..", "-"} {
		if strings.Contains(s, candidate) {
			sep = candidate
			break
		}
	}

	if sep == "" {
		// Single line e.g. "42"
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid line number: %w", err)
		}
		if n <= 0 {
			n = 1
		}
		return n, n, nil
	}

	parts := strings.SplitN(s, sep, 2)
	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	start := 1
	if startStr != "" {
		n, err := strconv.Atoi(startStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start line %q: %w", startStr, err)
		}
		start = n
	}

	end := totalLines
	if endStr != "" {
		n, err := strconv.Atoi(endStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end line %q: %w", endStr, err)
		}
		end = n
	}

	if start < 1 {
		start = 1
	}
	if end < start {
		end = start
	}

	return start, end, nil
}
