package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/privacy"
)

func TestClassifyTier1_Categories(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		note     string
		exitCode *int
		want     ActivityCategory
	}{
		// 1. Tool name heuristics
		{"Edit tool", "Edit", "", nil, CategoryEdit},
		{"apply_patch tool", "apply_patch", "", nil, CategoryEdit},
		{"Write tool", "Write", "", nil, CategoryEdit},
		{"Read tool", "Read", "", nil, CategoryInspection},
		{"Grep tool", "Grep", "", nil, CategoryInspection},
		{"find_by_name tool", "find_by_name", "", nil, CategoryInspection},
		{"view_file tool", "view_file", "", nil, CategoryInspection},
		{"read_url_content tool", "read_url_content", "", nil, CategoryInspection},
		{"web tool", "Web", "", nil, CategoryInspection},
		{"TestTool", "TestTool", "", nil, CategoryTest},
		{"Agent tool", "Agent", "", nil, CategoryWorkflow},
		{"AgentLifecycle tool", "AgentLifecycle", "", nil, CategoryWorkflow},
		{"list_agents tool", "list_agents", "", nil, CategoryWorkflow},
		{"Plan tool", "Plan", "", nil, CategoryWorkflow},

		// 2. ScoreShell notes and error outputs
		{"go test failure", "Bash", "go test failure", intPtr(1), CategoryTest},
		{"test failure", "Bash", "test failure", intPtr(1), CategoryTest},
		{"linter violation", "Bash", "linter violation", intPtr(1), CategoryTest},
		{"go build / syntax error", "Bash", "go build / syntax error", intPtr(2), CategoryBuild},
		{"rust compile error", "Bash", "rust compile error", intPtr(2), CategoryBuild},
		{"c/c++ build error", "Bash", "c/c++ build error", intPtr(2), CategoryBuild},
		{"go runtime panic", "Bash", "go runtime panic / fatal error", intPtr(2), CategoryDebug},
		{"segmentation fault", "Bash", "segmentation fault / fatal signal", intPtr(139), CategoryDebug},
		{"python traceback", "Bash", "python traceback / uncaught exception", intPtr(1), CategoryDebug},
		{"unhandled promise rejection", "Bash", "unhandled promise rejection / fatal error", intPtr(1), CategoryDebug},
		{"signal termination", "Bash", "signal termination (signal 9)", intPtr(137), CategoryDebug},

		// 3. Real note regex patterns
		{"committed ticket", "Bash", "committed ticket 128 + README", nil, CategoryWorkflow},
		{"updated README index", "Bash", "updated issues/README.md index for ticket 128", nil, CategoryWorkflow},
		{"filed issue", "Bash", "filed issue 128 per-agent system-prompt self-audit", nil, CategoryWorkflow},
		{"loaded tool feedback", "Bash", "loaded tool feedback protocol instructions", nil, CategoryWorkflow},
		{"git status clean", "Bash", "git status: clean tree, 5 commits ahead of origin/main", nil, CategoryGit},
		{"git commit command", "Bash", "git commit -m 'feat: something'", nil, CategoryGit},
		{"settings.json config", "Bash", "updated settings.json hooks", nil, CategoryConfig},
		{"dotfiles setup", "Bash", "setup dotfiles and environment setup", nil, CategoryConfig},
		{"go test execution", "Bash", "go test and diff check passed for 60 minute canvas timeout", nil, CategoryTest},
		{"all unit tests passed", "Bash", "All unit tests, vet/check, and installation passed after implementing feedback behavior.", nil, CategoryTest},
		{"go build execution", "Bash", "go build ./cmd/harnez", nil, CategoryBuild},
		{"make install", "Bash", "ran make install to refresh binary", nil, CategoryBuild},
		{"panic trace inspection", "Bash", "analyzing panic in goroutine 1", nil, CategoryDebug},
		{"read issue inspection", "Bash", "read issue 122 and 040 for ticket format/prior-art", nil, CategoryInspection},
		{"inspected issue numbering", "Bash", "inspected issue numbering and README format", nil, CategoryInspection},

		// 4. Fallback to other
		{"clean success bash", "Bash", "clean success", intPtr(0), CategoryOther},
		{"empty note bash", "Bash", "", intPtr(0), CategoryOther},
		{"unrecognized custom prose", "Bash", "something random happened here without keywords", intPtr(0), CategoryOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyTier1(tt.toolName, tt.note, tt.exitCode)
			if got != tt.want {
				t.Errorf("ClassifyTier1(%q, %q, %v) = %q, want %q", tt.toolName, tt.note, tt.exitCode, got, tt.want)
			}
		})
	}
}

