package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

type quotaExitDriver struct {
	recordingAgentDriver
	captureStarted <-chan struct{}
}

func (d *quotaExitDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	<-d.captureStarted
	return &subagent.TurnResult{ContextTokens: 50000}, nil
}

func (d *quotaExitDriver) RunStream(ctx context.Context, _ subagent.RunOptions, emit subagent.EventFunc) (*subagent.TurnResult, error) {
	<-d.captureStarted
	emit(subagent.Event{Kind: "session", Text: "provider-thread"})
	emit(subagent.Event{Kind: "tokens", ContextTokens: 1000000000})
	return nil, ctx.Err()
}

func (d *quotaExitDriver) ResumeStream(ctx context.Context, _, _ string, _ subagent.Model, emit subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.RunStream(ctx, subagent.RunOptions{}, emit)
}

func TestAgentEarlyReturnJoinsQuotaCapture(t *testing.T) {
	for _, path := range []string{"resume compaction", "resume watchdog", "start watchdog"} {
		t.Run(path, func(t *testing.T) {
			t.Setenv(agentRoleEnv, "")
			t.Setenv(agentSessionEnv, "")
			t.Setenv("HOME", t.TempDir())
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", ContextTokens: 1000000000}); err != nil {
				t.Fatal(err)
			}
			started, finished := make(chan struct{}), make(chan struct{})
			driver := &quotaExitDriver{captureStarted: started}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			deps := agentDeps{
				store:  func() (*subagent.FileSessionStore, error) { return store, nil },
				parent: func() string { return "" },
				find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
					return s.Find(id)
				},
				quota: func(ctx context.Context, _ string, _ bool) usage.TurnQuotaReading {
					close(started)
					<-ctx.Done()
					defer close(finished)
					return usage.TurnQuotaReading{Error: ctx.Err().Error()}
				},
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cmd := &cobra.Command{}
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			wantError := "crossed live token threshold"
			if path == "start watchdog" {
				err = runStart(ctx, cmd, deps, startRequest{Role: "advisor", Prompt: "prompt", ModelSpec: "codex:luna", Name: "new-worker", Dir: ".", StreamMode: streamFull})
			} else {
				json := path == "resume compaction"
				if json {
					wantError = "Codex did not auto-compact"
				}
				err = runResume(ctx, cmd, deps, resumeRequest{Name: "worker", Prompt: "prompt", JSON: json, StreamMode: streamFull})
			}
			select {
			case <-finished:
			default:
				// Drain even on a regression so this test cannot leak the writer.
				cancel()
				select {
				case <-started:
					<-finished
				default: // The command may have failed before starting capture.
				}
				t.Error("command returned before quota capture finished")
			}
			if err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("error = %v, want %q", err, wantError)
			}
		})
	}
}
