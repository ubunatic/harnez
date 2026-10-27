package glossary

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// RuleID is the canonical lint rule identifier for glossary checks.
const RuleID = "glossary"

const suppressDirective = "harnez-lint:ignore"

// fileSuppressDirective skips the whole file when it appears on the first line.
const fileSuppressDirective = suppressDirective + "-file"

// Finding represents a single forbidden-term occurrence.
type Finding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Col         int    `json:"col"`
	Term        string `json:"term"`
	Phrase      string `json:"phrase"`
	Replacement string `json:"replacement"`
	Reason      string `json:"reason,omitempty"`
	RuleID      string `json:"rule_id"`
}

// FormatMessage returns the human-readable diagnostic message for the finding.
func (f Finding) FormatMessage() string {
	if f.Reason != "" {
		return fmt.Sprintf("forbidden term %q — use %s (%s)", f.Phrase, f.Replacement, f.Reason)
	}
	return fmt.Sprintf("forbidden term %q — use %s", f.Phrase, f.Replacement)
}

// FormatUnix formats the finding as "file:line:col: message [glossary]".
func (f Finding) FormatUnix() string {
	return fmt.Sprintf("%s:%d:%d: %s [%s]", f.File, f.Line, f.Col, f.FormatMessage(), f.RuleID)
}

type compiledCandidate struct {
	term        string
	replacement string
	reason      string
	scope       Scope
}

type compiledRule struct {
	phrase        string
	caseSensitive bool
	re            *regexp.Regexp
	candidates    []compiledCandidate
}

