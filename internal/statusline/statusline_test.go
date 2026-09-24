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
