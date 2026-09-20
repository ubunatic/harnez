package subagent

import (
	"regexp"
	"testing"
)

func TestGenerateSessionName(t *testing.T) {
	first, err := GenerateSessionName(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[a-z]+-[a-z]+$`).MatchString(first) {
		t.Fatalf("generated name %q is not memorable adjective-animal form", first)
	}
	second, err := GenerateSessionName(func(candidate string) bool { return candidate == first })
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatalf("generated duplicate name %q", second)
	}
}

func TestGenerateSessionNameExhausted(t *testing.T) {
	if _, err := GenerateSessionName(func(string) bool { return true }); err == nil {
		t.Fatal("expected exhausted name pool error")
	}
}