// TestClassification_NeverLeaksSecretsInActivityCategory tests that secret strings
// (tokens, API keys, email addresses, absolute paths, passwords) embedded in notes
// are NEVER leaked into the ActivityCategory string, and that the serialized JSON
// for export only contains the canonical category enum.
func TestClassification_NeverLeaksSecretsInActivityCategory(t *testing.T) {
	secretStrings := []string{
		"sk-proj-1234567890abcdef1234567890abcdef",
		"ghp_0123456789abcdefghijklmnopqrstuvwxyz",
		"/home/secretuser/topsecret/internal_project/credentials.json",
		"admin@internal-corp.enterprise.com",
		"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"AcmeSuperConfidentialClientName",
	}

	for _, secret := range secretStrings {
		t.Run("secret_"+secret[:min(10, len(secret))], func(t *testing.T) {
			note := fmt.Sprintf("ran go test with token %s against secret server", secret)
			cat := ClassifyTier1("Bash", note, nil)
			if !cat.IsValid() {
				t.Fatalf("category %q is not a valid enum", cat)
			}
			catStr := string(cat)

			// The category string MUST NOT contain any part of the secret
			if strings.Contains(catStr, secret) {
				t.Fatalf("secret leaked directly into activity_category: %q", catStr)
			}
			for _, part := range strings.Split(secret, "/") {
				if len(part) > 4 && strings.Contains(catStr, part) {
					t.Fatalf("secret part %q leaked into activity_category: %q", part, catStr)
				}
			}

			// Also verify in ExportToolCall JSON serialization under all privacy levels
			for _, level := range []privacy.Level{privacy.LevelPublic, privacy.LevelAgentSanitized, privacy.LevelInternal, privacy.LevelRaw} {
				row := ToolCall{
					SessionID: "s1",
					ToolName:  "Bash",
					Note:      note,
				}
				exp := BuildExportLevel([]ToolCall{row}, time.Now(), level, nil)
				data, err := json.Marshal(exp)
				if err != nil {
					t.Fatalf("json marshal: %v", err)
				}
				var parsed struct {
					ToolCalls []struct {
						ActivityCategory string `json:"activity_category"`
					} `json:"tool_calls"`
				}
				if err := json.Unmarshal(data, &parsed); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if len(parsed.ToolCalls) != 1 {
					t.Fatalf("expected 1 tool call, got %d", len(parsed.ToolCalls))
				}
				ac := parsed.ToolCalls[0].ActivityCategory
				if ac != string(CategoryTest) {
					t.Errorf("expected activity_category %q, got %q", CategoryTest, ac)
				}
				if strings.Contains(ac, secret) {
					t.Fatalf("secret %q leaked in exported activity_category: %s", secret, data)
				}
			}
		})
	}
}

// fakeBatchClassifier is a test double for Tier 3 batch classification
type fakeBatchClassifier struct {
	calls     int
	callSizes []int
	mapping   map[string]ActivityCategory
}

func (f *fakeBatchClassifier) ClassifyBatch(_ context.Context, notes []string) ([]ActivityCategory, error) {
	f.calls++
	f.callSizes = append(f.callSizes, len(notes))
	out := make([]ActivityCategory, len(notes))
	for i, n := range notes {
		if cat, ok := f.mapping[n]; ok {
			out[i] = cat
		} else {
			out[i] = CategoryOther
		}
	}
	return out, nil
}

