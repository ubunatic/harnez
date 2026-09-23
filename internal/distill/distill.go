// Package distill filters low-signal noise (passing test spam, ANSI codes,
// repeated warnings, oversized output) out of routine command output so it
// costs fewer tokens in an agent's context window.
package distill

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
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
	Mode       Mode
	MaxLines   int // 0 disables line-based head/tail truncation
	MaxBytes   int // 0 disables the byte-based hard cap (see FilterHeadTailBytes)
	NoDedup    bool
	SimplePath bool // When true, forces simple slice-based processing rather than fast zero-alloc paths
}

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// StripANSI removes terminal color and cursor-movement escape sequences.
func StripANSI(s string) string {
	// Fast path: avoid expensive regex evaluation if no escape sequences exist.
	if !strings.Contains(s, "\x1b[") {
		return s
	}
	return ansiRE.ReplaceAllString(s, "")
}

var (
	prefixRun     = []byte("=== RUN")
	prefixPass    = []byte("--- PASS")
	prefixSkip    = []byte("--- SKIP")
	prefixFail    = []byte("--- FAIL")
	prefixOk      = []byte("ok ")
	prefixPkgFail = []byte("FAIL")
	prefixPkgPass = []byte("PASS")
)

// FilterGoTest drops "=== RUN" / "--- PASS" noise from `go test -v` output,
// keeping failure blocks, panics, build errors, and package summary lines.
func FilterGoTest(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, 10*1024*1024)

	var out strings.Builder
	var pendingBuf []byte
	first := true

	writeLine := func(b []byte) {
		if !first {
			out.WriteByte('\n')
		}
		out.Write(b)
		first = false
	}

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		trimmedBytes := bytes.TrimSpace(lineBytes)

		switch {
		case bytes.HasPrefix(trimmedBytes, prefixRun):
			pendingBuf = pendingBuf[:0]
			pendingBuf = append(pendingBuf, lineBytes...)
		case bytes.HasPrefix(trimmedBytes, prefixPass) || bytes.HasPrefix(trimmedBytes, prefixSkip):
			pendingBuf = pendingBuf[:0]
		case bytes.HasPrefix(trimmedBytes, prefixFail):
			if len(pendingBuf) > 0 {
				writeLine(pendingBuf)
				pendingBuf = pendingBuf[:0]
			}
			writeLine(lineBytes)
		case bytes.HasPrefix(lineBytes, prefixOk) || bytes.HasPrefix(lineBytes, prefixPkgFail) || bytes.HasPrefix(lineBytes, prefixPkgPass):
			if len(pendingBuf) > 0 {
				writeLine(pendingBuf)
				pendingBuf = pendingBuf[:0]
			}
			writeLine(lineBytes)
		default:
			if len(pendingBuf) > 0 {
				pendingBuf = append(pendingBuf, '\n')
				pendingBuf = append(pendingBuf, lineBytes...)
			} else {
				writeLine(lineBytes)
			}
		}
	}
	if len(pendingBuf) > 0 {
		writeLine(pendingBuf)
	}
	return out.String()
}

// FilterGit condenses `git status` output, collapsing long untracked-file
// listings into a single count line.
func FilterGit(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, 10*1024*1024)

	var out strings.Builder
	untrackedCount := 0
	inUntracked := false
	first := true

	writeLine := func(b []byte) {
		if !first {
			out.WriteByte('\n')
		}
		out.Write(b)
		first = false
	}

	flushUntracked := func() {
		if untrackedCount > 0 {
			if !first {
				out.WriteByte('\n')
			}
			first = false
			out.WriteString("  [")
			var buf [20]byte
			out.Write(strconv.AppendInt(buf[:0], int64(untrackedCount), 10))
			out.WriteString(" untracked files omitted]")
			untrackedCount = 0
		}
	}

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		trimmedBytes := bytes.TrimSpace(lineBytes)

		if bytes.HasPrefix(trimmedBytes, []byte("Untracked files:")) {
			inUntracked = true
			writeLine(lineBytes)
			continue
		}
		if inUntracked {
			switch {
			case len(trimmedBytes) == 0:
				flushUntracked()
				inUntracked = false
				writeLine(lineBytes)
			case bytes.HasPrefix(trimmedBytes, []byte("(use ")):
				writeLine(lineBytes)
			default:
				untrackedCount++
			}
			continue
		}
		writeLine(lineBytes)
	}
	flushUntracked()
	return out.String()
}

