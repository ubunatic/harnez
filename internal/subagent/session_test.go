package subagent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSessionStore_Save(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	sess := &Session{
		ID:               "test-session-1",
		Name:             "test-agent",
		Provider:         "codex",
		Model:            "gpt-5.6-luna",
		Tier:             "low",
		WorkingDir:       "/home/user/project",
		ParentSessionID:  "parent-1",
		CallerPID:        1234,
		HarnessType:      "codex-cli",
		Status:           "running",
		TokensCumulative: 5000,
		TokensTurn:       500,
		CachedTokens:     100,
		CreatedAt:        time.Now(),
		LastActiveAt:     time.Now(),
	}

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	path := filepath.Join(dir, "test-session-1.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Session file not created: %v", err)
	}
}

func TestFileSessionStore_SaveNoID(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	sess := &Session{
		Name: "test-agent",
	}

	err = store.Save(sess)
	if err == nil {
		t.Fatal("Save should fail when session has no ID")
	}
}

func TestFileSessionStore_Get(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	now := time.Now()
	sess := &Session{
		ID:               "test-session-1",
		Name:             "test-agent",
		Provider:         "codex",
		Model:            "gpt-5.6-luna",
		Tier:             "low",
		WorkingDir:       "/home/user/project",
		ParentSessionID:  "parent-1",
		CallerPID:        1234,
		HarnessType:      "codex-cli",
		Status:           "running",
		TokensCumulative: 5000,
		TokensTurn:       500,
		CachedTokens:     100,
		CreatedAt:        now,
		LastActiveAt:     now,
	}

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := store.Get("test-session-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != sess.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, sess.ID)
	}
	if retrieved.Name != sess.Name {
		t.Errorf("Name mismatch: got %q, want %q", retrieved.Name, sess.Name)
	}
	if retrieved.Provider != sess.Provider {
		t.Errorf("Provider mismatch: got %q, want %q", retrieved.Provider, sess.Provider)
	}
	if retrieved.TokensCumulative != sess.TokensCumulative {
		t.Errorf("TokensCumulative mismatch: got %d, want %d", retrieved.TokensCumulative, sess.TokensCumulative)
	}
}

func TestFileSessionStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Fatal("Get should fail for nonexistent session")
	}
}

func TestFileSessionStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	sess1 := &Session{
		ID:              "sess-1",
		Name:            "agent-1",
		Provider:        "codex",
		ParentSessionID: "parent-1",
		Status:          "running",
	}
	sess2 := &Session{
		ID:              "sess-2",
		Name:            "agent-2",
		Provider:        "claude",
		ParentSessionID: "parent-1",
		Status:          "idle",
	}
	sess3 := &Session{
		ID:              "sess-3",
		Name:            "agent-3",
		Provider:        "codex",
		ParentSessionID: "parent-2",
		Status:          "running",
	}

	for _, s := range []*Session{sess1, sess2, sess3} {
		if err := store.Save(s); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	all, err := store.List("", true)
	if err != nil {
		t.Fatalf("List all failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List all returned %d sessions, want 3", len(all))
	}

	filtered, err := store.List("parent-1", false)
	if err != nil {
		t.Fatalf("List filtered failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("List parent-1 returned %d sessions, want 2", len(filtered))
	}
	for _, s := range filtered {
		if s.ParentSessionID != "parent-1" {
			t.Errorf("Unexpected parent session ID: %q", s.ParentSessionID)
		}
	}
}

func TestFileSessionStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	sess := &Session{
		ID:       "test-session-1",
		Name:     "test-agent",
		Provider: "codex",
	}

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Delete("test-session-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get("test-session-1")
	if err == nil {
		t.Fatal("Session should not exist after deletion")
	}
}

func TestFileSessionStore_DeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	err = store.Delete("nonexistent")
	if err == nil {
		t.Fatal("Delete should fail for nonexistent session")
	}
}

func TestFileSessionStore_FindByNameAndProviderID(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &Session{ID: "registry-id", ProviderSessionID: "provider-id", Name: "calm-otter"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	got, err := store.Find("calm-otter")
	if err != nil || got.ID != sess.ID {
		t.Fatalf("Find by name = %#v, %v", got, err)
	}
	if got.ProviderID() != "provider-id" {
		t.Fatalf("ProviderID = %q, want provider-id", got.ProviderID())
	}
	legacy := &Session{ID: "legacy-id"}
	if legacy.ProviderID() != legacy.ID {
		t.Fatalf("legacy ProviderID = %q, want %q", legacy.ProviderID(), legacy.ID)
	}
}

func TestFileSessionStore_CreateRejectsNameIDCollisions(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&Session{ID: "registry-id", Name: "calm-otter"}); err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*Session{
		{ID: "other-id", Name: "calm-otter"},
		{ID: "other-id", Name: "registry-id"},
		{ID: "calm-otter", Name: "other-name"},
	} {
		if err := store.Create(sess); !errors.Is(err, ErrSessionNameInUse) {
			t.Errorf("Create(%#v) error = %v, want ErrSessionNameInUse", sess, err)
		}
	}
}

func TestFileSessionStore_CreateConcurrentNameReservation(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	start := make(chan struct{})
	for _, id := range []string{"one", "two"} {
		go func(id string) {
			<-start
			results <- store.Create(&Session{ID: id, Name: "swift-falcon"})
		}(id)
	}
	close(start)
	successes, collisions := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, ErrSessionNameInUse) {
			collisions++
		} else {
			t.Fatalf("unexpected Create error: %v", err)
		}
	}
	if successes != 1 || collisions != 1 {
		t.Fatalf("successes=%d collisions=%d", successes, collisions)
	}
}

