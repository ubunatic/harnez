package subagent

import (
	"strings"
	"sync"
	"testing"
)

func TestSessionQueueSurvivesWritersAndAcknowledgesSnapshot(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &Session{ID: "worker", Name: "worker", Status: "running"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"FIRST", "SECOND"} {
		if _, err := store.QueueMessage("worker", "", "host", message); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	prompt := PrependQueuedMessages(snapshot.PendingMessages, "continue")
	if strings.Count(prompt, "FIRST") != 1 || strings.Count(prompt, "SECOND") != 1 || strings.Index(prompt, "FIRST") > strings.Index(prompt, "SECOND") || !strings.Contains(prompt, "host") || !strings.HasSuffix(prompt, "continue") {
		t.Fatalf("prompt = %q", prompt)
	}
	// A writer with an old snapshot must preserve messages sent while it ran.
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		if _, err := store.QueueMessage("worker", "", "peer", "LATER"); err != nil {
			t.Error(err)
		}
	})
	sess.Turn = 1
	if err := store.SaveQueuedTurn(sess, snapshot.PendingMessages); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	got, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Turn != 1 || len(got.PendingMessages) != 1 || got.PendingMessages[0].Text != "LATER" {
		t.Fatalf("session = %#v", got)
	}
	if err := store.Save(snapshot); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Get(sess.ID)
	if len(got.PendingMessages) != 1 {
		t.Fatal("stale save resurrected consumed messages")
	}
}

func TestSessionQueueResolution(t *testing.T) {
	store, _ := NewSessionStore(t.TempDir())
	for _, sess := range []*Session{
		{ID: "a", Name: "duplicate", Status: "running", WorkingDir: "/a"},
		{ID: "b", Name: "duplicate", Status: "running", WorkingDir: "/b"},
		{ID: "done", Name: "done", Status: "completed"},
		{ID: "interactive", Name: "interactive", Status: "active", HarnessType: "interactive"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, want string }{
		{"missing", "unknown"}, {"duplicate", "ambiguous"}, {"done", "completed"}, {"interactive", "exec"},
	} {
		_, err := store.QueueMessage(tc.name, "", "host", "hello")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if tc.name == "duplicate" && (!strings.Contains(err.Error(), "id=a") || !strings.Contains(err.Error(), "id=b")) {
			t.Fatal(err)
		}
	}
	matched, err := store.QueueMessage("duplicate", "/b", "host", "scoped")
	if err != nil || matched.ID != "b" {
		t.Fatalf("directory filter: %#v, %v", matched, err)
	}
	otherStore, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherStore.QueueMessage("duplicate", "", "host", "wrong store"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("store filter: %v", err)
	}
	if err := store.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("b"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.QueueMessage("duplicate", "", "host", "hello"); err == nil || !strings.Contains(err.Error(), "deleted") {
		t.Fatalf("deleted: %v", err)
	}
}
