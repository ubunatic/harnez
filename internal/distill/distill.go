// Package distill filters low-signal noise (passing test spam, ANSI codes,
// repeated warnings, oversized output) out of routine command output so it
// costs fewer tokens in an agent's context window.
package distill

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Mode selects which structured filter to apply before generic dedup/truncation.
type Mode string

const (
	ModeAuto   Mode = "auto"
	ModeGoTest Mode = "gotest"
	ModeGit    Mode = "git"
	ModeRaw    Mode = "raw"
)

// Options controls the distillation pipeline.
type Options struct {
	Mode     Mode
	MaxLines int // 0 disables head/tail truncation
	NoDedup  bool
}

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// StripANSI removes terminal color and cursor-movement escape sequences.
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// FilterGoTest drops "=== RUN" / "--- PASS" noise from `go test -v` output,
// keeping failure blocks, panics, build errors, and package summary lines.
func FilterGoTest(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var out []string
	var pending []string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "=== RUN"):
			pending = []string{line}
		case strings.HasPrefix(trimmed, "--- PASS"), strings.HasPrefix(trimmed, "--- SKIP"):
			pending = nil
		case strings.HasPrefix(trimmed, "--- FAIL"):
			pending = append(pending, line)
			out = append(out, pending...)
			pending = nil
		case strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "FAIL") || strings.HasPrefix(line, "PASS"):
			out = append(out, line)
		default:
			if pending != nil {
				pending = append(pending, line)
			} else {
				out = append(out, line)
			}
		}
	}
	out = append(out, pending...)
	return strings.Join(out, "\n")
}

// FilterGit condenses `git status` output, collapsing long untracked-file
// listings into a single count line.
func FilterGit(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var out []string
	var untracked []string
	inUntracked := false

	flushUntracked := func() {
		if len(untracked) > 0 {
			out = append(out, fmt.Sprintf("  [%d untracked files omitted]", len(untracked)))
			untracked = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Untracked files:") {
			inUntracked = true
			out = append(out, line)
			continue
		}
		if inUntracked {
			switch {
			case trimmed == "":
				flushUntracked()
				inUntracked = false
				out = append(out, line)
			case strings.HasPrefix(trimmed, "(use "):
				out = append(out, line)
			default:
				untracked = append(untracked, line)
			}
			continue
		}
		out = append(out, line)
	}
	flushUntracked()
	return strings.Join(out, "\n")
}

// FilterDeduplicate collapses runs of identical consecutive lines into a
// single "[xN] <line>" entry.
func FilterDeduplicate(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var out []string
	var prev string
	count := 0
	seen := false

	flush := func() {
		if count == 0 {
			return
		}
		if count == 1 {
			out = append(out, prev)
		} else {
			out = append(out, fmt.Sprintf("[x%d] %s", count, prev))
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if seen && line == prev {
			count++
			continue
		}
		flush()
		prev = line
		count = 1
		seen = true
	}
	flush()
	return strings.Join(out, "\n")
}

// FilterHeadTail truncates lines beyond maxLines, keeping the first and last
// halves and noting how many lines were omitted in between.
func FilterHeadTail(lines []string, maxLines int) string {
	if maxLines <= 0 || len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	head := maxLines / 2
	tail := maxLines - head
	omitted := len(lines) - head - tail

	out := make([]string, 0, maxLines+1)
	out = append(out, lines[:head]...)
	out = append(out, fmt.Sprintf("[... %d lines omitted ...]", omitted))
	out = append(out, lines[len(lines)-tail:]...)
	return strings.Join(out, "\n")
}

// DetectMode guesses the structured filter to apply from output content.
func DetectMode(s string) Mode {
	switch {
	case strings.Contains(s, "=== RUN") || strings.Contains(s, "--- FAIL") || strings.Contains(s, "--- PASS"):
		return ModeGoTest
	case strings.Contains(s, "On branch ") || strings.Contains(s, "Untracked files:") || strings.Contains(s, "Changes not staged"):
		return ModeGit
	default:
		return ModeRaw
	}
}

// Distill runs the full pipeline: ANSI stripping, mode-specific structured
// filtering, deduplication, and head/tail truncation.
func Distill(input string, opts Options) string {
	s := StripANSI(input)

	mode := opts.Mode
	if mode == "" || mode == ModeAuto {
		mode = DetectMode(s)
	}
	switch mode {
	case ModeGoTest:
		s = FilterGoTest(strings.NewReader(s))
	case ModeGit:
		s = FilterGit(strings.NewReader(s))
	}

	if !opts.NoDedup {
		s = FilterDeduplicate(strings.NewReader(s))
	}
	if opts.MaxLines > 0 {
		s = FilterHeadTail(strings.Split(s, "\n"), opts.MaxLines)
	}
	return s
}

// DistillWithMetrics runs the full pipeline and returns the distilled output
// alongside raw and distilled byte counts.
func DistillWithMetrics(input string, opts Options) (string, int64, int64) {
	rawBytes := int64(len(input))
	distilled := Distill(input, opts)
	distilledBytes := int64(len(distilled))
	return distilled, rawBytes, distilledBytes
}

// DetectModeFromArgs guesses the structured filter from a wrapped command's
// argv, e.g. []string{"go", "test", "./..."} -> ModeGoTest.
func DetectModeFromArgs(args []string) Mode {
	if len(args) == 0 {
		return ModeRaw
	}
	switch {
	case args[0] == "go" && len(args) > 1 && args[1] == "test":
		return ModeGoTest
	case args[0] == "git":
		return ModeGit
	default:
		return ModeRaw
	}
}
