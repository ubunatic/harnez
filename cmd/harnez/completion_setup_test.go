package main

import (
	"strings"
	"testing"
)

func TestCompletionRendererScriptsForEachShell(t *testing.T) {
	render := completionRenderer(newRootCmd())
	want := map[string]string{
		"bash": "__start_harnez",
		"zsh":  "#compdef harnez",
		"fish": "complete -c harnez",
	}
	for shell, marker := range want {
		script, err := render(shell)
		if err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if !strings.Contains(string(script), marker) {
			t.Errorf("%s script lacks %q", shell, marker)
		}
	}
	if _, err := render("tcsh"); err == nil {
		t.Error("unknown shell rendered a completion script")
	}
}
