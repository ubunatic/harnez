package statusline

import (
	"strings"
	"testing"
)

func TestRenderCollapsesHome(t *testing.T) {
	in := `{"cwd":"/home/uwe/projects/ubunatic.com","workspace":{"current_dir":"/home/uwe/projects/ubunatic.com"}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "~/projects/ubunatic.com"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderFallsBackToWorkspaceCurrentDir(t *testing.T) {
	in := `{"workspace":{"current_dir":"/home/uwe/projects"}}`
	got, err := Render(strings.NewReader(in), "/home/uwe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "~/projects"; got != want {
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