// FilterDeduplicate collapses runs of identical consecutive lines into a
// single "[xN] <line>" entry.
func FilterDeduplicate(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, 10*1024*1024)

	var out strings.Builder
	var prev []byte
	count := 0
	seen := false
	first := true

	writeLine := func(b []byte) {
		if !first {
			out.WriteByte('\n')
		}
		out.Write(b)
		first = false
	}

	flush := func() {
		if count == 0 {
			return
		}
		if count == 1 {
			writeLine(prev)
		} else {
			if !first {
				out.WriteByte('\n')
			}
			first = false
			out.WriteString("[x")
			var buf [20]byte
			out.Write(strconv.AppendInt(buf[:0], int64(count), 10))
			out.WriteString("] ")
			out.Write(prev)
		}
	}

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if seen && bytes.Equal(lineBytes, prev) {
			count++
			continue
		}
		flush()
		prev = append(prev[:0], lineBytes...)
		count = 1
		seen = true
	}
	flush()
	return out.String()
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

	var out strings.Builder
	for i := 0; i < head; i++ {
		out.WriteString(lines[i])
		out.WriteByte('\n')
	}
	out.WriteString("[... ")
	var buf [20]byte
	out.Write(strconv.AppendInt(buf[:0], int64(omitted), 10))
	out.WriteString(" lines omitted ...]\n")
	for i := len(lines) - tail; i < len(lines); i++ {
		out.WriteString(lines[i])
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// FilterHeadTailString truncates lines in string s beyond maxLines without
// splitting into a []string slice.
func FilterHeadTailString(s string, maxLines int) string {
	if maxLines <= 0 {
		return s
	}

	numLines := strings.Count(s, "\n") + 1
	if numLines <= maxLines {
		return s
	}

	head := maxLines / 2
	tail := maxLines - head
	omitted := numLines - head - tail

	headEnd := 0
	for i := 0; i < head; i++ {
		idx := strings.IndexByte(s[headEnd:], '\n')
		if idx < 0 {
			break
		}
		headEnd += idx + 1
	}

	skipLines := numLines - tail
	tailStart := 0
	for i := 0; i < skipLines; i++ {
		idx := strings.IndexByte(s[tailStart:], '\n')
		if idx < 0 {
			break
		}
		tailStart += idx + 1
	}

	var out strings.Builder
	out.Grow(headEnd + (len(s) - tailStart) + 40)
	out.WriteString(s[:headEnd])
	out.WriteString("[... ")
	var buf [20]byte
	out.Write(strconv.AppendInt(buf[:0], int64(omitted), 10))
	out.WriteString(" lines omitted ...]\n")
	out.WriteString(s[tailStart:])
	return out.String()
}

// truncationSentinel prefixes every byte-cap truncation note. It is a fixed,
// grep-able string so an agent (or a downstream tool) can reliably detect
// "this output was truncated, don't treat it as complete" without having to
// parse free-text, and so it's never mistaken for meaningful command output.
const truncationSentinel = "[harnez-distill:truncated"

// FilterHeadTailBytes enforces a hard byte cap on s, keeping whole lines from
// the head and tail and eliding the middle — the same head+tail shape as
// FilterHeadTail, but bounded by bytes rather than line count so a handful of
// pathologically long lines (a giant JSON dump, a single huge stack trace)
// can't blow past a byte budget that a line-count cap alone wouldn't catch.
//
// Placement rationale (see issue 182): build/test logs put the actionable
// signal at both ends — the invoked command and early setup/compile errors
// at the head, the final failure/summary at the tail — so both ends are kept
// and only the noisy middle is elided, matching the "middle_lines" strategy
// used by comparable tools (e.g. OpenAI Codex's head+tail tool-output cap)
// rather than a plain head-only or tail-only cut.
//
// The note is a fixed sentinel (truncationSentinel) followed by exact
// omitted/total/cap byte counts, so it's machine-parseable rather than
// free-text an agent might mistake for real output or grounds to retry the
// command expecting a different, untruncated result.
func FilterHeadTailBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}

	headBytes := maxBytes / 2
	tailBytes := maxBytes - headBytes

	// Round the head cut back to the preceding newline so we never split a
	// line in half.
	headEnd := headBytes
	if headEnd > len(s) {
		headEnd = len(s)
	}
	if idx := strings.LastIndexByte(s[:headEnd], '\n'); idx >= 0 {
		headEnd = idx + 1
	} else {
		headEnd = 0
	}

	// Round the tail cut forward to the following newline for the same
	// reason.
	tailStart := len(s) - tailBytes
	if tailStart < 0 {
		tailStart = 0
	}
	if idx := strings.IndexByte(s[tailStart:], '\n'); idx >= 0 {
		tailStart += idx + 1
	}

	if tailStart <= headEnd {
		// Not enough room for a clean split (e.g. one gigantic line with no
		// newlines nearby); fall back to a hard byte cut at the midpoint.
		headEnd = headBytes
		tailStart = headEnd
	}

	omitted := tailStart - headEnd
	note := fmt.Sprintf("\n%s %d bytes omitted, %d of %d total bytes kept, %d-byte cap]\n",
		truncationSentinel, omitted, len(s)-omitted, len(s), maxBytes)
	return s[:headEnd] + note + s[tailStart:]
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
		if opts.SimplePath {
			s = FilterHeadTail(strings.Split(s, "\n"), opts.MaxLines)
		} else {
			s = FilterHeadTailString(s, opts.MaxLines)
		}
	}
	if opts.MaxBytes > 0 {
		s = FilterHeadTailBytes(s, opts.MaxBytes)
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