func compileRules(g *Glossary) []*compiledRule {
	if g == nil {
		return nil
	}
	type ruleKey struct {
		normPhrase    string
		caseSensitive bool
	}
	byKey := make(map[ruleKey]*compiledRule)
	var order []ruleKey

	for _, t := range g.Terms {
		if !t.IsEnabled() {
			continue
		}
		termScope := Scope{}
		if t.Scope != nil {
			termScope = *t.Scope
		}
		for _, fb := range t.Forbidden {
			norm := fb.Phrase
			if !fb.CaseSensitive {
				norm = strings.ToLower(fb.Phrase)
			}
			k := ruleKey{normPhrase: norm, caseSensitive: fb.CaseSensitive}
			cr, ok := byKey[k]
			if !ok {
				re := buildPhraseRegex(fb.Phrase, fb.CaseSensitive)
				if re == nil {
					continue
				}
				cr = &compiledRule{
					phrase:        fb.Phrase,
					caseSensitive: fb.CaseSensitive,
					re:            re,
				}
				byKey[k] = cr
				order = append(order, k)
			}
			cr.candidates = append(cr.candidates, compiledCandidate{
				term:        t.Term,
				replacement: fb.Replacement,
				reason:      fb.Reason,
				scope:       termScope,
			})
		}
	}

	out := make([]*compiledRule, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func buildPhraseRegex(phrase string, caseSensitive bool) *regexp.Regexp {
	if phrase == "" {
		return nil
	}
	escaped := regexp.QuoteMeta(phrase)
	prefix := ""
	if !caseSensitive {
		prefix = "(?i)"
	}
	leftBound := ""
	if isWordByte(phrase[0]) {
		leftBound = `\b`
	}
	rightBound := ""
	if isWordByte(phrase[len(phrase)-1]) {
		rightBound = `\b`
	}
	re, err := regexp.Compile(prefix + leftBound + escaped + rightBound)
	if err != nil {
		return nil
	}
	return re
}

// CheckFile checks a single file's content against the glossary.
// relPath should be relative to the repository root (or file path as supplied).
func (g *Glossary) CheckFile(relPath string, content []byte) []Finding {
	if g == nil || len(g.Terms) == 0 {
		return nil
	}
	normPath := NormalizeRelPath(relPath)
	if normPath == "" {
		return nil
	}
	if isClosedIssueTicket(normPath, content) {
		return nil
	}
	if hasFileLevelSuppression(content) {
		return nil
	}

	rules := compileRules(g)
	var active []*compiledRule
	for _, r := range rules {
		for _, c := range r.candidates {
			if c.scope.Matches(normPath, g.DefaultScope) {
				active = append(active, r)
				break
			}
		}
	}
	if len(active) == 0 {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(normPath))
	var findings []Finding
	switch ext {
	case ".go":
		findings = g.scanGo(relPath, normPath, content, active)
	case ".md", ".markdown":
		findings = g.scanMarkdown(relPath, normPath, content, active)
	default:
		findings = g.scanLines(relPath, normPath, content, active, false)
	}

	findings = filterLineSuppressed(findings, content)
	sortFindings(findings)
	return findings
}

// CheckDir walks rootDir and checks all matching files against the glossary.
func (g *Glossary) CheckDir(rootDir string) ([]Finding, error) {
	if g == nil {
		return nil, nil
	}
	var all []Finding
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		normRel := NormalizeRelPath(rel)
		if !g.anyRuleMatchesPath(normRel) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		all = append(all, g.CheckFile(normRel, content)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortFindings(all)
	return all, nil
}

func (g *Glossary) anyRuleMatchesPath(normRel string) bool {
	if g.DefaultScope.Matches(normRel, g.DefaultScope) {
		return true
	}
	for _, t := range g.Terms {
		if !t.IsEnabled() || t.Scope == nil {
			continue
		}
		if t.Scope.Matches(normRel, g.DefaultScope) {
			return true
		}
	}
	return false
}

var closedStatusRe = regexp.MustCompile(`(?m)^\*\*Status\*\*:\s*Closed\b`)

func isClosedIssueTicket(normPath string, content []byte) bool {
	if !MatchGlob("issues/*.md", normPath) && !MatchGlob("**/issues/*.md", normPath) {
		return false
	}
	return closedStatusRe.Match(content)
}

// hasFileLevelSuppression reports whether the first line carries a
// "harnez-lint:ignore-file glossary" directive, which skips the whole file.
func hasFileLevelSuppression(content []byte) bool {
	firstLine := content
	if idx := bytes.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}
	_, rest, ok := strings.Cut(string(firstLine), fileSuppressDirective)
	return ok && suppressRuleMatches(rest)
}

// lineSuppressesGlossary reports whether line carries a "harnez-lint:ignore glossary" directive.
// The "-file" form is excluded so it is never read as a line-level directive.
func lineSuppressesGlossary(line string) bool {
	_, rest, ok := strings.Cut(line, suppressDirective)
	if !ok || strings.HasPrefix(rest, "-file") {
		return false
	}
	return suppressRuleMatches(rest)
}

// suppressRuleMatches reports whether the rule list after a directive names RuleID.
func suppressRuleMatches(rest string) bool {
	rulesPart, _, _ := strings.Cut(rest, "--")
	rulesPart, _, _ = strings.Cut(rulesPart, "-->")
	for _, r := range strings.FieldsFunc(rulesPart, func(c rune) bool {
		return c == ',' || c == ' ' || c == '\t'
	}) {
		if r == RuleID {
			return true
		}
	}
	return false
}

func filterLineSuppressed(findings []Finding, content []byte) []Finding {
	if len(findings) == 0 || !bytes.Contains(content, []byte(suppressDirective)) {
		return findings
	}
	lines := strings.Split(string(content), "\n")
	kept := findings[:0]
	for _, f := range findings {
		suppressed := false
		if f.Line >= 1 && f.Line <= len(lines) && lineSuppressesGlossary(lines[f.Line-1]) {
			suppressed = true
		}
		// A directive on the line above applies only when it is a standalone comment,
		// so an inline directive suppresses just its own line.
		if f.Line >= 2 && f.Line-1 <= len(lines) && isStandaloneComment(lines[f.Line-2]) && lineSuppressesGlossary(lines[f.Line-2]) {
			suppressed = true
		}
		if !suppressed {
			kept = append(kept, f)
		}
	}
	return kept
}

// isStandaloneComment reports whether line starts with a comment opener, so a directive
// in it is the whole line's content rather than a trailing note on code.
func isStandaloneComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "#")
}

func (g *Glossary) scanMarkdown(displayPath, normPath string, content []byte, rules []*compiledRule) []Finding {
	return g.scanLines(displayPath, normPath, content, rules, true)
}

