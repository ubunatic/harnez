package lint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Language represents a recognized target language or file format.
type Language string

const (
	LangAuto     Language = "auto"
	LangBash     Language = "bash"
	LangGo       Language = "go"
	LangMake     Language = "make"
	LangMarkdown Language = "markdown"
	LangUnknown  Language = "unknown"
)

// ParseLanguage parses a user-supplied language string or alias.
func ParseLanguage(s string) (Language, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	switch norm {
	case "", "auto":
		return LangAuto, nil
	case "bash", "sh", "shell":
		return LangBash, nil
	case "go", "golang":
		return LangGo, nil
	case "make", "makefile", "mk":
		return LangMake, nil
	case "md", "markdown":
		return LangMarkdown, nil
	default:
		return LangUnknown, fmt.Errorf("unknown language %q (expected auto, bash, go, make, markdown)", s)
	}
}

// DetectLanguage infers the Language from a file path and/or content shebang.
func DetectLanguage(path string, content []byte) Language {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".sh", ".bash":
		return LangBash
	case ".go":
		return LangGo
	case ".mk":
		return LangMake
	case ".md", ".markdown":
		return LangMarkdown
	}

	if base == "makefile" || base == "gnumakefile" || strings.HasPrefix(base, "makefile.") {
		return LangMake
	}

	// Check shebang
	if len(content) > 2 && content[0] == '#' && content[1] == '!' {
		firstLine := content
		if idx := bytes.IndexByte(content, '\n'); idx >= 0 {
			firstLine = content[:idx]
		}
		lineStr := strings.ToLower(string(firstLine))
		if strings.Contains(lineStr, "bash") || strings.Contains(lineStr, "/sh") || strings.HasSuffix(lineStr, " sh") {
			return LangBash
		}
	}

	return LangUnknown
}

// Severity represents the finding's impact level.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Finding describes a single rule violation in a file.
type Finding struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Col      int      `json:"col"`
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Rule defines a lint check.
type Rule interface {
	ID() string
	Description() string
	Languages() []Language
	Check(file string, content []byte) []Finding
}

// Linter coordinates rule execution across files and languages.
type Linter struct {
	rules []Rule
}

// NewLinter constructs a Linter with the provided rules.
func NewLinter(rules ...Rule) *Linter {
	return &Linter{rules: rules}
}

// DefaultLinter creates a Linter with all built-in rules registered.
func DefaultLinter() *Linter {
	return NewLinter(
		NewBashNoBracketRule(),
		NewBashNoSemicolonRule(),
		NewBashDanglingThenRule(),
		NewBashSourceOverDotRule(),
		NewBashStrictModeRule(),
		NewMarkerIntegrityRule(),
	)
}

// Rules returns registered rules.
func (l *Linter) Rules() []Rule {
	return l.rules
}

// LintFile reads a file from disk, detects language if needed, and checks it.
func (l *Linter) LintFile(path string, lang Language) ([]Finding, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return l.LintBytes(path, content, lang), nil
}

// LintBytes checks in-memory content against applicable rules.
func (l *Linter) LintBytes(path string, content []byte, lang Language) []Finding {
	if lang == LangAuto || lang == "" {
		lang = DetectLanguage(path, content)
	}

	var findings []Finding
	for _, rule := range l.rules {
		if ruleApplies(rule, lang) {
			findings = append(findings, rule.Check(path, content)...)
		}
	}

	// Sort findings deterministically by Line, then Col, then RuleID
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		if findings[i].Col != findings[j].Col {
			return findings[i].Col < findings[j].Col
		}
		return findings[i].RuleID < findings[j].RuleID
	})

	return findings
}

func ruleApplies(rule Rule, lang Language) bool {
	if lang == LangUnknown {
		return false
	}
	for _, target := range rule.Languages() {
		if target == lang {
			return true
		}
	}
	return false
}

// FormatFindingUnix formats a finding in GCC/Unix format: `file:line:col: message [rule_id]`.
func FormatFindingUnix(f Finding) string {
	return fmt.Sprintf("%s:%d:%d: %s [%s]", f.File, f.Line, f.Col, f.Message, f.RuleID)
}
