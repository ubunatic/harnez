package lint

import (
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/glossary"
)

func TestGlossaryRuleInLinter(t *testing.T) {
	g, err := glossary.Load()
	if err != nil {
		t.Fatalf("glossary.Load() error = %v", err)
	}
	linter := NewLinter(NewGlossaryRule(g))

	t.Run("flags forbidden phrase in markdown", func(t *testing.T) {
		content := []byte("# Doc\n\nCheck context used before compacting.\n")
		findings := linter.LintBytes("docs/Sample.md", content, LangAuto)
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1: %+v", len(findings), findings)
		}
		if findings[0].RuleID != "glossary" || findings[0].Line != 3 {
			t.Errorf("finding = %+v, want rule_id=glossary line=3", findings[0])
		}
		if !strings.Contains(findings[0].Message, `"context usage"`) {
			t.Errorf("Message = %q, want mention of \"context usage\"", findings[0].Message)
		}
	})

	t.Run("flags forbidden phrase in Go comment and respects harnez-lint:ignore", func(t *testing.T) {
		src := []byte("package subagent\n\n// context used here\n// harnez-lint:ignore glossary -- allowed\n// context size suppressed\n")
		findings := linter.LintBytes("internal/subagent/foo.go", src, LangAuto)
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1: %+v", len(findings), findings)
		}
		if findings[0].Line != 3 {
			t.Errorf("finding line = %d, want 3", findings[0].Line)
		}
	})
}
