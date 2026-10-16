package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func storedRouter(t *testing.T, dir, id string, d subagent.Driver) *Router {
	t.Helper()
	st, err := NewStore(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	r := &Router{Store: st, Driver: func(subagent.Model, string) subagent.Driver { return d }}
	if err := r.Load(); err != nil {
		t.Fatal(err)
	}
	return r
}

func kinds(cs []Chunk) string {
	var out []string
	for _, c := range cs {
		out = append(out, string(c.Kind)+":"+c.Text)
	}
	return strings.Join(out, "|")
}

// A restarted router (harnez tui --session X) resumes the backend session
// stored by the previous run instead of starting a new one.
func TestStorePersistsSessionsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	var calls []string
	first := storedRouter(t, dir, "tui-a", batchDriver{calls: &calls})
	if got := collectAll(t, first, "claude:sonnet", "a"); got[0] != (Chunk{ChunkSession, SessionNew}) {
		t.Fatalf("first turn chunks = %s", kinds(got))
	}

	second := storedRouter(t, dir, "tui-a", batchDriver{calls: &calls})
	if id := second.Session("claude:sonnet"); id != "s-claude" {
		t.Fatalf("restarted Session = %q, want s-claude", id)
	}
	got := collectAll(t, second, "claude:sonnet", "b")
	if got[0] != (Chunk{ChunkSession, SessionResumed}) {
		t.Fatalf("resumed turn chunks = %s", kinds(got))
	}
	if s := strings.Join(calls, "|"); s != "run claude a|resume s-claude b" {
		t.Fatalf("calls = %q", s)
	}
	// Another tui session id does not see these backend sessions.
	if id := storedRouter(t, dir, "tui-b", batchDriver{calls: &calls}).Session("claude:sonnet"); id != "" {
		t.Fatalf("tui-b sees session %q of tui-a", id)
	}
}

// A stored id the provider no longer knows starts a fresh session with a
// note, and the store then holds the new id.
func TestStoreStaleSessionStartsFreshAndUpdatesStore(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir, "tui-stale")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set("claude:sonnet", "gone"); err != nil {
		t.Fatal(err)
	}
	var calls []string
	r := storedRouter(t, dir, "tui-stale", batchDriver{calls: &calls, resumeErr: errors.New("No conversation found with session ID: gone")})
	got := collectAll(t, r, "claude:sonnet", "p")
	want := "session:resumed|activity:could not resume session gone (No conversation found with session ID: gone); starting a new session|session:new|text:one|text:two"
	if kinds(got) != want {
		t.Fatalf("chunks:\n%s\nwant:\n%s", kinds(got), want)
	}
	if s := strings.Join(calls, "|"); s != "resume gone p|run claude p" {
		t.Fatalf("calls = %q", s)
	}
	stored, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored["claude:sonnet"] != "s-claude" {
		t.Fatalf("store after fallback = %v, want s-claude", stored)
	}
}

// Writers on the same tui session id merge under the lock; different ids
// use different files.
func TestStoreConcurrentWritersKeepAllKeys(t *testing.T) {
	dir := t.TempDir()
	const n = 20
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(2)
		go func() {
			defer wg.Done()
			st, err := NewStore(dir, "shared") // a separate handle, like a second tui process
			if err != nil {
				t.Error(err)
				return
			}
			if err := st.Set(fmt.Sprintf("p:m%d", i), fmt.Sprintf("id%d", i)); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			st, err := NewStore(dir, "other")
			if err != nil {
				t.Error(err)
				return
			}
			if err := st.Set("p:m0", fmt.Sprintf("other%d", i)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	st, _ := NewStore(dir, "shared")
	got, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("shared store has %d keys, want %d: %v", len(got), n, got)
	}
	for i := range n {
		if got[fmt.Sprintf("p:m%d", i)] != fmt.Sprintf("id%d", i) {
			t.Fatalf("key p:m%d = %q", i, got[fmt.Sprintf("p:m%d", i)])
		}
	}
}

func TestNewStoreRejectsPathLikeIDs(t *testing.T) {
	for _, id := range []string{"", ".", "..", "a/b", "../x"} {
		if _, err := NewStore(t.TempDir(), id); err == nil {
			t.Errorf("NewStore(%q) accepted a path-like id", id)
		}
	}
}

// A cancelled turn keeps the stored id: cancellation is not a stale session.
func TestStoreKeepsSessionOnCancel(t *testing.T) {
	dir := t.TempDir()
	st, _ := NewStore(dir, "tui-c")
	if err := st.Set("claude:sonnet", "keep"); err != nil {
		t.Fatal(err)
	}
	var calls []string
	r := storedRouter(t, dir, "tui-c", batchDriver{calls: &calls, resumeErr: context.Canceled})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Turn(ctx, "claude:sonnet", "p", func(Chunk) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Turn err = %v", err)
	}
	if got, _ := st.Load(); got["claude:sonnet"] != "keep" {
		t.Fatalf("store after cancel = %v", got)
	}
}
