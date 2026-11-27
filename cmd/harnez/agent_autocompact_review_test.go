package main

import (
	"bytes"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

// TestRefusedResumeKeepsCompactTokens: a resume refused before launch must not
// persist a new --autocompact window (issue 1305 M4 item 1).
func TestRefusedResumeKeepsCompactTokens(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(sess *subagent.Session)
		parent  string
		wantErr string
	}{
		{
			name:    "blocked session",
			mutate:  func(sess *subagent.Session) { sess.ResumeBlockedReason = "test block" },
			wantErr: "cannot be resumed: test block",
		},
		{
			name:    "outside caller lineage",
			parent:  "other-caller",
			mutate:  func(sess *subagent.Session) { sess.ParentSessionID = "someone-else" },
			wantErr: "outside caller lineage",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(agentSessionEnv, "caller-session")
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			sess := &subagent.Session{ID: "sid-refused", Name: "refused", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-refused", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1, CompactTokens: 120000}
			tc.mutate(sess)
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			deps := toolLimitTestDeps(store)
			deps.parent = func() string { return tc.parent }
			req := resumeRequest{Name: "refused", Prompt: "continue", StreamMode: streamFull, AutocompactSet: true, AutocompactTokens: 150000}
			err = runResume(t.Context(), claudeLiveCmd(), deps, req)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("runResume error = %v, want %q", err, tc.wantErr)
			}
			got, err := resolveSession(store, "refused", "")
			if err != nil {
				t.Fatal(err)
			}
			if got.CompactTokens != 120000 {
				t.Fatalf("stored CompactTokens = %d after refused resume, want 120000", got.CompactTokens)
			}
		})
	}
}

// TestCompactChildGetsStoredWindow: `agent compact` builds its driver from the
// stored session model, so the Claude child gets the stored window (M4 item 2).
func TestCompactChildGetsStoredWindow(t *testing.T) {
	stream := claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_compact", 1000, "null"), claudeLiveResult(1000))
	envPath := autocompactClaude(t, stream, "true")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "sid-compact", Name: "compact-window", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-compact", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1, CompactTokens: 120000}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := claudeLiveCmd()
	cmd.SetContext(t.Context())
	if err := compactSession(cmd, store, sess); err != nil {
		t.Fatalf("compactSession: %v", err)
	}
	if got, want := readWindow(t, envPath), claudeWindowFor(t, 120000); got != want {
		t.Fatalf("compact child window = %q, want %s (120000 plus the spec offset)", got, want)
	}
}

// TestResumeInteractiveRejectsAutocompact: --autocompact with resume -i fails
// before the interactive branch runs (M4 item 3).
func TestResumeInteractiveRejectsAutocompact(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid-interactive", Name: "chat-auto", Provider: "claude", Model: "haiku", Tier: "low", HarnessType: "interactive", Status: "completed", ProviderSessionID: "sess-chat"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "--name", "chat-auto", "--autocompact", "120k", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--autocompact cannot be combined with interactive mode") {
		t.Fatalf("resume -i --autocompact error = %v", err)
	}
}

// TestResumeDryRunAutocompactRejectsUnsupportedProvider: a dry run with
// --autocompact on agy fails before any preview (M4 item 4).
func TestResumeDryRunAutocompactRejectsUnsupportedProvider(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid-agy", Name: "agy-worker", Provider: "agy", Model: "gemini", Tier: "low", Status: "completed", ProviderSessionID: "sess-agy", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := claudeLiveCmd()
	cmd.SetOut(&out)
	deps := toolLimitTestDeps(store)
	req := resumeRequest{Name: "agy-worker", Prompt: "continue", StreamMode: streamFull, DryRun: true, AutocompactSet: true, AutocompactTokens: 120000}
	err = runResume(t.Context(), cmd, deps, req)
	if err == nil || !strings.Contains(err.Error(), "--autocompact is supported for claude and codex only") {
		t.Fatalf("runResume dry-run error = %v", err)
	}
	if strings.Contains(out.String(), "argv:") {
		t.Fatalf("dry run printed a preview despite the refusal: %s", out.String())
	}
}