func TestCanManage_RootCaller(t *testing.T) {
	target := &Session{
		ID:              "sess-1",
		ParentSessionID: "parent-1",
	}
	if !CanManage("", target) {
		t.Fatal("Root caller (empty string) should be able to manage any session")
	}
}

func TestCanManage_DirectChild(t *testing.T) {
	target := &Session{
		ID:              "sess-1",
		ParentSessionID: "parent-1",
	}
	if !CanManage("parent-1", target) {
		t.Fatal("Parent should be able to manage direct child")
	}
}

func TestCanManage_Self(t *testing.T) {
	target := &Session{
		ID:              "sess-1",
		ParentSessionID: "parent-1",
	}
	if !CanManage("sess-1", target) {
		t.Fatal("Session should be able to manage itself")
	}
}

func TestCanManage_SelfByName(t *testing.T) {
	target := &Session{
		ID:              "sess-1",
		Name:            "swift-falcon",
		ParentSessionID: "parent-1",
	}
	if !CanManage("swift-falcon", target) {
		t.Fatal("Session should be able to manage itself by name")
	}
}

func TestCanManage_NoAccess(t *testing.T) {
	target := &Session{
		ID:              "sess-1",
		ParentSessionID: "parent-1",
	}
	if CanManage("unrelated", target) {
		t.Fatal("Unrelated session should not be able to manage target")
	}
}

func TestCanManage_Ancestor(t *testing.T) {
	target := &Session{
		ID:              "sess-1",
		ParentSessionID: "parent-1",
	}
	if !CanManage("grandparent-1", target) && target.ParentSessionID != "parent-1" {
		t.Fatal("Check implemented based on ParentSessionID not recursive ancestry")
	}
	if CanManage("grandparent-1", target) {
		t.Fatal("Ancestor (not direct parent) should not be able to manage target")
	}
}

func TestShouldCompact_BelowThreshold(t *testing.T) {
	if ShouldCompact(99999) {
		t.Fatal("ShouldCompact should return false for 99999 tokens")
	}
}

func TestShouldCompact_AtThreshold(t *testing.T) {
	if !ShouldCompact(100000) {
		t.Fatal("ShouldCompact should return true for 100000 tokens")
	}
}

func TestShouldCompact_AboveThreshold(t *testing.T) {
	if !ShouldCompact(150000) {
		t.Fatal("ShouldCompact should return true for 150000 tokens")
	}
}

func TestShouldCompact_Zero(t *testing.T) {
	if ShouldCompact(0) {
		t.Fatal("ShouldCompact should return false for 0 tokens")
	}
}

func TestDefaultStoreDir(t *testing.T) {
	dir := DefaultStoreDir()
	if dir == "" {
		t.Fatal("DefaultStoreDir returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DefaultStoreDir should return absolute path, got %q", dir)
	}
}

func TestFileSessionStore_CreateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "store", "dir")
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Store directory not created: %v", err)
	}

	sess := &Session{
		ID:   "test",
		Name: "test",
	}
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
}

func TestFileSessionStore_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	all, err := store.List("", true)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("List should return empty slice for empty store, got %d sessions", len(all))
	}
}

func TestFileSessionStore_ListIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()

	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("text"), 0600); err != nil {
		t.Fatalf("Failed to write non-JSON file: %v", err)
	}

	sess := &Session{
		ID:   "valid",
		Name: "test",
	}
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	all, err := store.List("", true)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("List should return 1 session (ignoring non-JSON), got %d", len(all))
	}
}

func TestFileSessionStore_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	now := time.Now().Round(time.Millisecond)
	original := &Session{
		ID:               "roundtrip-1",
		Name:             "roundtrip-agent",
		Provider:         "codex",
		Model:            "gpt-5.6-luna",
		Tier:             "med",
		WorkingDir:       "/home/user/project",
		ParentSessionID:  "parent-123",
		CallerPID:        4567,
		HarnessType:      "agy-ide",
		Status:           "completed",
		TokensCumulative: 75000,
		TokensTurn:       8000,
		CachedTokens:     500,
		CreatedAt:        now,
		LastActiveAt:     now,
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := store.Get("roundtrip-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != original.ID ||
		retrieved.Name != original.Name ||
		retrieved.Provider != original.Provider ||
		retrieved.Model != original.Model ||
		retrieved.Tier != original.Tier ||
		retrieved.WorkingDir != original.WorkingDir ||
		retrieved.ParentSessionID != original.ParentSessionID ||
		retrieved.CallerPID != original.CallerPID ||
		retrieved.HarnessType != original.HarnessType ||
		retrieved.Status != original.Status ||
		retrieved.TokensCumulative != original.TokensCumulative ||
		retrieved.TokensTurn != original.TokensTurn ||
		retrieved.CachedTokens != original.CachedTokens {
		t.Fatal("Roundtrip failed: fields do not match")
	}
}

func TestFileSessionStore_PreservesStartPrompt(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := "Investigate the release process"
	if err := store.Save(&Session{ID: "prompt", Name: "prompt-agent", StartPrompt: want}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got.StartPrompt != want {
		t.Fatalf("StartPrompt = %q, want %q", got.StartPrompt, want)
	}
}

func TestOpenSessionStoreDoesNotCreateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	store, err := OpenSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := store.List("", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("sessions = %d, want 0", len(sessions))
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("store directory exists after read-only open: %v", err)
	}
}
