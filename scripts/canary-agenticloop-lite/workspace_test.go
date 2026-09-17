package main

import (
	"reflect"
	"testing"
)

func TestExpectedWorkspaceFiles(t *testing.T) {
	docs := []string{"/assets/AgenticLoop.lite.md", "/assets/Bash.lite.md"}
	checks := []struct {
		link, delivery string
		want           []string
	}{
		{"soft", "native", []string{"AGENTS.md"}},
		{"hard", "native", []string{"AGENTS.md", "AgenticLoop.lite.md", "Bash.lite.md"}},
		{"soft", "png", []string{"AGENTS.md", "AgenticLoop.lite.png", "Bash.lite.png"}},
	}
	for _, check := range checks {
		if got := expectedWorkspaceFiles(docs, check.link, check.delivery); !reflect.DeepEqual(got, check.want) {
			t.Errorf("%s/%s: got %#v, want %#v", check.link, check.delivery, got, check.want)
		}
	}
}
