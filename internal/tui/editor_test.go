package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/loom"
)

// fakeEditor writes a shell script that records its file argument and then
// runs body (with $1 = file). It returns the script path.
func fakeEditor(t *testing.T, body string) (script, seen string) {
	t.Helper()
	dir := t.TempDir()
	seen = filepath.Join(dir, "seen")
	script = filepath.Join(dir, "fake-editor")
	src := "#!/bin/sh\nprintf '%s' \"$1\" > " + seen + "\n" + body + "\n"
	if err := os.WriteFile(script, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, seen
}

func newTestEditor(t *testing.T, editor string) *Editor {
	t.Helper()
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", editor)
	ed := NewEditor(spec.Editor)
	ed.TempDir = t.TempDir()
	return ed
}

func assertRemoved(t *testing.T, seen string) {
	t.Helper()
	data, err := os.ReadFile(seen)
	if err != nil {
		t.Fatalf("editor did not run: %v", err)
	}
	if _, err := os.Stat(string(data)); !os.IsNotExist(err) {
		t.Errorf("temp file %s not removed (stat err %v)", data, err)
	}
}

func TestEditorRoundtrip(t *testing.T) {
	script, seen := fakeEditor(t, `grep -q '^old text$' "$1" || exit 3
printf 'new\ntext\n' > "$1"`)
	ed := newTestEditor(t, script)
	got, err := ed.Edit("old text")
	if err != nil || got != "new\ntext" {
		t.Fatalf("Edit() = %q, %v; want %q", got, err, "new\ntext")
	}
	assertRemoved(t, seen)
}

func TestEditorNonZeroExit(t *testing.T) {
	script, seen := fakeEditor(t, `printf 'discard' > "$1"; exit 2`)
	ed := newTestEditor(t, script)
	got, err := ed.Edit("keep")
	if err == nil || !strings.Contains(err.Error(), "status 2") || got != "" {
		t.Fatalf("Edit() = %q, %v; want exit status 2 error", got, err)
	}
	assertRemoved(t, seen)
}

func TestEditorEmptyFile(t *testing.T) {
	script, seen := fakeEditor(t, `: > "$1"`)
	ed := newTestEditor(t, script)
	got, err := ed.Edit("something")
	if err != nil || got != "" {
		t.Fatalf("Edit() = %q, %v; want empty text", got, err)
	}
	assertRemoved(t, seen)
}

func TestEditorCommandResolution(t *testing.T) {
	env := map[string]string{}
	ed := &Editor{
		Spec:   EditorSpec{Fallbacks: []string{"nvim", "vim", "nano"}},
		Getenv: func(k string) string { return env[k] },
		LookPath: func(n string) (string, error) {
			if n == "vim" || n == "nano" {
				return "/bin/" + n, nil
			}
			return "", os.ErrNotExist
		},
	}
	check := func(want string) {
		t.Helper()
		argv, err := ed.Command()
		if err != nil || strings.Join(argv, " ") != want {
			t.Fatalf("Command() = %v, %v; want %q", argv, err, want)
		}
	}
	check("/bin/vim")
	env["EDITOR"] = "code -w"
	check("code -w")
	env["VISUAL"] = "hx"
	check("hx")

	ed.Spec.Fallbacks = []string{"nvim"}
	env = map[string]string{}
	if _, err := ed.Command(); err == nil {
		t.Fatal("Command() with no editor should fail")
	}
}

func TestEditorInjectedRunner(t *testing.T) {
	var argv []string
	ed := &Editor{
		Spec:    EditorSpec{TempPattern: "p-*.md"},
		Getenv:  func(k string) string { return map[string]string{"EDITOR": "ed -x"}[k] },
		TempDir: t.TempDir(),
		Run: func(a []string) error {
			argv = a
			return os.WriteFile(a[len(a)-1], []byte("edited\r\n\r\n"), 0o600)
		},
	}
	got, err := ed.Edit("in")
	if err != nil || got != "edited" {
		t.Fatalf("Edit() = %q, %v", got, err)
	}
	if len(argv) != 3 || argv[0] != "ed" || argv[1] != "-x" || !strings.HasPrefix(filepath.Base(argv[2]), "p-") {
		t.Fatalf("argv = %v", argv)
	}
	if _, err := os.Stat(argv[2]); !os.IsNotExist(err) {
		t.Errorf("temp file not removed")
	}
}

// TestAppCtrlGEditor covers the open_editor key: success replaces the prompt
// with the cursor at the end, failure keeps it, and posts queued while the
// editor runs apply after resume.
func TestAppCtrlGEditor(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWidget(spec, "m", "s", nil)
	ctrlG := loom.DecodeKey([]byte{7})
	if keyName(ctrlG) != "ctrl-g" {
		t.Fatalf("Ctrl+G decodes to %q", keyName(ctrlG))
	}

	// Without a pane the key only notes that it is unavailable.
	app.ConsumeKey(ctrlG)

	var queue []func()
	post := func(fn func()) { queue = append(queue, fn) }
	suspend := func(fn func() error) error {
		post(func() { AddTurn(app.Chat, RoleTool, "exec", "posted during suspend") })
		return fn()
	}
	script, _ := fakeEditor(t, `printf 'line1\nline2\n' > "$1"`)
	app.AttachEditor(suspend, newTestEditor(t, script))
	app.Prompt.SetText("draft")
	app.ConsumeKey(ctrlG)
	if app.Prompt.Text() != "line1\nline2" {
		t.Fatalf("prompt = %q after editor", app.Prompt.Text())
	}
	app.ConsumeKey(loom.KeyEvent{Text: "!"})
	if app.Prompt.Text() != "line1\nline2!" {
		t.Fatalf("cursor not at end: %q", app.Prompt.Text())
	}
	for _, fn := range queue {
		fn()
	}
	if !strings.Contains(strings.Join(RenderCanvas(app, 80, 24), "\n"), "posted during suspend") {
		t.Error("post queued during suspend was not applied")
	}

	bad, _ := fakeEditor(t, `exit 1`)
	app.editor = newTestEditor(t, bad)
	app.ConsumeKey(ctrlG)
	if app.Prompt.Text() != "line1\nline2!" {
		t.Fatalf("prompt = %q after failed editor, want kept", app.Prompt.Text())
	}
	if out := strings.Join(RenderCanvas(app, 80, 24), "\n"); !strings.Contains(out, "prompt kept") {
		t.Error("missing status note after failed editor")
	}
}
