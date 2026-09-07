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
	// Markdown HTML comment markers
	reMDBegin = regexp.MustCompile(`<!--\s*harnez:begin\s*(.*?)\s*-->`)
	reMDEnd   = regexp.MustCompile(`<!--\s*harnez:end\s*(.*?)\s*-->`)

	// Makefile comment markers
	reMKBegin = regexp.MustCompile(`^#\s*harnez:begin\s*(.*?)\s*$`)
	reMKEnd   = regexp.MustCompile(`^#\s*harnez:end\s*(.*?)\s*$`)
)

func (r *MarkerIntegrityRule) Check(file string, content []byte) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	var stack []openMarker

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Match Markdown begin
		if loc := reMDBegin.FindStringSubmatchIndex(line); loc != nil {
			name := strings.TrimSpace(line[loc[2]:loc[3]])
			col := loc[0] + 1
			if name == "" {
				findings = append(findings, Finding{
					File:     file,
					Line:     lineNum,
					Col:      col,
					RuleID:   "marker-invalid",
					Severity: SeverityError,
					Message:  "empty section name in harnez:begin marker",
				})
			}
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				findings = append(findings, Finding{
					File:     file,
					Line:     lineNum,
					Col:      col,
					RuleID:   "marker-nested",
					Severity: SeverityError,
					Message:  fmt.Sprintf("nested marker '%s' inside unclosed section '%s' from line %d", name, top.name, top.line),
				})
			}
			stack = append(stack, openMarker{name: name, line: lineNum, col: col})
			continue
		}

		// Match Markdown end
		if loc := reMDEnd.FindStringSubmatchIndex(line); loc != nil {
			name := strings.TrimSpace(line[loc[2]:loc[3]])
			col := loc[0] + 1
			if len(stack) == 0 {
				findings = append(findings, Finding{
					File:     file,
					Line:     lineNum,
					Col:      col,
					RuleID:   "marker-unopened",
					Severity: SeverityError,
					Message:  fmt.Sprintf("found harnez:end '%s' with no preceding harnez:begin", name),
				})
			} else {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if top.name != name {
					findings = append(findings, Finding{
						File:     file,
						Line:     lineNum,
						Col:      col,
						RuleID:   "marker-mismatch",
						Severity: SeverityError,
						Message:  fmt.Sprintf("mismatched marker: harnez:begin '%s' (line %d) closed by harnez:end '%s'", top.name, top.line, name),
					})
				}
			}
			continue
		}

		// Match Makefile begin
		if loc := reMKBegin.FindStringSubmatchIndex(line); loc != nil {
			name := strings.TrimSpace(line[loc[2]:loc[3]])
			col := loc[0] + 1
			if name == "" {
				findings = append(findings, Finding{
					File:     file,
					Line:     lineNum,
					Col:      col,
					RuleID:   "marker-invalid",
					Severity: SeverityError,
					Message:  "empty section name in harnez:begin marker",
				})
			}
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				findings = append(findings, Finding{
					File:     file,
					Line:     lineNum,
					Col:      col,
					RuleID:   "marker-nested",
					Severity: SeverityError,
					Message:  fmt.Sprintf("nested marker '%s' inside unclosed section '%s' from line %d", name, top.name, top.line),
				})
			}
			stack = append(stack, openMarker{name: name, line: lineNum, col: col})
			continue
		}

		// Match Makefile end
		if loc := reMKEnd.FindStringSubmatchIndex(line); loc != nil {
			name := strings.TrimSpace(line[loc[2]:loc[3]])
			col := loc[0] + 1
			if len(stack) == 0 {
				findings = append(findings, Finding{
					File:     file,
					Line:     lineNum,
					Col:      col,
					RuleID:   "marker-unopened",
					Severity: SeverityError,
					Message:  fmt.Sprintf("found harnez:end '%s' with no preceding harnez:begin", name),
				})
			} else {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if top.name != name {
					findings = append(findings, Finding{
						File:     file,
						Line:     lineNum,
						Col:      col,
						RuleID:   "marker-mismatch",
						Severity: SeverityError,
						Message:  fmt.Sprintf("mismatched marker: harnez:begin '%s' (line %d) closed by harnez:end '%s'", top.name, top.line, name),
					})
				}
			}
			continue
		}
	}

	// Any unclosed markers left on stack at EOF
	for _, unclosed := range stack {
		findings = append(findings, Finding{
			File:     file,
			Line:     unclosed.line,
			Col:      unclosed.col,
			RuleID:   "marker-unclosed",
			Severity: SeverityError,
			Message:  fmt.Sprintf("unclosed marker: harnez:begin '%s' opened at line %d was never closed", unclosed.name, unclosed.line),
		})
	}

	return findings
}
