package main

import (
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

// A detached Jules start must not report success before the worker has either
// created the Jules session (ProviderSessionID recorded) or ended without one.
func newAwaitJulesStore(t *testing.T) (*subagent.FileSessionStore, *subagent.Session) {
	t.Helper()
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "jules-detach-1", Name: "jules-detach", Provider: "jules", Status: "running", CreatedAt: time.Now(), LastActiveAt: time.Now()}
	if err := store.Create(sess); err != nil {
		t.Fatal(err)
	}
	return store, sess
}

func TestAwaitJulesCreateReturnsWhenSessionIsCreated(t *testing.T) {
	store, sess := newAwaitJulesStore(t)
	go func() {
		time.Sleep(60 * time.Millisecond)
		current, err := store.Get(sess.ID)
		if err != nil {
			return
		}
		current.ProviderSessionID = "jules-123"
		_ = store.Save(current)
	}()
	exited := make(chan error) // the worker is still running
	got, err := awaitJulesCreate(store, sess.ID, exited)
	if err != nil {
		t.Fatalf("awaitJulesCreate() error = %v, want nil", err)
	}
	if got.ProviderSessionID != "jules-123" {
		t.Fatalf("returned ProviderSessionID = %q, want jules-123", got.ProviderSessionID)
	}
}

func TestAwaitJulesCreateReportsWorkerFailure(t *testing.T) {
	store, sess := newAwaitJulesStore(t)
	sess.Status = "failed"
	sess.LastError = "create Jules session: Jules API returned HTTP 400: Precondition check failed"
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	exited <- nil
	_, err := awaitJulesCreate(store, sess.ID, exited)
	if err == nil {
		t.Fatal("awaitJulesCreate() error = nil, want the create failure")
	}
	if !strings.Contains(err.Error(), "Precondition check failed") {
		t.Fatalf("error %q does not name the Jules create error", err)
	}
}

func TestAwaitJulesCreateFailsWhenWorkerExitsWithoutSession(t *testing.T) {
	store, sess := newAwaitJulesStore(t)
	exited := make(chan error, 1)
	exited <- nil
	if _, err := awaitJulesCreate(store, sess.ID, exited); err == nil {
		t.Fatal("awaitJulesCreate() error = nil, want an error for a worker that exited without a Jules session")
	}
}
