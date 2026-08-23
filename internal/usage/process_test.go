package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsAgentName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude", "claude"},
		{"/usr/local/bin/claude", "claude"},
		{"agy", "agy"},
		{"/home/uwe/.local/bin/agy", "agy"},
		{"codex", "codex"},
		{"/opt/codex/bin/codex", "codex"},
		{"bash", ""},
		{"node", ""},
		{"claude-wrapper", ""},
	}

	for _, tt := range tests {
		got := isAgentName(tt.input)
		if got != tt.want {
			t.Errorf("isAgentName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCountProcessesFromProc(t *testing.T) {
	tempProc := t.TempDir()

	// Create fake PID 101: claude via comm
	pid101 := filepath.Join(tempProc, "101")
	if err := os.Mkdir(pid101, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid101, "comm"), []byte("claude\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create fake PID 102: agy via comm
	pid102 := filepath.Join(tempProc, "102")
	if err := os.Mkdir(pid102, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid102, "comm"), []byte("agy\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create fake PID 103: codex via cmdline (comm is empty/wrapper)
	pid103 := filepath.Join(tempProc, "103")
	if err := os.Mkdir(pid103, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid103, "comm"), []byte("node\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid103, "cmdline"), append([]byte("/usr/bin/codex"), 0, 'f', 'o', 'o', 0), 0644); err != nil {
		t.Fatal(err)
	}

	// Create fake PID 104: unrelated process
	pid104 := filepath.Join(tempProc, "104")
	if err := os.Mkdir(pid104, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid104, "comm"), []byte("zsh\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Non-numeric directory
	if err := os.Mkdir(filepath.Join(tempProc, "self"), 0755); err != nil {
		t.Fatal(err)
	}

	counts, err := countProcessesFromProc(tempProc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if counts.Claude != 1 {
		t.Errorf("expected 1 claude, got %d", counts.Claude)
	}
	if counts.AGY != 1 {
		t.Errorf("expected 1 agy, got %d", counts.AGY)
	}
	if counts.Codex != 1 {
		t.Errorf("expected 1 codex, got %d", counts.Codex)
	}
	if counts.Total() != 3 {
		t.Errorf("expected total 3, got %d", counts.Total())
	}
}

func TestCountRunningAgentProcesses(t *testing.T) {
	// Should run cleanly without crashing on the host
	counts := CountRunningAgentProcesses()
	if counts.Total() < 0 {
		t.Errorf("invalid total: %d", counts.Total())
	}
}
