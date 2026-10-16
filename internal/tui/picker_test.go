package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/tui/router"
	"ubunatic.com/loom"
)

// newPickerApp returns an app whose pickers read files (pre-indexed) and
// the skills in skillDirs; submits are recorded in *sent.
func newPickerApp(t *testing.T, files []string, skillDirs ...string) (*AppWidget, *asyncLoop, *[]string) {
	t.Helper()
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	sent := &[]string{}
	app := NewAppWidget(spec, "claude:sonnet:low", "s", func(p string) { *sent = append(*sent, p) })
	l := &asyncLoop{}
	app.AttachRunner(nil, l.post, router.ResolveSpec)
	app.AttachPickers(t.TempDir(), skillDirs, l.post)
	t.Cleanup(app.Shutdown)
	if files != nil {
		s := app.sources
		s.files.state = lazyReady
		s.files.items = fileItems(files)
	}
	return app, l, sent
}

func typeText(app *AppWidget, s string) {
	for _, r := range s {
		app.ConsumeKey(loom.KeyEvent{Text: string(r)})
	}
}

func matchNames(app *AppWidget) []string {
	var out []string
	for _, m := range app.picker.matches {
		out = append(out, m.Name)
	}
	return out
}

var repoFiles = []string{"README.md", "cmd/harnez/tui.go", "internal/tui/app.go", "internal/tui/apply_test.go", "internal/claude/apply.go", "docs/Go.md"}

func TestPickerTriggerParsing(t *testing.T) {
	cases := []struct {
		typed string
		want  pickKind
	}{
		{"@", pickFile},
		{"see @", pickFile},
		{"a@", pickNone},       // mid-word: e-mail addresses stay text
		{"a@b", pickNone},      // still mid-word
		{"/", pickCommand},     // very start
		{"x /", pickNone},      // not at the start
		{" /", pickNone},       // leading space is not the start
		{"see @x y", pickNone}, // a space ends the file query
		{"/mod", pickCommand},
	}
	for _, tc := range cases {
		app, _, _ := newPickerApp(t, repoFiles)
		typeText(app, tc.typed)
		if app.picker.kind != tc.want {
			t.Errorf("%q: picker kind %d, want %d", tc.typed, app.picker.kind, tc.want)
		}
	}
	// "/" on a later line is not the start of the prompt.
	app, _, _ := newPickerApp(t, repoFiles)
	typeText(app, "x")
	app.ConsumeKey(loom.KeyEvent{Key: "alt-enter"})
	typeText(app, "/")
	if app.picker.isOpen() {
		t.Error("/ on line 2 opened a picker")
	}
	// Without AttachPickers there is no file index, so @ stays text.
	spec, _ := LoadSpec()
	bare := NewAppWidget(spec, "m", "s", nil)
	typeText(bare, "@")
	if bare.picker.isOpen() {
		t.Error("@ opened a picker without a file source")
	}
}

func TestRankItems(t *testing.T) {
	items := fileItems(repoFiles)
	names := func(q string, limit int) []string {
		var out []string
		for _, it := range rankItems(items, q, limit) {
			out = append(out, it.Name)
		}
		return out
	}
	if got := names("", 3); strings.Join(got, ",") != "README.md,cmd/harnez/tui.go,internal/tui/app.go" {
		t.Errorf("empty query = %v, want source order capped at 3", got)
	}
	got := names("app", 10)
	if len(got) != 3 || got[0] != "internal/tui/app.go" {
		t.Errorf("app = %v, want internal/tui/app.go first of 3", got)
	}
	if got := names("tui/app", 10); len(got) == 0 || got[0] != "internal/tui/app.go" {
		t.Errorf("tui/app = %v", got)
	}
	if got := names("aply", 10); len(got) != 0 {
		t.Errorf("short typo matched %v; 4-char tokens get no edit budget", got)
	}
	if got := names("tuigo", 10); len(got) != 0 {
		t.Errorf("unrelated query matched %v", got)
	}
	if got := names("app", 1); len(got) != 1 {
		t.Errorf("limit 1 = %v", got)
	}
}

