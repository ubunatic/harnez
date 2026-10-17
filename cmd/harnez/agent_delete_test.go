package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentStopAndDeleteAllManageableSessions(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "one", Name: "one", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}},
		{ID: "two", Name: "two", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "foreign"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"stop", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(recorder.stopped) != 1 || recorder.stopped[0] != "one" {
		t.Fatalf("stopped sessions = %#v", recorder.stopped)
	}

	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(recorder.deleted) != 1 || recorder.deleted[0] != "one" {
		t.Fatalf("deleted sessions = %#v", recorder.deleted)
	}
	if _, err := store.Get("one"); err == nil {
		t.Fatal("manageable session was not deleted")
	}
	if _, err := store.Get("two"); err != nil {
		t.Fatalf("foreign session was deleted: %v", err)
	}
}

func TestAgentDeleteAllSkipsUnratedSessionWithWarning(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "unrated", Name: "needs-rating", Provider: "agy", Model: "gemini", Status: "completed", HarnessType: "interactive", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1}}}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"delete", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --all error = %v", err)
	}
	if got := stderr.String(); !strings.Contains(got, "warning: unrated latest turns:") || !strings.Contains(got, "needs-rating: harnez agent rate --name needs-rating <1-5> \"<reason>\"") {
		t.Fatalf("warning = %q", got)
	}
	if _, err := store.Get(sess.ID); err != nil {
		t.Fatalf("unrated session was deleted: %v", err)
	}
}

func TestAgentDeleteAllRepeatSafeAndRetryFailed(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	failed := &subagent.Session{ID: "failed", Name: "failed-session", Provider: "agy", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller"}
	if err := store.Save(failed); err != nil {
		t.Fatal(err)
	}
	old := agentDriver
	shouldFail := true
	var deleted []string
	agentDriver = func(subagent.Model, string) subagent.Driver {
		return mockAgentDeleteDriver{delete: func(id string) error {
			deleted = append(deleted, id)
			if id == "failed" && shouldFail {
				return errors.New("provider delete failed")
			}
			return nil
		}}
	}
	defer func() { agentDriver = old }()
	run := func(args ...string) (string, string, error) {
		var out, stderr bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		err := cmd.Execute()
		return out.String(), stderr.String(), err
	}
	_, _, err = run("delete", "--all", "--force")
	if err == nil {
		t.Fatal("first delete-all should expose the provider failure")
	}
	got, err := store.Get("failed")
	if err != nil || got.Status != "delete-failed" || !strings.Contains(got.LastError, "provider delete failed") {
		t.Fatalf("failed record = %#v, %v", got, err)
	}
	listOut, _, err := run("list")
	if err != nil || strings.Contains(listOut, "failed-session") {
		t.Fatalf("default list = %q, err %v", listOut, err)
	}
	listOut, _, err = run("list", "--failed")
	if err != nil || !strings.Contains(listOut, "failed-session") || !strings.Contains(listOut, "delete-failed") {
		t.Fatalf("failed list = %q, err %v", listOut, err)
	}
	_, _, err = run("delete", "--all", "--force")
	if err != nil {
		t.Fatalf("second delete-all: %v", err)
	}
	shouldFail = false
	_, _, err = run("delete", "--retry-failed")
	if err != nil {
		t.Fatalf("retry-failed: %v", err)
	}
	if _, err := store.Get("failed"); err == nil {
		t.Fatal("successful retry retained failed record")
	}
	if len(deleted) != 2 {
		t.Fatalf("provider deletes = %#v", deleted)
	}
}

func TestAgentDeleteAllRetriesCodexDeleteFailedRecordWithMissingRollout(t *testing.T) {
	storeDir := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HARNEZ_SESSION_ID", "caller")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "missing-rollout", Name: "never-started", Provider: "codex", Model: "model", Status: "delete-failed", LastError: "unknown session", HarnessType: "batch", ParentSessionID: "caller"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--all", "--force", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete --all: %v", err)
	}
	if _, err := store.Get(sess.ID); err == nil {
		t.Fatal("missing-rollout failed record was not cleared")
	}
}

func TestAgentDeleteAllForceSuppressesWarningAndDiscoversNewSession(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	first := &subagent.Session{ID: "first", Name: "first-session", Provider: "claude", Model: "model", Status: "completed", HarnessType: "interactive", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1}}}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	run := func(force bool) (string, string, error) {
		var out, stderr bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		args := []string{"delete", "--all"}
		if force {
			args = append(args, "--force")
		}
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		err := cmd.Execute()
		return out.String(), stderr.String(), err
	}
	_, stderr, err := run(false)
	if err != nil || !strings.Contains(stderr, "unrated latest turns") {
		t.Fatalf("without force stderr=%q err=%v", stderr, err)
	}
	_, stderr, err = run(true)
	if err != nil || strings.Contains(stderr, "unrated latest turns") {
		t.Fatalf("with force stderr=%q err=%v", stderr, err)
	}
	if err := store.Save(&subagent.Session{ID: "new", Name: "new-session", Provider: "claude", Model: "model", Status: "completed", HarnessType: "interactive", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}}); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(true)
	if err != nil || !strings.Contains(out, "new-session") {
		t.Fatalf("new session output=%q err=%v", out, err)
	}
}

