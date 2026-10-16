package router

import (
	"errors"
	"slices"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestNativeArgvPerProvider(t *testing.T) {
	dir := t.TempDir()
	r := New(dir)
	name := func(spec string) string {
		m, err := subagent.ResolveModel(spec)
		if err != nil {
			t.Fatal(err)
		}
		return m.Name
	}
	cases := []struct {
		spec string
		want []string
	}{
		{"claude:sonnet", []string{"claude", "--resume", "c-1", "--model", name("claude:sonnet")}},
		{"codex:luna", []string{"codex", "resume", "c-1", "-m", name("codex:luna"), "-C", dir}},
		{"agy:flash38:med", []string{"agy", "--conversation", "c-1", "--model", name("agy:flash38:med"), "--effort", "medium"}},
	}
	for _, c := range cases {
		m, _ := subagent.ResolveModel(c.spec)
		if err := r.setSession(sessionKey(m), "c-1"); err != nil {
			t.Fatal(err)
		}
		nc, err := r.Native(c.spec)
		if err != nil {
			t.Fatalf("Native(%s): %v", c.spec, err)
		}
		if !slices.Equal(nc.Argv, c.want) || nc.SessionID != "c-1" || nc.Dir != dir || nc.Provider != m.Provider {
			t.Errorf("Native(%s) = %+v; want argv %q", c.spec, nc, c.want)
		}
	}
}

func TestNativeNoSessionAndUnsupported(t *testing.T) {
	r := New(t.TempDir())
	nc, err := r.Native("claude:sonnet")
	if !errors.Is(err, ErrNoSession) || nc.Provider != "claude" || nc.Argv != nil {
		t.Fatalf("Native without session = %+v, %v; want ErrNoSession", nc, err)
	}
	m, err := subagent.ResolveModel("pi:some-model")
	if err != nil {
		t.Skipf("no pi model: %v", err)
	}
	_ = r.setSession(sessionKey(m), "p-1")
	nc, err = r.Native("pi:some-model")
	if !errors.Is(err, subagent.ErrInteractiveUnsupported) || nc.Provider != "pi" {
		t.Fatalf("Native(pi) = %+v, %v; want ErrInteractiveUnsupported", nc, err)
	}
}

func TestInvalidStoredSessionIDIsDropped(t *testing.T) {
	for _, bad := range []string{"-rf", "--help", "a b", "x\n--y", "id\x1b[2J", "a\tb", "ü"} {
		st, err := NewStore(t.TempDir(), "t")
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Set("claude:"+mustName(t, "claude:sonnet"), bad); err != nil {
			t.Fatal(err)
		}
		r := New(t.TempDir())
		r.Store = st
		if _, err := r.Load(); err != nil {
			t.Fatal(err)
		}
		nc, err := r.Native("claude:sonnet")
		if !errors.Is(err, ErrNoSession) || !errors.Is(err, ErrInvalidSession) || nc.Argv != nil {
			t.Errorf("Native with id %q = %+v, %v", bad, nc, err)
		}
		if got, _, _ := st.Load(); len(got) != 0 {
			t.Errorf("id %q kept in store: %v", bad, got)
		}
	}
	for _, good := range []string{"c-1", "0199a5e2-7c1d-7f00-9a6b-1a2b3c4d5e6f", "conv_ab.cd:1"} {
		r := New(t.TempDir())
		_ = r.setSession("claude:"+mustName(t, "claude:sonnet"), good)
		if nc, err := r.Native("claude:sonnet"); err != nil || nc.SessionID != good {
			t.Errorf("Native with id %q = %+v, %v", good, nc, err)
		}
	}
}

func TestTurnDropsInvalidStoredSessionID(t *testing.T) {
	r := New(t.TempDir())
	key := "claude:" + mustName(t, "claude:sonnet")
	_ = r.setSession(key, "--dangerous")
	if id, dropped := r.session(key); id != "" || !errors.Is(dropped, ErrInvalidSession) {
		t.Fatalf("session = %q, %v", id, dropped)
	}
	if id := r.Session("claude:sonnet"); id != "" {
		t.Errorf("Session after drop = %q", id)
	}
}

func mustName(t *testing.T, spec string) string {
	t.Helper()
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		t.Fatal(err)
	}
	return m.Name
}