func (g *Glossary) scanLines(displayPath, normPath string, content []byte, rules []*compiledRule, trackMarkdownFences bool) []Finding {
	lines := strings.Split(string(content), "\n")
	var findings []Finding
	inFence := false
	var fenceMarker string

	for i, line := range lines {
		lineNum := i + 1
		if trackMarkdownFences {
			trimmed := strings.TrimSpace(line)
			if !inFence {
				if m := leadingFenceMarker(trimmed); m != "" {
					inFence = true
					fenceMarker = m
					continue
				}
			} else {
				if isClosingFence(trimmed, fenceMarker) {
					inFence = false
					fenceMarker = ""
				}
				continue
			}
		}
		findings = append(findings, g.matchLineText(displayPath, normPath, lineNum, 1, line, rules)...)
	}
	return findings
}

func leadingFenceMarker(trimmed string) string {
	if strings.HasPrefix(trimmed, "```") {
		n := 0
		for n < len(trimmed) && trimmed[n] == '`' {
			n++
		}
		return trimmed[:n]
	}
	if strings.HasPrefix(trimmed, "~~~") {
		n := 0
		for n < len(trimmed) && trimmed[n] == '~' {
			n++
		}
		return trimmed[:n]
	}
	return ""
}

func isClosingFence(trimmed, openMarker string) bool {
	if openMarker == "" || !strings.HasPrefix(trimmed, openMarker) {
		return false
	}
	ch := openMarker[0]
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != ch && trimmed[i] != ' ' && trimmed[i] != '\t' && trimmed[i] != '\r' {
			return false
		}
	}
	return true
}

func (g *Glossary) scanGo(displayPath, normPath string, content []byte, rules []*compiledRule) []Finding {
	fset := token.NewFileSet()
	file := fset.AddFile(normPath, fset.Base(), len(content))
	var s scanner.Scanner
	s.Init(file, content, nil, scanner.ScanComments)

	var findings []Finding
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.COMMENT && tok != token.STRING {
			continue
		}
		basePos := fset.Position(pos)
		subLines := strings.Split(lit, "\n")
		for idx, sub := range subLines {
			lineNum := basePos.Line + idx
			startCol := 1
			if idx == 0 {
				startCol = basePos.Column
			}
			findings = append(findings, g.matchLineText(displayPath, normPath, lineNum, startCol, sub, rules)...)
		}
	}
	return findings
}

func (g *Glossary) matchLineText(displayPath, normPath string, lineNum, startCol int, text string, rules []*compiledRule) []Finding {
	var findings []Finding
	for _, r := range rules {
		var matchedCandidates []compiledCandidate
		for _, c := range r.candidates {
			if c.scope.Matches(normPath, g.DefaultScope) {
				matchedCandidates = append(matchedCandidates, c)
			}
		}
		if len(matchedCandidates) == 0 {
			continue
		}

		termName, replacement, reason := combineCandidates(matchedCandidates)
		locs := r.re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			matchedText := text[loc[0]:loc[1]]
			col := startCol + loc[0]
			findings = append(findings, Finding{
				File:        displayPath,
				Line:        lineNum,
				Col:         col,
				Term:        termName,
				Phrase:      matchedText,
				Replacement: replacement,
				Reason:      reason,
				RuleID:      RuleID,
			})
		}
	}
	return findings
}

func combineCandidates(candidates []compiledCandidate) (termName, replacement, reason string) {
	var terms []string
	seenTerms := make(map[string]bool, len(candidates))
	var replacements []string
	seenRepl := make(map[string]bool, len(candidates))

	for _, c := range candidates {
		if !seenTerms[c.term] {
			seenTerms[c.term] = true
			terms = append(terms, c.term)
		}
		if !seenRepl[c.replacement] {
			seenRepl[c.replacement] = true
			replacements = append(replacements, fmt.Sprintf("%q", c.replacement))
		}
		if reason == "" && c.reason != "" {
			reason = c.reason
		}
	}
	return strings.Join(terms, " / "), strings.Join(replacements, " or "), reason
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		if findings[i].Col != findings[j].Col {
			return findings[i].Col < findings[j].Col
		}
		return findings[i].Phrase < findings[j].Phrase
	})
}
