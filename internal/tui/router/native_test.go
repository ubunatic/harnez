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
