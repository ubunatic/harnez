package readdiscipline

import (
	"strings"
	"testing"
)

func TestEmbeddedSpecLoads(t *testing.T) {
	spec, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if spec.MaxLines != 100 {
		t.Errorf("MaxLines = %d, want 100", spec.MaxLines)
	}
	if !strings.Contains(spec.Deny(), "fewer than 100 lines") {
		t.Errorf("Deny() does not render the spec threshold: %q", spec.Deny())
	}
	if !strings.Contains(spec.Guidance(), "over 100 lines") {
		t.Errorf("Guidance() does not render the spec threshold: %q", spec.Guidance())
	}
}

// A changed threshold in the spec must reach the rendered text: no literal
// 100 may survive in Go code.
func TestThresholdComesFromSpec(t *testing.T) {
	spec, err := Parse([]byte(`max_lines: 42
deny_message: "block at {max_lines} lines, read fewer than {max_lines}"
subagent_guidance: "files over {max_lines} lines"
`))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	if spec.MaxLines != 42 {
		t.Errorf("MaxLines = %d, want 42", spec.MaxLines)
	}
	if got, want := spec.Deny(), "block at 42 lines, read fewer than 42"; got != want {
		t.Errorf("Deny() = %q, want %q", got, want)
	}
	if got, want := spec.Guidance(), "files over 42 lines"; got != want {
		t.Errorf("Guidance() = %q, want %q", got, want)
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"zero threshold", "max_lines: 0\ndeny_message: \"a {max_lines}\"\nsubagent_guidance: \"b {max_lines}\"\n"},
		{"missing guidance", "max_lines: 100\ndeny_message: \"a {max_lines}\"\n"},
		{"unknown key", "max_lines: 100\ndeny_message: \"a {max_lines}\"\nsubagent_guidance: \"b {max_lines}\"\nextra: 1\n"},
		{"deny without placeholder", "max_lines: 100\ndeny_message: \"no placeholder\"\nsubagent_guidance: \"b {max_lines}\"\n"},
		{"guidance without placeholder", "max_lines: 100\ndeny_message: \"a {max_lines}\"\nsubagent_guidance: \"no placeholder\"\n"},
		{"string threshold", "max_lines: \"100\"\ndeny_message: \"a {max_lines}\"\nsubagent_guidance: \"b {max_lines}\"\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.yaml)); err == nil {
				t.Fatal("Parse() accepted invalid spec, want error")
			}
		})
	}
}
