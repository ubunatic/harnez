package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunClaudeInstructionsHook(t *testing.T) {
	var out bytes.Buffer
	if err := runClaudeInstructionsHook(&out); err != nil {
		t.Fatalf("runClaudeInstructionsHook: %v", err)
	}
	for _, want := range []string{"Never propose or edit the literal CxxxE.md", "Always target AGENTS.md", "check the repository's docs/ and AGENTS.md"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("instruction output missing %q: %s", want, out.String())
		}
	}
}
