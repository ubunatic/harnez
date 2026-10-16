package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/tui/router"
	"ubunatic.com/loom"
)

// newModelApp returns an idle app on claude:sonnet:low whose resolve calls
// are counted, so tests can prove a pick runs the /model validation path.
func newModelApp(t *testing.T, run TurnFunc) (*AppWidget, *loop, *[]string) {
	t.Helper()
	app, l := newRunnerApp(t, run)
	resolved := &[]string{}
	app.resolve = func(s string) (string, error) {
		*resolved = append(*resolved, s)
		return router.ResolveSpec(s)
	}
	return app, l, resolved
}

func TestModelPickerOpensViaKeyAndBareModel(t *testing.T) {
	app, _, _ := newModelApp(t, nil)
	for _, k := range app.Spec.Keybindings.OpenModels {
		name := strings.ReplaceAll(k, "+", "-")
		app.ConsumeKey(loom.KeyEvent{Key: name})
		if app.picker.kind != pickModel {
			t.Fatalf("%s did not open the model picker", k)
		}
		if len(app.picker.matches) == 0 || app.picker.matches[app.picker.sel].Name != app.Model {
			t.Fatalf("%s: selection is not the active model: %v", k, matchNames(app))
		}
		app.ConsumeKey(loom.KeyEvent{Key: name})
		if app.picker.isOpen() {
			t.Fatalf("%s again should close the picker", k)
		}
	}
	typeLine(app, "/model")
	if app.picker.kind != pickModel || app.Prompt.Text() != "" {
		t.Fatalf("bare /model: kind %v prompt %q", app.picker.kind, app.Prompt.Text())
	}
	if n := len(app.Chat.Turns()); n != 0 {
		t.Fatalf("bare /model added %d chat turns", n)
	}
}

func TestModelPickerListsSpecModelsGroupedByProvider(t *testing.T) {
	app, _, _ := newModelApp(t, nil)
	app.openModelPicker()
	got := matchNames(app)
	want := map[string]bool{}
	for _, e := range app.modelEntries() {
		want[e.Spec] = true
	}
	if len(got) != len(want) || !want["codex:luna:med"] || !want["claude:sonnet:high"] {
		t.Fatalf("matches %v, want every spec/agent.yaml tier entry", got)
	}
	seen := map[string]bool{}
	prev := ""
	for _, s := range got {
		if p := providerOf(s); p != prev {
			if seen[p] {
				t.Fatalf("provider %s split into two groups: %v", p, got)
			}
			seen[p], prev = true, p
		}
	}
}

func TestModelPickerFilterSelectSwitchesLikeModelCommand(t *testing.T) {
	var ran string
	app, l, resolved := newModelApp(t, func(_ context.Context, model, _ string, _ func(router.Chunk)) error {
		ran = model
		return nil
	})
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-t"})
	typeText(app, "astrx")
	app.ConsumeKey(loom.KeyEvent{Key: "backspace"})
	app.ConsumeKey(loom.KeyEvent{Key: "backspace"})
	typeText(app, "ra")
	if app.picker.query != "astra" || app.Prompt.Text() != "" {
		t.Fatalf("query %q prompt %q; typing must filter, not edit the prompt", app.picker.query, app.Prompt.Text())
	}
	got := matchNames(app)
	if len(got) == 0 {
		t.Fatal("no matches for astra")
	}
	for _, s := range got {
		if !strings.Contains(s, "astra") {
			t.Fatalf("filter kept %q: %v", s, got)
		}
	}
	app.ConsumeKey(loom.KeyEvent{Key: "down"})
	want := got[1]
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	if app.picker.isOpen() || app.Model != want {
		t.Fatalf("open %v model %q, want %q", app.picker.isOpen(), app.Model, want)
	}
	if len(*resolved) != 1 || (*resolved)[0] != want {
		t.Fatalf("resolve calls %v; the pick must run the /model validation", *resolved)
	}
	note := lastTurn(app)
	if note.Role != RoleTool || note.Content != strings.ReplaceAll(app.Spec.ModelPicker.Labels.Switched, "{{ .Model }}", want) {
		t.Fatalf("note %#v", note)
	}
	if !strings.Contains(app.formatHeader(), want) {
		t.Fatalf("header %q lacks %s", app.formatHeader(), want)
	}
	// Same chat result as typing the command by hand.
	typeLine(app, "/model "+want)
	if lastTurn(app).Content != note.Content || lastTurn(app).Role != note.Role {
		t.Fatalf("/model note %#v differs from picker note %#v", lastTurn(app), note)
	}
	typeLine(app, "hi")
	l.drainUntil(t, app)
	if ran != want {
		t.Fatalf("turn ran on %q, want %q", ran, want)
	}
}

