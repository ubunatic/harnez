package subagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInboxEnqueuePersistsAtomicallyAndReloadsFIFO(t *testing.T) {
	dir := t.TempDir()
	inbox, err := OpenInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	first, err := inbox.Enqueue("sender-a", "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := inbox.Enqueue("sender-b", "second")
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) != 2 {
		t.Fatalf("persisted files = %d, err=%v; want 2", len(files), err)
	}
	for _, entry := range mustReadDir(t, dir) {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Fatalf("temporary file remained after persistence: %s", entry.Name())
		}
	}
	reloaded, err := OpenInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Next(); got.ID != first.ID || got.Text != "first" {
		t.Fatalf("first reloaded message = %#v", got)
	}
	if got := reloaded.Next(); got.ID != second.ID || got.Text != "second" {
		t.Fatalf("second reloaded message = %#v", got)
	}
}

func TestInboxReloadSortsByQueuedAt(t *testing.T) {
	dir := t.TempDir()
	older := InboxMessage{ID: "older", Text: "older", State: "queued", QueuedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	newer := InboxMessage{ID: "newer", Text: "newer", State: "queued", QueuedAt: older.QueuedAt.Add(time.Second)}
	writeInboxFixture(t, dir, newer)
	writeInboxFixture(t, dir, older)
	inbox, err := OpenInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := inbox.Next(); got.ID != "older" {
		t.Fatalf("first message = %q, want older", got.ID)
	}
}

func TestInboxEnforceMaxMessageBytes(t *testing.T) {
	inbox, err := OpenInboxWithLimit(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inbox.Enqueue("sender", "four"); err == nil {
		t.Fatal("expected over-limit message rejection")
	}
	if _, err := inbox.Enqueue("sender", "€"); err != nil {
		t.Fatalf("three-byte UTF-8 message rejected: %v", err)
	}
}

func TestInboxMarkDeliveredPersistsState(t *testing.T) {
	dir := t.TempDir()
	inbox, err := OpenInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := inbox.Enqueue("sender", "payload")
	if err != nil {
		t.Fatal(err)
	}
	if err := inbox.Mark(msg.ID, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, msg.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted InboxMessage
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.State != "delivered" || persisted.DeliveredAt.IsZero() {
		t.Fatalf("persisted message = %#v", persisted)
	}
	reloaded, err := OpenInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Next(); got != nil {
		t.Fatalf("delivered message reloaded in queue: %#v", got)
	}
}

func writeInboxFixture(t *testing.T, dir string, msg InboxMessage) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, msg.ID+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustReadDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
