package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/quota1"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestRunStartRecordsQuotaBoundariesForTurn(t *testing.T) {
	storeDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "telemetry.sqlite")
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	driver := &recordingAgentDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var calls []bool
	var captureOnce sync.Once
	captureStarted := make(chan struct{})
	continueCapture := make(chan struct{})
	driver.runHook = func() {
		<-captureStarted
		close(continueCapture)
	}
	capture := func(_ context.Context, _ string, force bool) usage.TurnQuotaReading {
		captureOnce.Do(func() {
			close(captureStarted)
			<-continueCapture
		})
		calls = append(calls, force)
		return usage.TurnQuotaReading{CapturedAt: time.Now().UTC(), HasCache: true, CacheAgeMS: 123}
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = runStart(t.Context(), cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, quota: capture, dbPath: dbPath}, startRequest{Prompt: "p", StoredPrompt: "p", ModelSpec: "codex:luna", Dir: ".", StreamMode: streamFull, JSON: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []bool{false, false}) {
		t.Fatalf("quota capture force flags=%v, want [false false]", calls)
	}
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	usageStore, err := usagestore.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer usageStore.Close()
	var boundaries int
	if err := usageStore.QueryRow(context.Background(), `SELECT count(*) FROM turn_quota_boundaries WHERE session_id=? AND turn=1`, sessions[0].ID).Scan(&boundaries); err != nil {
		t.Fatal(err)
	}
	if boundaries != 2 {
		t.Fatalf("stored boundary count=%d, want 2", boundaries)
	}
	tokens, err := usageStore.TurnTokenUsages(context.Background(), sessions[0].ID)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("stored token rows=%+v err=%v, want delta and cumulative", tokens, err)
	}
	if _, err := os.Stat(filepath.Join(storeDir, "quota-readings.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("legacy quota spool exists after turn: stat err=%v", err)
	}
}

func TestTurnQuotaBaselineUsesBeforeCaptureCompletion(t *testing.T) {
	turnStarted := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	capturedAt := turnStarted.Add(5 * time.Second)
	before := usage.TurnQuotaReading{CapturedAt: capturedAt, HasCache: true, CacheAgeMS: 40}

	got := turnQuotaBaseline(turnStarted, before)
	want := capturedAt.Add(-40 * time.Millisecond)
	if !got.Equal(want) || !got.After(turnStarted) {
		t.Fatalf("turnQuotaBaseline = %s, want %s after turn start", got, want)
	}
}

func TestWarnQuota1Changes(t *testing.T) {
	dir := t.TempDir()
	state, _, err := quota1.ResolveStateFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	runAt := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if err := os.MkdirAll(filepath.Dir(state), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte(runAt.Format(time.RFC3339Nano)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%d.go", i))
		if err := os.WriteFile(path, []byte("package test\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, runAt.Add(time.Minute), runAt.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	warnQuota1Changes(cmd, dir, runAt)
	got := errOut.String()
	if !strings.Contains(got, "7 file(s) changed after the last quota-1 run at "+runAt.Format(time.RFC3339)+":") || !strings.Contains(got, "— code is untested, run make test-q1") {
		t.Fatalf("warning = %q", got)
	}
	if strings.Count(got, "file-") != 5 || out.Len() != 0 {
		t.Fatalf("warning should list 5 files once on stderr; stderr=%q stdout=%q", got, out.String())
	}
}

func TestAgentTurnsWarnAboutQuota1Changes(t *testing.T) {
	for _, mode := range []string{"start", "resume"} {
		t.Run(mode, func(t *testing.T) {
			repoDir := t.TempDir()
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			runAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
			state, _, err := quota1.ResolveStateFile(repoDir)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(state), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(state, []byte(runAt.Format(time.RFC3339Nano)+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(repoDir, "changed.go")
			if err := os.WriteFile(source, []byte("package changed\n"), 0644); err != nil {
				t.Fatal(err)
			}
			future := runAt.Add(time.Minute)
			if err := os.Chtimes(source, future, future); err != nil {
				t.Fatal(err)
			}
			driver := &recordingAgentDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			cmd := &cobra.Command{}
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}
			if mode == "start" {
				err = runStart(t.Context(), cmd, deps, startRequest{Prompt: "task", StoredPrompt: "task", ModelSpec: "codex:luna", Dir: repoDir, StreamMode: streamFull, JSON: true})
			} else {
				if err := store.Save(&subagent.Session{ID: "resume-id", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: repoDir, Status: "completed", ContextTokens: 100}); err != nil {
					t.Fatal(err)
				}
				deps.find = func(_ *cobra.Command, s *subagent.FileSessionStore, name string) (*subagent.Session, error) {
					sessions, err := s.List("", true)
					if err != nil {
						return nil, err
					}
					for _, session := range sessions {
						if session.Name == name || session.ID == name {
							return session, nil
						}
					}
					return nil, fmt.Errorf("session %q not found", name)
				}
				err = runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", StreamMode: streamFull, JSON: true})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr.String(), "1 file(s) changed after the last quota-1 run at "+runAt.Format(time.RFC3339)+": changed.go — code is untested, run make test-q1") {
				t.Fatalf("stderr warning = %q", stderr.String())
			}
			if !strings.HasPrefix(stdout.String(), "{") {
				t.Fatalf("stdout should remain JSON, got %q", stdout.String())
			}
		})
	}
}