func TestModelPickerEscClosesWithoutSwitching(t *testing.T) {
	app, _, resolved := newModelApp(t, nil)
	app.ConsumeKey(loom.KeyEvent{Key: "alt-m"})
	typeText(app, "opus")
	app.ConsumePaste(loom.PasteEvent{Text: "x\ny"})
	if app.picker.query != "opusx y" || app.Prompt.Text() != "" {
		t.Fatalf("paste: query %q prompt %q", app.picker.query, app.Prompt.Text())
	}
	app.ConsumeKey(loom.KeyEvent{Key: "esc"})
	if app.picker.isOpen() || app.QuitRequested || app.Model != "claude:sonnet:low" || len(*resolved) != 0 {
		t.Fatalf("esc: open %v quit %v model %q resolves %v", app.picker.isOpen(), app.QuitRequested, app.Model, *resolved)
	}
	// Enter with no matches closes and keeps the model.
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-t"})
	typeText(app, "zzzzqqq")
	if len(app.picker.matches) != 0 {
		t.Fatalf("matches %v", matchNames(app))
	}
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	if app.picker.isOpen() || app.Model != "claude:sonnet:low" || len(app.Chat.Turns()) != 0 {
		t.Fatalf("empty enter: open %v model %q turns %d", app.picker.isOpen(), app.Model, len(app.Chat.Turns()))
	}
}

