package lint

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// BashNoBracketRule flags forbidden `[` or `[[` conditionals in Bash scripts.
type BashNoBracketRule struct{}

func NewBashNoBracketRule() *BashNoBracketRule {
	return &BashNoBracketRule{}
}

func (r *BashNoBracketRule) ID() string {
	return "bash-no-bracket"
}

func (r *BashNoBracketRule) Description() string {
	return "Flag forbidden `[` or `[[` conditionals in favor of `if test`"
}

func (r *BashNoBracketRule) Languages() []Language {
	return []Language{LangBash}
}

// Regex matching `if [`, `if [[`, `elif [`, `while [`, `until [`, or standalone `[ ... ]` / `[[ ... ]]`
var (
	reIfBracket  = regexp.MustCompile(`(^|\s|;|&&|\|\||!)(if|elif|while|until)\s+(\[|\[\[)\s`)
	reCmdBracket = regexp.MustCompile(`(^|;|&&|\|\||!|\()\s*(\[|\[\[)\s`)
)

func (r *BashNoBracketRule) Check(file string, content []byte) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		code, _ := stripComment(line)
		if strings.TrimSpace(code) == "" {
			continue
		}

		// Check inside code (outside quotes)
		unquoted, charMap := maskQuotes(code)

		if loc := reIfBracket.FindStringSubmatchIndex(unquoted); loc != nil {
			bracketIdx := loc[6]
			origCol := charMap[bracketIdx] + 1
			findings = append(findings, Finding{
				File:     file,
				Line:     lineNum,
				Col:      origCol,
				RuleID:   r.ID(),
				Severity: SeverityError,
				Message:  "forbidden bracket conditional '[' or '[['; use 'if test ...' instead",
			})
			continue
		}

		if loc := reCmdBracket.FindStringSubmatchIndex(unquoted); loc != nil {
			bracketIdx := loc[4]
			origCol := charMap[bracketIdx] + 1
			findings = append(findings, Finding{
				File:     file,
				Line:     lineNum,
				Col:      origCol,
				RuleID:   r.ID(),
				Severity: SeverityError,
				Message:  "forbidden bracket conditional '[' or '[['; use 'if test ...' instead",
			})
		}
	}

	return findings
}

// BashNoSemicolonRule flags forbidden `; then` or `; do` constructs.
type BashNoSemicolonRule struct{}

func NewBashNoSemicolonRule() *BashNoSemicolonRule {
	return &BashNoSemicolonRule{}
}

func (r *BashNoSemicolonRule) ID() string {
	return "bash-no-semicolon"
}

func (r *BashNoSemicolonRule) Description() string {
	return "Flag forbidden semicolons before `then` or `do`"
}

func (r *BashNoSemicolonRule) Languages() []Language {
	return []Language{LangBash}
}

var reSemiThenDo = regexp.MustCompile(`;\s*(then|do)\b`)

func (r *BashNoSemicolonRule) Check(file string, content []byte) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		code, _ := stripComment(line)
		if strings.TrimSpace(code) == "" {
			continue
		}

		unquoted, charMap := maskQuotes(code)
		if loc := reSemiThenDo.FindStringIndex(unquoted); loc != nil {
			semiIdx := loc[0]
			origCol := charMap[semiIdx] + 1
			keyword := strings.TrimSpace(unquoted[semiIdx+1 : loc[1]])
			findings = append(findings, Finding{
				File:     file,
				Line:     lineNum,
				Col:      origCol,
				RuleID:   r.ID(),
				Severity: SeverityError,
				Message:  "forbidden semicolon before '" + keyword + "'; break lines before '" + keyword + "' instead",
			})
		}
	}

	return findings
}

// BashDanglingThenRule flags `then` on its own line without a command on the same line.
type BashDanglingThenRule struct{}

func NewBashDanglingThenRule() *BashDanglingThenRule {
	return &BashDanglingThenRule{}
}

func (r *BashDanglingThenRule) ID() string {
	return "bash-dangling-then"
}

func (r *BashDanglingThenRule) Description() string {
	return "Flag standalone `then` without immediate command on the same line"
}

func (r *BashDanglingThenRule) Languages() []Language {
	return []Language{LangBash}
}

