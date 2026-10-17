package lint

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// MarkerIntegrityRule verifies integrity of harnez section markers in Markdown and Makefiles.
type MarkerIntegrityRule struct{}

func NewMarkerIntegrityRule() *MarkerIntegrityRule {
	return &MarkerIntegrityRule{}
}

func (r *MarkerIntegrityRule) ID() string {
	return "marker-integrity"
}

func (r *MarkerIntegrityRule) Description() string {
	return "Verify integrity of harnez:begin and harnez:end section markers"
}

func (r *MarkerIntegrityRule) Languages() []Language {
	return []Language{LangMarkdown, LangMake}
}

type openMarker struct {
	name string
	line int
	col  int
}

var (
	// Markdown HTML comment markers. Only a marker that starts its line counts:
	// prose and inline code that mention a marker are not markers.
	reMDBegin = regexp.MustCompile(`^\s*<!--\s*harnez:begin\s*(.*?)\s*-->`)
	reMDEnd   = regexp.MustCompile(`^\s*<!--\s*harnez:end\s*(.*?)\s*-->`)

	// Makefile comment markers
	reMKBegin = regexp.MustCompile(`^#\s*harnez:begin\s*(.*?)\s*$`)
	reMKEnd   = regexp.MustCompile(`^#\s*harnez:end\s*(.*?)\s*$`)
)

// matchMarker reports whether line holds a begin or end marker, and its name and column.
func matchMarker(line string) (begin bool, name string, col int, ok bool) {
	for _, m := range []struct {
		re    *regexp.Regexp
		begin bool
	}{{reMDBegin, true}, {reMDEnd, false}, {reMKBegin, true}, {reMKEnd, false}} {
		if loc := m.re.FindStringSubmatchIndex(line); loc != nil {
			col := len(line) - len(strings.TrimLeft(line, " \t")) + 1
			return m.begin, strings.TrimSpace(line[loc[2]:loc[3]]), col, true
		}
	}
	return false, "", 0, false
}

// isFence reports whether line opens or closes a fenced Markdown code block.
func isFence(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

func (r *MarkerIntegrityRule) Check(file string, content []byte) []Finding {
	var findings []Finding
	add := func(line, col int, rule, msg string) {
		findings = append(findings, Finding{File: file, Line: line, Col: col, RuleID: rule, Severity: SeverityError, Message: msg})
	}
	markdown := DetectLanguage(file, content) != LangMake
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	inFence := false
	var stack []openMarker

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if markdown && isFence(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		begin, name, col, ok := matchMarker(line)
		switch {
		case !ok:
		case begin:
			if name == "" {
				add(lineNum, col, "marker-invalid", "empty section name in harnez:begin marker")
			}
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				add(lineNum, col, "marker-nested", fmt.Sprintf("nested marker '%s' inside unclosed section '%s' from line %d", name, top.name, top.line))
			}
			stack = append(stack, openMarker{name: name, line: lineNum, col: col})
		case len(stack) == 0:
			add(lineNum, col, "marker-unopened", fmt.Sprintf("found harnez:end '%s' with no preceding harnez:begin", name))
		default:
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.name != name {
				add(lineNum, col, "marker-mismatch", fmt.Sprintf("mismatched marker: harnez:begin '%s' (line %d) closed by harnez:end '%s'", top.name, top.line, name))
			}
		}
	}

	// Any unclosed markers left on stack at EOF
	for _, unclosed := range stack {
		add(unclosed.line, unclosed.col, "marker-unclosed", fmt.Sprintf("unclosed marker: harnez:begin '%s' opened at line %d was never closed", unclosed.name, unclosed.line))
	}

	return findings
}