func TestFilePickerSelectInsertsPath(t *testing.T) {
	for _, key := range []string{"enter", "tab"} {
		app, _, sent := newPickerApp(t, repoFiles)
		typeText(app, "see @tui/ap")
		if m := matchNames(app); len(m) == 0 || m[0] != "internal/tui/app.go" {
			t.Fatalf("matches = %v", m)
		}
		app.ConsumeKey(loom.KeyEvent{Key: key})
		if got, want := app.Prompt.Text(), "see @internal/tui/app.go "; got != want || len(*sent) != 0 || app.picker.isOpen() {
			t.Fatalf("%s: text %q sent %v open %v; want %q, nothing sent, closed", key, got, *sent, app.picker.isOpen(), want)
		}
		typeText(app, "please")
		app.ConsumeKey(loom.KeyEvent{Key: "enter"})
		if len(*sent) != 1 || (*sent)[0] != "see @internal/tui/app.go please" {
			t.Fatalf("closed picker: Enter sent %v", *sent)
		}
	}
}

func TestPickerUpDownSelect(t *testing.T) {
	app, _, _ := newPickerApp(t, repoFiles)
	typeText(app, "@app")
	m := matchNames(app)
	app.ConsumeKey(loom.KeyEvent{Key: "down"})
	app.ConsumeKey(loom.KeyEvent{Key: "down"})
	app.ConsumeKey(loom.KeyEvent{Key: "down"}) // clamps at the last match
	app.ConsumeKey(loom.KeyEvent{Key: "up"})
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	if want := "@" + m[1] + " "; app.Prompt.Text() != want {
		t.Fatalf("text %q, want %q", app.Prompt.Text(), want)
	}
}

func TestPickerEscAndBackspaceClose(t *testing.T) {
	app, _, sent := newPickerApp(t, repoFiles)
	typeText(app, "x @ap")
	if r := app.ConsumeKey(loom.KeyEvent{Key: "esc"}); r != loom.Handled() || app.QuitRequested {
		t.Fatal("Esc in the picker must close it, not quit")
	}
	if app.picker.isOpen() || app.Prompt.Text() != "x @ap" {
		t.Fatalf("after Esc: open %v text %q", app.picker.isOpen(), app.Prompt.Text())
	}
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	if len(*sent) != 1 {
		t.Fatalf("Enter after Esc should send, sent %v", *sent)
	}

	typeText(app, "@ab")
	for range 2 {
		app.ConsumeKey(loom.KeyEvent{Key: "backspace"})
	}
	if !app.picker.isOpen() || app.picker.query != "" {
		t.Fatalf("backspace to @: open %v query %q", app.picker.isOpen(), app.picker.query)
	}
	app.ConsumeKey(loom.KeyEvent{Key: "backspace"})
	if app.picker.isOpen() || app.Prompt.Text() != "" {
		t.Fatalf("backspace past @: open %v text %q", app.picker.isOpen(), app.Prompt.Text())
	}
}

// Keys keep their meaning with no picker open: up walks history, Enter sends.
func TestClosedPickerKeysUnchanged(t *testing.T) {
	app, _, sent := newPickerApp(t, repoFiles)
	typeText(app, "mail a@b.c")
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	app.ConsumeKey(loom.KeyEvent{Key: "up"})
	if len(*sent) != 1 || app.Prompt.Text() != "mail a@b.c" {
		t.Fatalf("sent %v, history text %q", *sent, app.Prompt.Text())
	}
}

