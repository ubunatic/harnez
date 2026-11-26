package usagereport

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// line is one transcript assistant line with usage (fixture, no real data).
func line(id, model, at string, output int) string {
	return `{"type":"assistant","timestamp":"` + at + `","sessionId":"ignored","message":{"id":"` + id +
		`","model":"` + model + `","usage":{"input_tokens":5,"output_tokens":` + strconv.Itoa(output) +
		`,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n"
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A message streamed to the main transcript and repeated in a nested sidechain
// file counts once, with its final (largest) output; the nested file belongs to
// the parent session by its directory name.
func TestScanCallsDedupesNestedFiles(t *testing.T) {
	projects := filepath.Join(t.TempDir(), "projects")
	main := filepath.Join(projects, "-work-proj", "s1.jsonl")
	nested := filepath.Join(projects, "-work-proj", "s1", "subagents", "agent-a.jsonl")
	writeFile(t, main, line("m1", "claude-haiku-5-5", "2026-11-13T10:01:00Z", 4)+
		line("m1", "claude-haiku-5-5", "2026-11-13T10:01:01Z", 10)+
		line("m2", "claude-opus-5-5", "2026-11-13T10:02:00Z", 7))
	writeFile(t, nested, line("m1", "claude-haiku-5-5", "2026-11-13T10:01:02Z", 12)+
		line("m3", "claude-haiku-5-5", "2026-11-13T10:03:00Z", 1))
	calls, err := ScanCalls(projects)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %d, want 3 (m1, m2, m3)", len(calls))
	}
	byID := map[string]Call{}
	for _, c := range calls {
		byID[c.MessageID] = c
	}
	if m1 := byID["m1"]; m1.Usage.Output != 12 || m1.Session != "s1" || m1.Usage.Model != "claude-haiku-5-5" {
		t.Fatalf("m1 = %+v, want final output 12 in session s1", m1)
	}
	if m2 := byID["m2"]; m2.Usage.Model != "claude-opus-5-5" || m2.At.IsZero() {
		t.Fatalf("m2 = %+v, want its own model and a timestamp", m2)
	}
}

// Lines without usage, all-zero usage (the synthetic post-compact events) and
// a missing message id are skipped, never counted as zero-token calls.
func TestScanCallsSkipsNonCalls(t *testing.T) {
	projects := filepath.Join(t.TempDir(), "projects")
	zero := `{"type":"assistant","timestamp":"2026-11-13T10:00:00Z","message":{"id":"z","model":"claude-haiku-5-5","usage":{"input_tokens":0,"output_tokens":0}}}` + "\n"
	noID := `{"type":"assistant","timestamp":"2026-11-13T10:00:00Z","message":{"model":"claude-haiku-5-5","usage":{"input_tokens":9,"output_tokens":9}}}` + "\n"
	user := `{"type":"user","timestamp":"2026-11-13T10:00:00Z","message":{"id":"u","content":"hi"}}` + "\n"
	writeFile(t, filepath.Join(projects, "-w", "s2.jsonl"), zero+noID+user+"not json\n")
	calls, err := ScanCalls(projects)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("calls = %+v, want none", calls)
	}
}

// No projects directory means no calls, not an error.
func TestScanCallsMissingDirectoryIsEmpty(t *testing.T) {
	calls, err := ScanCalls(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(calls) != 0 {
		t.Fatalf("missing dir: calls=%d err=%v, want empty and nil", len(calls), err)
	}
}