type mockAgentDeleteDriver struct{ delete func(string) error }

func (d mockAgentDeleteDriver) Run(context.Context, subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, nil
}

func (d mockAgentDeleteDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	return nil, nil
}

func (d mockAgentDeleteDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return nil, nil
}

func (d mockAgentDeleteDriver) Stop(context.Context, string) error { return nil }

func (d mockAgentDeleteDriver) Delete(_ context.Context, id string) error { return d.delete(id) }

func TestAgentStopAndDeleteAllWithNoSessions(t *testing.T) {
	storeDir := t.TempDir()
	for _, args := range [][]string{{"stop", "--all"}, {"delete", "--all"}} {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestAgentDeleteAllCompleted(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "completed1", Name: "done1", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}},
		{ID: "completed2", Name: "done2", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}},
		{ID: "running", Name: "active", Provider: "codex", Model: "model", Status: "active", HarnessType: "batch", ParentSessionID: "caller"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model, string) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"delete", "--all-completed", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "done1") || !strings.Contains(output, "done2") || strings.Contains(output, "active") {
		t.Fatalf("output = %q, want done1 and done2, not active", output)
	}
	if len(recorder.deleted) != 2 {
		t.Fatalf("deleted sessions = %#v, want 2", recorder.deleted)
	}
	if _, err := store.Get("completed1"); err == nil {
		t.Fatal("completed1 was not deleted")
	}
	if _, err := store.Get("completed2"); err == nil {
		t.Fatal("completed2 was not deleted")
	}
	if _, err := store.Get("running"); err != nil {
		t.Fatalf("active session was deleted: %v", err)
	}
}

func TestAgentDeleteAllCompletedRejectsFlagConflicts(t *testing.T) {
	storeDir := t.TempDir()
	for _, args := range [][]string{
		{"delete", "--all-completed", "--all"},
		{"delete", "--all-completed", "--name", "session"},
		{"delete", "--all-completed", "somename"},
	} {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err == nil {
			t.Fatalf("%v: err = %v, want error", args, err)
		}
	}
}

func TestRateLatestSessionTurnPersistsRatingAndReason(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "session-1", Name: "worker", Turn: 2, TurnRecords: []subagent.TurnRecord{{Turn: 1}, {Turn: 2}}}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if err := rateLatestSessionTurn(store, sess, 4, "tests green"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TurnRecords[1].Rating == nil || *loaded.TurnRecords[1].Rating != 4 || loaded.TurnRecords[1].RatingReason != "tests green" {
		t.Fatalf("latest turn rating = %+v", loaded.TurnRecords[1])
	}
	if err := rateLatestSessionTurn(store, loaded, 6, "invalid"); err == nil {
		t.Fatal("out-of-range score accepted")
	}
}

func TestRateLatestSessionTurnCreatesRecordForPreM1Session(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "legacy", Name: "dev519", Turn: 2}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if err := rateLatestSessionTurn(store, sess, 4, "good run"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.TurnRecords) != 1 || loaded.TurnRecords[0].Turn != 2 || loaded.TurnRecords[0].Rating == nil || *loaded.TurnRecords[0].Rating != 4 {
		t.Fatalf("legacy session turn records = %+v", loaded.TurnRecords)
	}
}

func TestAgentDeleteWarnsAndProtectsUnratedSessions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		rated      bool
		force      bool
		wantDelete bool
		wantWarn   bool
	}{
		{name: "rated", rated: true, wantDelete: true},
		{name: "unrated", wantWarn: true},
		{name: "forced", force: true, wantDelete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			sess := &subagent.Session{ID: "session-" + tc.name, Name: "worker", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1}}, HarnessType: "interactive", Status: "completed"}
			if tc.rated {
				rating := 4
				sess.TurnRecords[0].Rating = &rating
			}
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			out, stderr := new(bytes.Buffer), new(bytes.Buffer)
			cmd := newAgentCmd()
			cmd.SetOut(out)
			cmd.SetErr(stderr)
			args := []string{"delete", "--name", "worker", "--store-dir", storeDir}
			if tc.force {
				args = append(args, "--force")
			}
			cmd.SetArgs(args)
			err = cmd.Execute()
			if tc.wantDelete && err != nil {
				t.Fatalf("delete error = %v", err)
			}
			if !tc.wantDelete && err == nil {
				t.Fatal("unrated session deletion succeeded without --force")
			}
			if got := stderr.String(); tc.wantWarn {
				if !strings.Contains(got, "warning: unrated latest turns:") || !strings.Contains(got, `worker: harnez agent rate --name worker <1-5> "<reason>"`) {
					t.Fatalf("warning = %q", got)
				}
			} else if strings.Contains(stderr.String(), "warning: unrated latest turns:") {
				t.Fatalf("rated deletion warning = %q", stderr.String())
			}
			_, findErr := store.Get(sess.ID)
			if tc.wantDelete && findErr == nil {
				t.Fatal("session remains after successful deletion")
			}
			if !tc.wantDelete && findErr != nil {
				t.Fatalf("refused session was deleted: %v", findErr)
			}
		})
	}
}