func TestModelPickerSwitchDuringTurnAppliesNext(t *testing.T) {
	started := make(chan struct{})
	var models []string
	app, l, _ := newModelApp(t, func(ctx context.Context, model, prompt string, _ func(router.Chunk)) error {
		models = append(models, model)
		if prompt == "long" {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})
	typeLine(app, "long")
	<-started
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-t"})
	typeText(app, "haiku")
	app.ConsumeKey(loom.KeyEvent{Key: "enter"})
	if !app.Busy() || !strings.HasPrefix(app.Model, "claude:haiku") {
		t.Fatalf("busy %v model %q", app.Busy(), app.Model)
	}
	want := strings.ReplaceAll(app.Spec.ModelPicker.Labels.SwitchedBusy, "{{ .Model }}", app.Model)
	if lastTurn(app).Content != want {
		t.Fatalf("note %q, want %q", lastTurn(app).Content, want)
	}
	// Esc in the open picker closes only the picker; a second Esc cancels.
	app.ConsumeKey(loom.KeyEvent{Key: "ctrl-t"})
	app.ConsumeKey(loom.KeyEvent{Key: "esc"})
	if !app.Busy() || app.picker.isOpen() {
		t.Fatalf("esc in picker: busy %v open %v", app.Busy(), app.picker.isOpen())
	}
	app.ConsumeKey(loom.KeyEvent{Key: "esc"})
	l.drainUntil(t, app)
	typeLine(app, "next")
	l.drainUntil(t, app)
	if len(models) != 2 || models[0] != "claude:sonnet:low" || models[1] != app.Model {
		t.Fatalf("turn models %v, want the old model then %s", models, app.Model)
	}
}

func TestModelPickerMarkersAndRenderAt80Cols(t *testing.T) {
	app, _, _ := newModelApp(t, nil)
	app.AttachSessions(func(spec string) string {
		if strings.HasPrefix(spec, "claude:sonnet:") {
			return "sess-1"
		}
		return ""
	})
	app.openModelPicker()
	rows := RenderCanvas(app, 80, 24)
	mp := app.Spec.ModelPicker
	top, bottom := -1, -1
	found := map[string]string{}
	for i, r := range rows {
		plain := stripANSI(r)
		if w := loom.StringWidth(plain); w > 80 {
			t.Errorf("row %d is %d cols: %q", i, w, plain)
		}
		if strings.Contains(plain, "┌") && strings.Contains(plain, mp.Title) {
			top = i
		}
		if top >= 0 && bottom < 0 && strings.Contains(plain, "└") {
			bottom = i
		}
		for _, s := range []string{"claude:sonnet:low", "claude:sonnet:med", "claude:opus:low"} {
			if strings.Contains(plain, s+" ") {
				found[s] = plain
			}
		}
	}
	if top < 0 || bottom <= top {
		t.Fatalf("no overlay box:\n%s", strings.Join(rows, "\n"))
	}
	if err := loom.ValidateAnsiBox(strings.Join(rows[top:bottom+1], "\n")); err != nil {
		t.Errorf("box check: %v\n%s", err, strings.Join(rows[top:bottom+1], "\n"))
	}
	act, ses := mp.ActiveMarker, mp.SessionMarker
	marks := func(s string) (bool, bool) {
		row := found[s]
		head := row[:strings.Index(row, s)]
		return strings.Contains(head, act), strings.Contains(head, ses)
	}
	for s, want := range map[string][2]bool{
		"claude:sonnet:low": {true, true},
		"claude:sonnet:med": {false, true},
		"claude:opus:low":   {false, false},
	} {
		if found[s] == "" {
			t.Fatalf("%s not rendered:\n%s", s, strings.Join(rows, "\n"))
		}
		if a, ss := marks(s); a != want[0] || ss != want[1] {
			t.Errorf("%s: active %v session %v, want %v: %q", s, a, ss, want, found[s])
		}
	}
	heading := false
	for _, r := range rows[top+1 : bottom] {
		if strings.TrimSpace(strings.Trim(stripANSI(r), "│")) == "claude" {
			heading = true
		}
	}
	if !heading {
		t.Errorf("no claude provider heading:\n%s", strings.Join(rows[top:bottom+1], "\n"))
	}
}

// TestKeybindingsDoNotConflict checks every spec binding: open_models shares
// no key with any other action and avoids terminal aliases of Enter/Tab/
// Backspace/Esc; the actions live while the prompt has focus are pairwise
// disjoint (quit/cancel_turn overlap on purpose: cancel wins while busy).
func TestKeybindingsDoNotConflict(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	norm := func(k string) string { return strings.ReplaceAll(strings.ToLower(k), "+", "-") }
	all := map[string][]string{}
	v := reflect.ValueOf(spec.Keybindings)
	for i := 0; i < v.NumField(); i++ {
		tag := v.Type().Field(i).Tag.Get("yaml")
		keys := v.Field(i).Interface().([]string)
		if len(keys) == 0 && tag != "cancel_turn" {
			t.Errorf("%s has no keys", tag)
		}
		all[tag] = keys
	}
	for _, k := range all["open_models"] {
		switch norm(k) {
		case "ctrl-m", "ctrl-i", "ctrl-h", "ctrl-[", "ctrl-j", "enter", "tab", "esc", "backspace":
			t.Errorf("open_models key %q is a terminal alias of another key", k)
		}
		for action, keys := range all {
			if action != "open_models" && bound(norm(k), keys) {
				t.Errorf("open_models key %q is also bound to %s", k, action)
			}
		}
	}
	prompt := []string{"send_prompt", "newline", "quit", "scroll_up", "scroll_down", "open_editor", "open_native",
		"toggle_jobs", "open_models", "line_start", "line_end", "delete_to_start", "delete_to_end", "delete_word",
		"word_left", "word_right"}
	owner := map[string]string{}
	for _, action := range prompt {
		for _, k := range all[action] {
			n := norm(k)
			if prev, ok := owner[n]; ok {
				t.Errorf("%q bound to both %s and %s", k, prev, action)
			}
			owner[n] = action
		}
	}
	// Keys that move the prompt cursor are plain names in promptKey.
	for _, n := range []string{"up", "down", "left", "right", "backspace"} {
		if a, ok := owner[n]; ok {
			t.Errorf("%s is bound to %s but drives the prompt cursor", n, a)
		}
	}
}