func writeSkill(t *testing.T, dir, name, front string) {
	t.Helper()
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(front), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListSkills(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	writeSkill(t, a, "sprint", "---\nname: sprint\ndescription: |\n  Run the sprint\n  loop.\n---\nbody")
	writeSkill(t, a, "plain", "no frontmatter")
	writeSkill(t, a, "broken", "---\nname: [x\n---\n")
	writeSkill(t, b, "sprint", "---\nname: sprint\ndescription: shadowed\n---\n")
	writeSkill(t, b, "review", "---\nname: review\ndescription: Review a diff\n---\n")
	if err := os.WriteFile(filepath.Join(b, "stray.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := listSkills([]string{a, filepath.Join(a, "missing"), b})
	var flat []string
	for _, it := range got {
		flat = append(flat, it.Name+"="+it.Desc)
	}
	if want := "plain=,review=Review a diff,sprint=Run the sprint loop."; strings.Join(flat, ",") != want {
		t.Fatalf("skills = %q, want %q", strings.Join(flat, ","), want)
	}
}

func TestCommandPickerListsBuiltinsAndSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "sprint", "---\nname: sprint\ndescription: Run the sprint loop\n---\n")
	writeSkill(t, dir, "model", "---\nname: model\ndescription: shadowed by the builtin\n---\n")
	app, l, _ := newPickerApp(t, nil, dir)
	typeText(app, "/")
	l.until(t, func() bool { return !app.pickerLoading() })
	m := matchNames(app)
	if strings.Join(m, ",") != "model,sprint" {
		t.Fatalf("commands = %v, want model,sprint", m)
	}
	if !strings.Contains(screen(app), "/sprint  Run the sprint loop") {
		t.Fatalf("picker lacks skill row:\n%s", screen(app))
	}
	typeText(app, "spr")
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	if app.Prompt.Text() != "/sprint " {
		t.Fatalf("text %q, want /sprint ", app.Prompt.Text())
	}
}

// /model typed by hand still runs: the space closes the picker.
func TestModelCommandTypedByHand(t *testing.T) {
	app, _, sent := newPickerApp(t, nil)
	typeText(app, "/model codex:luna")
	if app.picker.isOpen() {
		t.Fatal("picker still open after a space")
	}
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	if len(*sent) != 0 || !strings.HasPrefix(app.Model, "codex:luna") {
		t.Fatalf("model %q sent %v; want the /model command handled", app.Model, *sent)
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	for _, f := range []string{"a.go", "sub/b.go", "ignored.log", "new.txt"} {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "a.go", "sub/b.go", ".gitignore"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestListRepoFilesHonorsGitignore(t *testing.T) {
	list := func(dir string, limit int) []string {
		files, err := listRepoFiles(t.Context(), dir, limit, nil)
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	dir := gitRepo(t)
	got := strings.Join(list(dir, 100), ",")
	if want := ".gitignore,a.go,new.txt,sub/b.go"; got != want {
		t.Fatalf("files = %q, want %q", got, want)
	}
	if n := len(list(dir, 2)); n != 2 {
		t.Fatalf("cap 2 gave %d files", n)
	}
	plain := t.TempDir()
	for _, f := range []string{"x.txt", ".hidden/y.txt"} {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(plain, f)), 0o755)
		_ = os.WriteFile(filepath.Join(plain, f), nil, 0o644)
	}
	if got := strings.Join(list(plain, 100), ","); got != "x.txt" {
		t.Fatalf("walk fallback = %q, want x.txt", got)
	}
}

// The index builds in a goroutine and lands through post; run with -race.
func TestFileIndexBuiltViaPost(t *testing.T) {
	dir := gitRepo(t)
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	app := NewAppWidget(spec, "m", "s", nil)
	l := &asyncLoop{}
	app.AttachPickers(dir, nil, l.post)
	t.Cleanup(app.Shutdown)
	typeText(app, "@")
	if !strings.Contains(screen(app), spec.Picker.Labels.Indexing) {
		t.Fatalf("no indexing row before the index lands:\n%s", screen(app))
	}
	typeText(app, "b")
	l.until(t, func() bool { return !app.pickerLoading() })
	if m := matchNames(app); strings.Join(m, ",") != "sub/b.go" {
		t.Fatalf("matches after index = %v", m)
	}
	// Cached: a second @ does not rebuild.
	app.ConsumeKey(loom.KeyEvent{Key: "esc"})
	typeText(app, " @")
	if app.pickerLoading() || l.drain() != 0 {
		t.Fatal("second @ rebuilt the index")
	}
}

func TestPickerRenderAt80Cols(t *testing.T) {
	long := "docs/studies/" + strings.Repeat("長い名前", 12) + "/final.md"
	app, _, _ := newPickerApp(t, []string{long, "internal/tui/app.go"})
	typeText(app, "fix @")
	rows := RenderCanvas(app, 80, 20)
	divider := -1
	for i, r := range rows {
		if w := loom.StringWidth(stripANSI(r)); w > 80 {
			t.Errorf("row %d is %d cols wide: %q", i, w, r)
		}
		if strings.HasPrefix(stripANSI(r), "────") {
			divider = i
		}
	}
	if divider < 3 {
		t.Fatalf("no divider:\n%s", strings.Join(rows, "\n"))
	}
	bottom, path := stripANSI(rows[divider-1]), stripANSI(rows[divider-3])
	if !strings.Contains(bottom, "└") || !strings.Contains(path, "…") || !strings.Contains(path, "/final.md") {
		t.Fatalf("box above divider wrong:\n%s", strings.Join(rows, "\n"))
	}
	// Left border sits one column left of the @ ("> fix @" puts @ at col 6).
	if x := strings.Index(bottom, "└"); loom.StringWidth(bottom[:x]) != 5 {
		t.Fatalf("box x = %d, want 5:\n%s", loom.StringWidth(bottom[:x]), strings.Join(rows, "\n"))
	}
	if err := loom.ValidateAnsiBox(strings.Join(rows[divider-4:divider], "\n")); err != nil {
		t.Errorf("box check: %v", err)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i++; i < len(s) && (s[i] < '@' || s[i] > '~' || s[i] == '['); i++ {
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestRankWordStartAboveMidWord(t *testing.T) {
	items := fileItems([]string{"scripts/lint.sh", "internal/tui/app.go", "docs/PrintJob.md", "src/my_int.go", "src/splint.go"})
	var got []string
	for _, it := range rankItems(items, "int", 10) {
		got = append(got, it.Name)
	}
	// Word-start hits first (basename hit breaks the tie), then mid-word ones.
	want := "src/my_int.go,internal/tui/app.go,src/splint.go,scripts/lint.sh,docs/PrintJob.md"
	if strings.Join(got, ",") != want {
		t.Fatalf("int = %v, want %s", got, want)
	}
	for _, tc := range []struct {
		name, q string
		want    bool
	}{
		{"a/internal", "int", true}, {"a_int", "int", true}, {"a-int", "int", true}, {"a.int", "int", true},
		{"fooInt", "int", true}, {"lint", "int", false}, {"FOOint", "int", false}, {"a/in", "int", false},
	} {
		if wordStartHit(tc.name, tc.q) != tc.want {
			t.Errorf("wordStartHit(%q, %q) = %v, want %v", tc.name, tc.q, !tc.want, tc.want)
		}
	}
}

func TestMidWordPickReplacesWholeToken(t *testing.T) {
	app, _, _ := newPickerApp(t, []string{"foo.go", "bar.go"})
	typeText(app, "see @foo.go now")
	// Caret after "@fo", inside the token.
	for range len("o.go now") {
		app.Prompt.MoveLeft()
	}
	app.closePicker()
	app.picker = picker{kind: pickFile, row: 0, col: 4}
	app.syncPicker()
	if app.picker.query != "fo" {
		t.Fatalf("query %q, caret not inside token", app.picker.query)
	}
	app.ConsumeKey(loom.KeyEvent{Key: "tab"})
	if got, want := app.Prompt.Text(), "see @foo.go now"; got != want {
		t.Fatalf("text %q, want %q (tail replaced, no doubled space)", got, want)
	}
	if _, col := app.Prompt.Cursor(); col != len("see @foo.go ") {
		t.Fatalf("caret col %d, want after the space", col)
	}
	typeText(app, "x")
	if got := app.Prompt.Text(); got != "see @foo.go xnow" {
		t.Fatalf("typing after pick: %q", got)
	}

	app, _, _ = newPickerApp(t, []string{"bar.go"})
	typeText(app, "@bxx.go")
	for range len("xx.go") {
		app.Prompt.MoveLeft()
	}
	app.picker = picker{kind: pickFile, row: 0, col: 0}
	app.syncPicker()
	app.ConsumeKey(loom.KeyEvent{Key: "tab"})
	if got := app.Prompt.Text(); got != "@bar.go " {
		t.Fatalf("token at end of line: %q, want %q", got, "@bar.go ")
	}
}

func TestPickPathWithSpacesIsQuoted(t *testing.T) {
	app, _, _ := newPickerApp(t, []string{"my file.go", "plain.go"})
	typeText(app, "@my")
	app.ConsumeKey(loom.KeyEvent{Key: "tab"})
	if got, want := app.Prompt.Text(), `@"my file.go" `; got != want {
		t.Fatalf("text %q, want %q", got, want)
	}
	app, _, _ = newPickerApp(t, []string{"my file.go", "plain.go"})
	typeText(app, "@pl")
	app.ConsumeKey(loom.KeyEvent{Key: "tab"})
	if got := app.Prompt.Text(); got != "@plain.go " {
		t.Fatalf("unquoted: %q", got)
	}
}

func TestWalkSkipsHeavyDirsAndReportsErrors(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.go", "node_modules/x/y.js", "src/node_modules/z.js", "src/b.go"} {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, f), nil, 0o644)
	}
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Picker.SkipDirs) == 0 {
		t.Fatal("spec picker.skip_dirs is empty")
	}
	files, err := listRepoFiles(t.Context(), dir, 100, spec.Picker.SkipDirs)
	if err != nil || strings.Join(files, ",") != "a.go,src/b.go" {
		t.Fatalf("files %v err %v, want a.go,src/b.go", files, err)
	}

	// A missing directory is a read error, not "no matches".
	missing := filepath.Join(dir, "gone")
	if _, err := listRepoFiles(t.Context(), missing, 10, nil); err == nil {
		t.Fatal("missing dir: want a read error")
	}
	app := NewAppWidget(spec, "m", "s", nil)
	l := &asyncLoop{}
	app.AttachPickers(missing, nil, l.post)
	t.Cleanup(app.Shutdown)
	typeText(app, "@")
	l.until(t, func() bool { return !app.pickerLoading() })
	scr := screen(app)
	if strings.Contains(scr, spec.Picker.Labels.NoMatches) || !strings.Contains(scr, "cannot read files") {
		t.Fatalf("want the read error note, got:\n%s", scr)
	}
}

func TestEscClosesOnlyPickerDuringRunningTurn(t *testing.T) {
	started := make(chan struct{})
	app, l := newRunnerApp(t, func(ctx context.Context, _, _ string, _ func(router.Chunk)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	app.AttachPickers(t.TempDir(), nil, l.post)
	t.Cleanup(app.Shutdown)
	app.sources.files = lazyList{state: lazyReady, items: fileItems(repoFiles)}
	typeLine(app, "long task")
	<-started
	typeText(app, "@ap")
	if !app.picker.isOpen() || !app.Busy() {
		t.Fatalf("picker open %v busy %v", app.picker.isOpen(), app.Busy())
	}
	app.ConsumeKey(loom.KeyEvent{Key: "esc"})
	if app.picker.isOpen() || !app.Busy() || app.QuitRequested {
		t.Fatalf("Esc: picker open %v, busy %v, quit %v; want closed picker, turn still running", app.picker.isOpen(), app.Busy(), app.QuitRequested)
	}
	app.ConsumeKey(loom.KeyEvent{Key: "esc"})
	l.drainUntil(t, app)
	if app.Busy() {
		t.Fatal("second Esc should cancel the turn")
	}
}

func TestTabSelectsPickerItemWhileBusy(t *testing.T) {
	app, _, _ := newPickerApp(t, repoFiles)
	typeText(app, "@tui/ap")
	if r := app.ConsumeKey(loom.KeyEvent{Key: "tab"}); r != loom.Handled() {
		t.Fatal("Tab with the picker open must be handled")
	}
	if got := app.Prompt.Text(); got != "@internal/tui/app.go " {
		t.Fatalf("Tab text %q", got)
	}
	if app.picker.isOpen() {
		t.Fatal("picker still open after Tab")
	}
	// With no picker open Tab is left to the container (focus move).
	if r := app.ConsumeKey(loom.KeyEvent{Key: "tab"}); r == loom.Handled() {
		t.Fatal("Tab with no picker must not be consumed")
	}
}
