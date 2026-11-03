package subagent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const julesAdoptID = "5832791262383075829"

func julesStoreCount(t *testing.T, store *FileSessionStore) int {
	t.Helper()
	sessions, err := store.List("", true)
	if err != nil {
		t.Fatal(err)
	}
	return len(sessions)
}

func assertNoJulesKey(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), julesTestKey) {
		t.Fatalf("API key leaked into error: %v", err)
	}
}

func TestAdoptJulesSessionUnknownIDWritesNothing(t *testing.T) {
	for _, status := range []int{404, 401} {
		fake := &fakeJules{sessionID: julesAdoptID, states: []julesSession{{}}, getStatus: []int{status}}
		h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
		store, err := NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		_, err = AdoptJulesSession(context.Background(), store, h.driver, julesAdoptID, "", t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "HTTP ") {
			t.Fatalf("status %d: err = %v, want a clean API error", status, err)
		}
		assertNoJulesKey(t, err)
		if n := julesStoreCount(t, store); n != 0 {
			t.Errorf("status %d: an unknown ID left %d records", status, n)
		}
	}
}

func TestAdoptJulesSessionCreatesOneRecordAndReusesIt(t *testing.T) {
	fake := &fakeJules{sessionID: julesAdoptID, states: []julesSession{{State: "AWAITING_USER_FEEDBACK"}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := AdoptJulesSession(context.Background(), store, h.driver, julesAdoptID, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "jules-075829" || first.Provider != "jules" || first.ProviderSessionID != julesAdoptID {
		t.Fatalf("adopted record = %+v", first)
	}
	if n := julesStoreCount(t, store); n != 1 {
		t.Fatalf("records after first adopt = %d, want 1", n)
	}
	second, err := AdoptJulesSession(context.Background(), store, h.driver, julesAdoptID, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Errorf("second adopt made record %s, want reused %s", second.ID, first.ID)
	}
	if n := julesStoreCount(t, store); n != 1 {
		t.Errorf("records after second adopt = %d, want 1", n)
	}
	if fake.gets != 1 {
		t.Errorf("GETs = %d, want 1: a repeat adopt needs no second read", fake.gets)
	}
}

func TestAdoptJulesSessionNameRules(t *testing.T) {
	fake := &fakeJules{sessionID: julesAdoptID, states: []julesSession{{State: "AWAITING_USER_FEEDBACK"}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdoptJulesSession(context.Background(), store, h.driver, julesAdoptID, "mine", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := AdoptJulesSession(context.Background(), store, h.driver, julesAdoptID, "other", t.TempDir()); err == nil || !strings.Contains(err.Error(), "already adopted as") {
		t.Errorf("a different --name for an adopted ID: err = %v", err)
	}
	taken, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := taken.Create(&Session{ID: "taken-id", Name: "taken", Provider: "claude"}); err != nil {
		t.Fatal(err)
	}
	_, err = AdoptJulesSession(context.Background(), taken, h.driver, julesAdoptID, "taken", t.TempDir())
	if !errors.Is(err, ErrSessionNameInUse) {
		t.Errorf("a name owned by another session: err = %v, want ErrSessionNameInUse", err)
	}
	if n := julesStoreCount(t, taken); n != 1 {
		t.Errorf("records = %d, want 1: a refused adopt must not write", n)
	}
}

func TestAdoptJulesSessionRejectsMalformedIDBeforeAnyRequest(t *testing.T) {
	fake := &fakeJules{states: []julesSession{{}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdoptJulesSession(context.Background(), store, h.driver, "../sessions", "", t.TempDir()); err == nil {
		t.Fatal("a path-like ID was accepted")
	}
	if fake.gets != 0 {
		t.Errorf("GETs = %d for a malformed ID, want 0", fake.gets)
	}
}

func TestInspectJulesSessionIsOneReadAndWritesNothing(t *testing.T) {
	fake := &fakeJules{sessionID: julesAdoptID, states: []julesSession{{State: "AWAITING_USER_FEEDBACK", URL: "https://jules.test/session/x"}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	status, err := h.driver.InspectJulesSession(context.Background(), julesAdoptID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "AWAITING_USER_FEEDBACK" {
		t.Errorf("state = %q", status.State)
	}
	if fake.gets != 1 || *h.sleeps != 0 {
		t.Errorf("GETs = %d sleeps = %d, want one read and no polling", fake.gets, *h.sleeps)
	}
	if !strings.Contains(status.Summary(), `harnez agent resume --model jules --id `+julesAdoptID) {
		t.Errorf("summary lacks the next step:\n%s", status.Summary())
	}
}

func TestResumeWaitOnAdoptedSessionSendsNoMessage(t *testing.T) {
	fake := &fakeJules{sessionID: julesAdoptID, states: []julesSession{{State: "AWAITING_USER_FEEDBACK"}}}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	if _, err := h.driver.Resume(context.Background(), julesAdoptID, "wait", Model{}); err != nil {
		t.Fatal(err)
	}
	if len(fake.messages) != 0 || fake.approvals != 0 {
		t.Errorf("wait sent messages %v and approvals %d", fake.messages, fake.approvals)
	}
}

func TestInspectJulesSessionShowsReasonAndLastMessage(t *testing.T) {
	acts := []map[string]any{
		{"originator": "agent", "createTime": "2026-11-02T10:00:01Z", "agentMessaged": map[string]any{"agentMessage": "Which branch should I use?"}},
		{"createTime": "2026-11-02T10:00:02Z", "sessionFailed": map[string]any{"reason": "setup script failed"}},
	}
	fake := &fakeJules{sessionID: julesAdoptID, states: []julesSession{{State: "FAILED"}}, activities: acts}
	h := newJulesHarness(t, fake, julesTestRemotes, julesEqualHeads)
	status, err := h.driver.InspectJulesSession(context.Background(), julesAdoptID)
	if err != nil {
		t.Fatal(err)
	}
	sum := status.Summary()
	for _, want := range []string{"Reason: setup script failed", "Jules says: Which branch should I use?"} {
		if !strings.Contains(sum, want) {
			t.Errorf("summary lacks %q:\n%s", want, sum)
		}
	}
}
