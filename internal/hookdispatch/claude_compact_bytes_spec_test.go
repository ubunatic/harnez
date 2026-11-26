package hookdispatch

import (
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

// Issue 1303 M3: the bound on the compaction transcript read, from the hooks spec.
const maxTranscriptBytesLine = "max_transcript_bytes: 8388608"

func TestLoadClaudeCompactMaxTranscriptBytesReadsEmbeddedSpec(t *testing.T) {
	got, err := LoadClaudeCompactMaxTranscriptBytes()
	if err != nil {
		t.Fatalf("LoadClaudeCompactMaxTranscriptBytes: %v", err)
	}
	if got != 8388608 {
		t.Errorf("LoadClaudeCompactMaxTranscriptBytes = %d, want 8388608", got)
	}
}

func TestParseClaudeCompactMaxTranscriptBytesRejectsBadValues(t *testing.T) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	base := string(raw)
	if !strings.Contains(base, maxTranscriptBytesLine) {
		t.Fatalf("spec has no %q line", maxTranscriptBytesLine)
	}
	cases := map[string]string{
		"zero":       strings.Replace(base, maxTranscriptBytesLine, "max_transcript_bytes: 0", 1),
		"negative":   strings.Replace(base, maxTranscriptBytesLine, "max_transcript_bytes: -1", 1),
		"not number": strings.Replace(base, maxTranscriptBytesLine, "max_transcript_bytes: many", 1),
		"missing":    strings.Replace(base, maxTranscriptBytesLine+"\n", "", 1),
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if spec == base {
				t.Fatal("test case did not change the spec")
			}
			if _, err := parseClaudeCompactMaxTranscriptBytes([]byte(spec)); err == nil {
				t.Error("parseClaudeCompactMaxTranscriptBytes accepted the bad spec, want an error")
			}
		})
	}
}