// TestTier2_ContentHashCaching verifies Tier 2 caching behavior:
// 1. Tier 1 matches skip cache and classifier entirely.
// 2. Cache miss triggers Tier 3 classifier once.
// 3. Subsequent call for same note hits Tier 2 cache with zero classifier invocations.
func TestTier2_ContentHashCaching(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	fc := &fakeBatchClassifier{
		mapping: map[string]ActivityCategory{
			"custom bespoke note 1": CategoryDebug,
			"custom bespoke note 2": CategoryConfig,
		},
	}

	calls := []ToolCall{
		{ToolName: "Bash", Note: "custom bespoke note 1"},
		{ToolName: "Bash", Note: "custom bespoke note 2"},
		{ToolName: "Bash", Note: "go test failure"}, // Tier 1 will match this as CategoryTest!
	}

	cats, err := ClassifyNotes(ctx, db, calls, fc, now)
	if err != nil {
		t.Fatalf("ClassifyNotes 1st: %v", err)
	}

	if len(cats) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(cats))
	}
	if cats[0] != CategoryDebug {
		t.Errorf("cats[0] = %q, want %q", cats[0], CategoryDebug)
	}
	if cats[1] != CategoryConfig {
		t.Errorf("cats[1] = %q, want %q", cats[1], CategoryConfig)
	}
	if cats[2] != CategoryTest {
		t.Errorf("cats[2] = %q, want %q", cats[2], CategoryTest)
	}

	// Classifier should have been called once for the 2 uncached notes (note 3 was resolved by Tier 1)
	if fc.calls != 1 {
		t.Fatalf("expected 1 classifier call, got %d", fc.calls)
	}
	if fc.callSizes[0] != 2 {
		t.Fatalf("expected batch size 2, got %d", fc.callSizes[0])
	}

	// Second run: exact same calls. Must be 100% served by Tier 1 & Tier 2 cache!
	cats2, err := ClassifyNotes(ctx, db, calls, fc, now)
	if err != nil {
		t.Fatalf("ClassifyNotes 2nd: %v", err)
	}
	if fc.calls != 1 {
		t.Fatalf("expected classifier call count to remain 1 (cache hit), got %d", fc.calls)
	}
	for i := range cats2 {
		if cats2[i] != cats[i] {
			t.Errorf("cats2[%d] = %q, want %q", i, cats2[i], cats[i])
		}
	}
}

// TestTier1_LatencyBudget asserts Tier 1 classification executes well under 1ms per call.
func TestTier1_LatencyBudget(t *testing.T) {
	notes := []string{
		"committed ticket 128 + README",
		"git status: clean tree, 5 commits ahead of origin/main",
		"go test and diff check passed for 60 minute canvas timeout",
		"go build ./cmd/harnez",
		"panic: runtime error: invalid memory address",
		"read issue 122 and 040 for ticket format/prior-art",
		"clean success",
		"some other arbitrary developer note",
	}

	const iterations = 5000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		n := notes[i%len(notes)]
		_ = ClassifyTier1("Bash", n, nil)
	}
	elapsed := time.Since(start)
	perCall := elapsed / time.Duration(iterations)

	// Under 1ms budget (in fact under 50µs)
	if perCall > time.Millisecond {
		t.Errorf("ClassifyTier1 latency per call = %v, want < 1ms", perCall)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestDefaultLocalClassifier_ClassifyBatch_ValidResponse verifies that a
// mocked local OpenAI-compatible endpoint's per-turn chat-completions replies
// are parsed into the expected ActivityCategory enum values, that the
// request never targets anything but the configured local BaseURL, and that
// the message history sent on each turn correctly grows to include every
// prior note and reply (the "one growing conversation" contract).
func TestDefaultLocalClassifier_ClassifyBatch_ValidResponse(t *testing.T) {
	notes := []string{"go test ./... failed", "some unrelated developer note"}
	want := []ActivityCategory{CategoryTest, CategoryOther}

	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		wantLen := 2 + 2*turn // system + (user,assistant)*turn + this turn's user
		if len(req.Messages) != wantLen {
			t.Fatalf("turn %d: got %d messages, want %d: %+v", turn, len(req.Messages), wantLen, req.Messages)
		}
		if req.Messages[0].Role != "system" {
			t.Fatalf("turn %d: messages[0].Role = %q, want system", turn, req.Messages[0].Role)
		}
		last := req.Messages[len(req.Messages)-1]
		wantContent := fmt.Sprintf("%d: %s", turn+1, notes[turn])
		if last.Role != "user" || last.Content != wantContent {
			t.Fatalf("turn %d: last message = %+v, want user %q", turn, last, wantContent)
		}

		reply := fmt.Sprintf("%d: %s", turn+1, want[turn])
		resp := chatCompletionResponse{}
		resp.Choices = []struct {
			Message chatCompletionMessage `json:"message"`
		}{
			{Message: chatCompletionMessage{Role: "assistant", Content: reply}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		turn++
	}))
	defer srv.Close()

	c := &DefaultLocalClassifier{BaseURL: srv.URL + "/v1", Timeout: 5 * time.Second}
	got, err := c.ClassifyBatch(context.Background(), notes)
	if err != nil {
		t.Fatalf("ClassifyBatch() unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ClassifyBatch() returned %d categories, want %d", len(got), len(want))
	}
	for i, cat := range got {
		if cat != want[i] {
			t.Errorf("ClassifyBatch()[%d] = %q, want %q", i, cat, want[i])
		}
	}
	if turn != len(notes) {
		t.Fatalf("server saw %d turns, want %d", turn, len(notes))
	}
}

