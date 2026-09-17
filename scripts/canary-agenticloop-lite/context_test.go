package main

import "testing"

func TestExpectedContextDocs(t *testing.T) {
	docs := []string{"/assets/AgenticLoop.lite.md", "/assets/Bash.lite.md"}
	if got := expectedContextDocs(docs, "png"); len(got) != 2 || got[0] != "AgenticLoop.lite.png" || got[1] != "Bash.lite.png" {
		t.Fatalf("PNG expected docs = %#v", got)
	}
	if got := expectedContextDocs(docs, "native"); len(got) != 2 || got[0] != "AgenticLoop.lite.md" || got[1] != "Bash.lite.md" {
		t.Fatalf("native expected docs = %#v", got)
	}
}
