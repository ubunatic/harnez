package tui

import (
	"context"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/tui/router"
	"ubunatic.com/loom"
)

// nativeApp returns an app on model whose router has stored backend session
// id for model ("" stores none), plus the argv list its fake runner records.
func nativeApp(t *testing.T, model, id string) (*AppWidget, *router.Router, *[][]string) {
	t.Helper()
	spec, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	m, err := subagent.ResolveModel(model)
	if err != nil {
		t.Fatal(err)
	}
	st, err := router.NewStore(t.TempDir(), "t")
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		// The router keys backend sessions by provider:name.
		if err := st.Set(m.Provider+":"+m.Name, id); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	rt := router.New(dir)
	rt.Store = st
	if _, err := rt.Load(); err != nil {
		t.Fatal(err)
	}
	app := NewAppWidget(spec, model, "s", nil)
	var runs [][]string
	suspend := func(f func() error) error { return f() }
	app.AttachNative(suspend, rt.Native, func(nc router.NativeCommand) error {
		if nc.Dir != dir {
			t.Errorf("native dir = %q; want %q", nc.Dir, dir)
		}
		runs = append(runs, nc.Argv)
		return nil
	})
	return app, rt, &runs
}

func ctrlO(app *AppWidget) { app.ConsumeKey(loom.KeyEvent{Key: "ctrl-o"}) }

func TestOpenNativeResumesSessionPerProvider(t *testing.T) {
	cases := []struct{ model, provider, want string }{
		{"claude:sonnet:low", "claude", "claude --resume c-id --model "},
		{"codex:luna", "codex", "codex resume c-id -m "},
		{"agy:flash38:med", "agy", "agy --conversation c-id --model "},
	}
	for _, c := range cases {
		t.Run(c.model, func(t *testing.T) {
			app, rt, runs := nativeApp(t, c.model, "c-id")
			app.Backend = router.SessionResumed
			header := app.formatHeader()
			ctrlO(app)
			if len(*runs) != 1 || !strings.HasPrefix(strings.Join((*runs)[0], " "), c.want) {
				t.Fatalf("runs = %q; want prefix %q", *runs, c.want)
			}
			if got := rt.Session(c.model); got != "c-id" {
				t.Errorf("session after return = %q; want c-id", got)
			}
			if got := app.formatHeader(); got != header {
				t.Errorf("header changed: %q -> %q", header, got)
			}
			if lt := lastTurn(app); lt.Role != RoleNote || lt.Content != "returned from "+c.provider {
				t.Errorf("last turn = %+v; want returned note", lt)
			}
		})
	}
}

func TestOpenNativeNotes(t *testing.T) {
	app, _, runs := nativeApp(t, "claude:sonnet:low", "")
	ctrlO(app)
	if lt := lastTurn(app); lt.Role != RoleNote || lt.Content != "no claude session yet; send a prompt first" {
		t.Errorf("no session: last turn = %+v", lt)
	}
	if len(*runs) != 0 {
		t.Errorf("no session ran %q", *runs)
	}

	app, _, runs = nativeApp(t, "pi:some-model", "p-id")
	ctrlO(app)
	if lt := lastTurn(app); lt.Role != RoleNote || lt.Content != "native session not supported for pi" {
		t.Errorf("pi: last turn = %+v", lt)
	}
	if len(*runs) != 0 {
		t.Errorf("pi ran %q", *runs)
	}

	app = NewAppWidget(app.Spec, "claude:sonnet:low", "s", nil)
	ctrlO(app)
	if lt := lastTurn(app); lt.Role != RoleError || lt.Content != app.Spec.Native.Labels.Unavailable {
		t.Errorf("unattached: last turn = %+v", lt)
	}
}

func TestOpenNativeDisallowedWhileBusy(t *testing.T) {
	app, _, runs := nativeApp(t, "claude:sonnet:low", "c-id")
	l := newLoop()
	release := make(chan struct{})
	app.AttachRunner(func(context.Context, string, string, func(router.Chunk)) error {
		<-release
		return nil
	}, l.post, router.ResolveSpec)
	typeLine(app, "hi")
	if !app.Busy() {
		t.Fatal("turn not running")
	}
	ctrlO(app)
	if lt := lastTurn(app); lt.Role != RoleNote || lt.Content != app.Spec.Native.Labels.Busy {
		t.Errorf("busy: last turn = %+v", lt)
	}
	if len(*runs) != 0 {
		t.Fatalf("busy ran %q", *runs)
	}
	close(release)
	l.drainUntil(t, app)
	ctrlO(app)
	if len(*runs) != 1 || !slices.Contains((*runs)[0], "c-id") {
		t.Errorf("after the turn: runs = %q; want one resume of c-id", *runs)
	}
}