func (r *BashDanglingThenRule) Check(file string, content []byte) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		code, _ := stripComment(line)
		trimmed := strings.TrimSpace(code)

		if trimmed == "then" {
			col := strings.Index(line, "then") + 1
			findings = append(findings, Finding{
				File:     file,
				Line:     lineNum,
				Col:      col,
				RuleID:   r.ID(),
				Severity: SeverityError,
				Message:  "standalone 'then' on its own line; place the first command directly after 'then' ('then <cmd>')",
			})
		}
	}

	return findings
}

// BashSourceOverDotRule flags `.` used for sourcing instead of `source`.
type BashSourceOverDotRule struct{}

func NewBashSourceOverDotRule() *BashSourceOverDotRule {
	return &BashSourceOverDotRule{}
}

func (r *BashSourceOverDotRule) ID() string {
	return "bash-source-over-dot"
}

func (r *BashSourceOverDotRule) Description() string {
	return "Flag `.` used for script sourcing in favor of `source`"
}

func (r *BashSourceOverDotRule) Languages() []Language {
	return []Language{LangBash}
}

var reDotSource = regexp.MustCompile(`(^|;|&&|\|\||\bthen\b|\belse\b|\bdo\b|\()\s*(\.\s+)`)

func (r *BashSourceOverDotRule) Check(file string, content []byte) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		code, _ := stripComment(line)
		if strings.TrimSpace(code) == "" {
			continue
		}

		unquoted, _ := maskQuotes(code)
		if loc := reDotSource.FindStringSubmatchIndex(unquoted); loc != nil {
			dotIdx := loc[4]
			origCol := dotIdx + 1
			afterDot := strings.TrimSpace(code[dotIdx+1:])
			// Avoid matching "." if target is empty or standard flag
			if afterDot != "" && !strings.HasPrefix(afterDot, "-") {
				findings = append(findings, Finding{
					File:     file,
					Line:     lineNum,
					Col:      origCol,
					RuleID:   r.ID(),
					Severity: SeverityError,
					Message:  "prefer 'source' over '.' for script and dotfile inclusion",
				})
			}
		}
	}

	return findings
}

// BashStrictModeRule checks that scripts with a bash shebang have `set -euo pipefail`.
type BashStrictModeRule struct{}

func NewBashStrictModeRule() *BashStrictModeRule {
	return &BashStrictModeRule{}
}

func (r *BashStrictModeRule) ID() string {
	return "bash-strict-mode"
}

func (r *BashStrictModeRule) Description() string {
	return "Flag missing `set -euo pipefail` strict mode header in bash scripts"
}

func (r *BashStrictModeRule) Languages() []Language {
	return []Language{LangBash}
}

func (r *BashStrictModeRule) Check(file string, content []byte) []Finding {
	var findings []Finding
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return nil
	}

	firstLine := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(firstLine, "#!") || !strings.Contains(firstLine, "bash") {
		return nil // only enforce strict mode on scripts with explicit bash shebang
	}

	firstCodeLine := 0
	hasStrict := false

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if firstCodeLine == 0 {
			firstCodeLine = i + 1
		}
		if strings.HasPrefix(line, "set -euo pipefail") || strings.HasPrefix(line, "set -e") {
			hasStrict = true
			break
		}
		// If we've seen non-empty non-comment code statements without strict mode, stop
		break
	}

	if !hasStrict {
		targetLine := 2
		if firstCodeLine > 0 {
			targetLine = firstCodeLine
		}
		findings = append(findings, Finding{
			File:     file,
			Line:     targetLine,
			Col:      1,
			RuleID:   r.ID(),
			Severity: SeverityWarning,
			Message:  "missing 'set -euo pipefail' strict mode header in bash script",
		})
	}

	return findings
}

// stripComment separates the executable code from trailing comments in a line.
func stripComment(line string) (string, int) {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if ch == '#' && !inSingle && !inDouble {
			return line[:i], i
		}
	}
	return line, -1
}

// maskQuotes replaces quoted contents with spaces, preserving string length and original indices.
func maskQuotes(code string) (string, []int) {
	out := make([]byte, len(code))
	charMap := make([]int, len(code))
	for i := 0; i < len(code); i++ {
		charMap[i] = i
	}

	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(code); i++ {
		ch := code[i]
		if escaped {
			escaped = false
			out[i] = ' '
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			out[i] = ' '
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			out[i] = ' '
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			out[i] = ' '
			continue
		}
		if inSingle || inDouble {
			out[i] = ' '
		} else {
			out[i] = ch
		}
	}
	return string(out), charMap
}