// TestDefaultLocalClassifier_ClassifyBatch_ServerDown verifies that when the
// local endpoint is unreachable, ClassifyBatch returns an error (rather than
// hanging or panicking) so ClassifyNotes' caller-side fallback to
// CategoryOther can take over.
func TestDefaultLocalClassifier_ClassifyBatch_ServerDown(t *testing.T) {
	// Use a port that is not listening: start and immediately close a server
	// to obtain a URL with nothing bound to it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close()

	c := &DefaultLocalClassifier{BaseURL: deadURL + "/v1", Timeout: 2 * time.Second}
	_, err := c.ClassifyBatch(context.Background(), []string{"a note"})
	if err == nil {
		t.Fatal("ClassifyBatch() expected error for unreachable local endpoint, got nil")
	}
}

// TestDefaultLocalClassifier_ClassifyBatch_BadResponse verifies that a
// malformed (not "<n>: <category>") model reply for one turn falls back to
// CategoryOther for that note alone, without erroring the whole batch — a
// single wayward reply should never invalidate an otherwise-working session.
func TestDefaultLocalClassifier_ClassifyBatch_BadResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{}
		resp.Choices = []struct {
			Message chatCompletionMessage `json:"message"`
		}{
			{Message: chatCompletionMessage{Role: "assistant", Content: "not a valid turn reply at all"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := &DefaultLocalClassifier{BaseURL: srv.URL + "/v1", Timeout: 5 * time.Second}
	got, err := c.ClassifyBatch(context.Background(), []string{"a note"})
	if err != nil {
		t.Fatalf("ClassifyBatch() unexpected error for malformed turn reply: %v", err)
	}
	if len(got) != 1 || got[0] != CategoryOther {
		t.Fatalf("ClassifyBatch() = %v, want [%q] (graceful per-note fallback)", got, CategoryOther)
	}
}

// TestDefaultLocalClassifier_ClassifyBatch_MidSessionFailure verifies that
// when the endpoint answers the first turn but then fails partway through a
// session, notes already classified are kept and only the remainder falls
// back to CategoryOther — a partial session shouldn't discard prior work.
func TestDefaultLocalClassifier_ClassifyBatch_MidSessionFailure(t *testing.T) {
	notes := []string{"go test failed", "second note", "third note"}
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if turn == 1 {
			// Simulate the server dying after the first turn.
			panic(http.ErrAbortHandler)
		}
		resp := chatCompletionResponse{}
		resp.Choices = []struct {
			Message chatCompletionMessage `json:"message"`
		}{
			{Message: chatCompletionMessage{Role: "assistant", Content: fmt.Sprintf("%d: test", turn+1)}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		turn++
	}))
	defer srv.Close()

	c := &DefaultLocalClassifier{BaseURL: srv.URL + "/v1", Timeout: 2 * time.Second}
	got, err := c.ClassifyBatch(context.Background(), notes)
	if err != nil {
		t.Fatalf("ClassifyBatch() unexpected error for mid-session failure: %v", err)
	}
	want := []ActivityCategory{CategoryTest, CategoryOther, CategoryOther}
	if len(got) != len(want) {
		t.Fatalf("ClassifyBatch() returned %d categories, want %d", len(got), len(want))
	}
	for i, cat := range got {
		if cat != want[i] {
			t.Errorf("ClassifyBatch()[%d] = %q, want %q", i, cat, want[i])
		}
	}
}

// TestDefaultLocalClassifier_ClassifyBatch_ViaClassifyNotes verifies that the
// existing ClassifyNotes fallback-to-CategoryOther logic still triggers
// gracefully when the HTTP-based classifier fails, with no panic or hang.
func TestDefaultLocalClassifier_ClassifyBatch_ViaClassifyNotes(t *testing.T) {
	c := &DefaultLocalClassifier{BaseURL: "http://127.0.0.1:1/v1", Timeout: 500 * time.Millisecond}
	calls := []ToolCall{
		{ToolName: "Bash", Note: "totally unclassifiable developer prose"},
	}
	cats, err := ClassifyNotes(context.Background(), nil, calls, c, time.Now())
	if err != nil {
		t.Fatalf("ClassifyNotes() unexpected error: %v", err)
	}
	if len(cats) != 1 || cats[0] != CategoryOther {
		t.Fatalf("ClassifyNotes() = %v, want [%q] (graceful fallback)", cats, CategoryOther)
	}
}
