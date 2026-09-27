package statusline

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestRenderCollapsesHome(t *testing.T) {
	in := `{"cwd":"/home/uwe/projects/ubunatic.com","workspace":{"current_dir":"/home/uwe/projects/ubunatic.com","project_dir":"/home/uwe/projects/ubunatic.com"}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "~/projects/ubunatic.com"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderUsesWorkspaceCurrentDir(t *testing.T) {
	in := `{"cwd":"/tmp/ignored","workspace":{"current_dir":"/home/uwe/projects","project_dir":"/home/uwe/projects"}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "~/projects"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderUnicodePathDisplayWidth(t *testing.T) {
	in := `{"workspace":{"current_dir":"/home/uwe/项目/voxi","project_dir":"/tmp"}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp → ~/项目/voxi"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if width := runewidth.StringWidth(got); width != 18 {
		t.Fatalf("display width = %d, want 18 for %q", width, got)
	}
}

func TestRenderPrefersCurrentDirToCWD(t *testing.T) {
	in := `{"cwd":"/home/uwe/projects/harnez","workspace":{"current_dir":"/home/uwe/projects/voxi","project_dir":"/home/uwe/projects/voxi","added_dirs":[],"repo":{"host":"codeberg.org","owner":"uwe","name":"voxi"}},"session_id":"session-1","session_name":"voxi","transcript_path":"/tmp/transcript"}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "~/projects/voxi"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderShowsProjectDrift(t *testing.T) {
	in := `{"cwd":"/home/uwe/projects/harnez","workspace":{"current_dir":"/home/uwe/projects/voxi","project_dir":"/tmp","added_dirs":[],"repo":{"host":"codeberg.org","owner":"uwe","name":"voxi"}}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp → ~/projects/voxi"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderFallsBackToCWDAndShowsProjectDrift(t *testing.T) {
	in := `{"cwd":"/home/uwe/projects/harnez","workspace":{"project_dir":"/tmp"}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp → ~/projects/harnez"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderHomeItself(t *testing.T) {
	in := `{"cwd":"/home/uwe"}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "~"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderOutsideHomeUnchanged(t *testing.T) {
	in := `{"cwd":"/etc/foo"}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/etc/foo"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderInvalidJSON(t *testing.T) {
	_, err := Render(strings.NewReader("not json"), "/home/uwe")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRenderClaudeContextUsageFromStatusLinePayload(t *testing.T) {
	// Claude Code documents these fields under context_window.current_usage.
	in := `{"workspace":{"current_dir":"/work","project_dir":"/work"},"context_window":{"context_window_size":200000,"total_input_tokens":156000,"current_usage":{"input_tokens":90000,"cache_creation_input_tokens":50000,"cache_read_input_tokens":16000}}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/work · 156k (10% cached)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderClaudeContextUsageUnavailable(t *testing.T) {
	for _, in := range []string{
		`{"cwd":"/work","context_window":{"current_usage":null}}`,
		`{"cwd":"/work","context_window":{}}`,
	} {
		got, err := Render(strings.NewReader(in), "/home/uwe")
		if err != nil {
			t.Fatal(err)
		}
		if want := "/work"; got != want {
			t.Errorf("payload %s: got %q, want %q", in, got, want)
		}
	}
}
