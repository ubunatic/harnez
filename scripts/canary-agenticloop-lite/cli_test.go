package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCLIDryRunScenarios(t *testing.T) {
	scenarios := [][]string{
		{"run", "shell-conditional", "--dry-run"},
		{"run", "shell-conditional", "--dry-run", "--png", "--soft", "--claude", "--lite"},
		{"run", "hello", "--dry-run", "--text", "--hard", "--full"},
	}
	for _, args := range scenarios {
		cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go run %v: %v\n%s", args, err, out)
		}
		text := string(out)
		if !strings.Contains(text, "PASS (dry-run)") || !strings.Contains(text, "AGENTS.md") {
			t.Fatalf("go run %v output missing dry-run result/tree:\n%s", args, text)
		}
	}
}
