package lint

import (
	"strings"
	"testing"
)

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		input   string
		want    Language
		wantErr bool
	}{
		{"auto", LangAuto, false},
		{"", LangAuto, false},
		{"bash", LangBash, false},
		{"sh", LangBash, false},
		{"shell", LangBash, false},
		{"go", LangGo, false},
		{"golang", LangGo, false},
		{"make", LangMake, false},
		{"makefile", LangMake, false},
		{"mk", LangMake, false},
		{"md", LangMarkdown, false},
		{"markdown", LangMarkdown, false},
		{"invalid", LangUnknown, true},
	}

	for _, tt := range tests {
		got, err := ParseLanguage(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLanguage(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("ParseLanguage(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path    string
		content string
		want    Language
	}{
		{"script.sh", "", LangBash},
		{"deploy.bash", "", LangBash},
		{"main.go", "", LangGo},
		{"Makefile", "", LangMake},
		{"rules.mk", "", LangMake},
		{"doc.md", "", LangMarkdown},
		{"README.markdown", "", LangMarkdown},
		{"custom_script", "#!/usr/bin/env bash\necho hi", LangBash},
		{"unknown.txt", "just plain text", LangUnknown},
	}

	for _, tt := range tests {
		got := DetectLanguage(tt.path, []byte(tt.content))
		if got != tt.want {
			t.Errorf("DetectLanguage(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestBashNoBracketRule(t *testing.T) {
	linter := NewLinter(NewBashNoBracketRule())

	badScript := `#!/usr/bin/env bash
if [ "$a" = "$b" ]; then
    echo "equal"
fi

if [[ "$a" == "$b" ]]; then
    echo "double"
fi

elif [ -f "$f" ]; then
    echo "found"

[ "$x" = "$y" ] && echo "yes"
`

	findings := linter.LintBytes("test.sh", []byte(badScript), LangBash)
	if len(findings) < 4 {
		t.Fatalf("expected at least 4 findings, got %d: %+v", len(findings), findings)
	}

	for _, f := range findings {
		if f.RuleID != "bash-no-bracket" {
			t.Errorf("unexpected rule ID: %s", f.RuleID)
		}
	}

	goodScript := `#!/usr/bin/env bash
set -euo pipefail
if test "$a" = "$b"
then printf 'equal\n'
fi

grep -ao '\[[A-Za-z]\]' file.txt
array[0]=1
`
	goodFindings := linter.LintBytes("test.sh", []byte(goodScript), LangBash)
	if len(goodFindings) != 0 {
		t.Fatalf("expected 0 findings for good script, got %d: %+v", len(goodFindings), goodFindings)
	}
}

func TestBashNoSemicolonRule(t *testing.T) {
	linter := NewLinter(NewBashNoSemicolonRule())

	badScript := `if test -f "$file"; then
    echo "bad"
fi
while test "$x" != "$y"; do
    echo "loop"
done
`
	findings := linter.LintBytes("test.sh", []byte(badScript), LangBash)
	if len(findings) != 2 {
		t.Fatalf("expected 2 semicolon findings, got %d: %+v", len(findings), findings)
	}

	goodScript := `if test -f "$file"
then echo "good"
fi
echo "; then this is in quotes"
`
	goodFindings := linter.LintBytes("test.sh", []byte(goodScript), LangBash)
	if len(goodFindings) != 0 {
		t.Fatalf("expected 0 findings for good script, got %d: %+v", len(goodFindings), goodFindings)
	}
}

func TestBashDanglingThenRule(t *testing.T) {
	linter := NewLinter(NewBashDanglingThenRule())

	badScript := `if test -f "$file"
then
    echo "bad"
fi
`
	findings := linter.LintBytes("test.sh", []byte(badScript), LangBash)
	if len(findings) != 1 {
		t.Fatalf("expected 1 dangling then finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Line != 2 {
		t.Errorf("expected finding on line 2, got line %d", findings[0].Line)
	}

	goodScript := `if test -f "$file"
then echo "good"
fi
`
	goodFindings := linter.LintBytes("test.sh", []byte(goodScript), LangBash)
	if len(goodFindings) != 0 {
		t.Fatalf("expected 0 findings, got %+v", goodFindings)
	}
}

func TestBashSourceOverDotRule(t *testing.T) {
	linter := NewLinter(NewBashSourceOverDotRule())

	badScript := `. ~/.bashrc
. "$DIR/lib.sh"
`
	findings := linter.LintBytes("test.sh", []byte(badScript), LangBash)
	if len(findings) != 2 {
		t.Fatalf("expected 2 dot-sourcing findings, got %d: %+v", len(findings), findings)
	}

	goodScript := `source ~/.bashrc
source "$DIR/lib.sh"
./script.sh
find . -name "*.go"
`
	goodFindings := linter.LintBytes("test.sh", []byte(goodScript), LangBash)
	if len(goodFindings) != 0 {
		t.Fatalf("expected 0 findings, got %+v", goodFindings)
	}
}

func TestMarkerIntegrityRule(t *testing.T) {
	linter := NewLinter(NewMarkerIntegrityRule())

	goodMD := `# Title
<!-- harnez:begin Section One -->
Some managed content
<!-- harnez:end Section One -->
<!-- harnez:begin Section Two -->
More content
<!-- harnez:end Section Two -->
`
	if findings := linter.LintBytes("test.md", []byte(goodMD), LangMarkdown); len(findings) != 0 {
		t.Fatalf("expected 0 findings for valid markers, got %+v", findings)
	}

	unclosedMD := `# Title
<!-- harnez:begin Section One -->
Unclosed content
`
	findings := linter.LintBytes("test.md", []byte(unclosedMD), LangMarkdown)
	if len(findings) != 1 || findings[0].RuleID != "marker-unclosed" {
		t.Fatalf("expected marker-unclosed finding, got %+v", findings)
	}

	mismatchedMD := `# Title
<!-- harnez:begin Section One -->
Content
<!-- harnez:end Section Two -->
`
	findings = linter.LintBytes("test.md", []byte(mismatchedMD), LangMarkdown)
	if len(findings) != 1 || findings[0].RuleID != "marker-mismatch" {
		t.Fatalf("expected marker-mismatch finding, got %+v", findings)
	}

	unopenedMD := `# Title
Content
<!-- harnez:end Section One -->
`
	findings = linter.LintBytes("test.md", []byte(unopenedMD), LangMarkdown)
	if len(findings) != 1 || findings[0].RuleID != "marker-unopened" {
		t.Fatalf("expected marker-unopened finding, got %+v", findings)
	}

	// Makefile markers
	goodMK := `# harnez:begin Section One
all:
	@echo hi
# harnez:end Section One
`
	if findings := linter.LintBytes("Makefile", []byte(goodMK), LangMake); len(findings) != 0 {
		t.Fatalf("expected 0 findings for valid makefile markers, got %+v", findings)
	}
}

func TestFormatFindingUnix(t *testing.T) {
	f := Finding{
		File:     "scripts/run.sh",
		Line:     15,
		Col:      4,
		RuleID:   "bash-no-bracket",
		Severity: SeverityError,
		Message:  "forbidden bracket conditional",
	}

	formatted := FormatFindingUnix(f)
	expected := "scripts/run.sh:15:4: forbidden bracket conditional [bash-no-bracket]"
	if formatted != expected {
		t.Errorf("got %q, want %q", formatted, expected)
	}
}

func TestDefaultLinterIntegration(t *testing.T) {
	dl := DefaultLinter()
	content := `#!/usr/bin/env bash
if [ -f "$file" ]; then
then
. ~/.bashrc
fi
`
	findings := dl.LintBytes("script.sh", []byte(content), LangAuto)
	if len(findings) == 0 {
		t.Fatal("expected findings from default linter")
	}

	var rules []string
	for _, f := range findings {
		rules = append(rules, f.RuleID)
	}
	ruleList := strings.Join(rules, ",")
	if !strings.Contains(ruleList, "bash-no-bracket") {
		t.Errorf("missing bash-no-bracket in %s", ruleList)
	}
	if !strings.Contains(ruleList, "bash-no-semicolon") {
		t.Errorf("missing bash-no-semicolon in %s", ruleList)
	}
	if !strings.Contains(ruleList, "bash-source-over-dot") {
		t.Errorf("missing bash-source-over-dot in %s", ruleList)
	}
}
