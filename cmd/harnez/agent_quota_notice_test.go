package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

type quotaNoticeDriver struct {
	recordingAgentDriver
	before func()
}

func (d *quotaNoticeDriver) Run(ctx context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	d.before()
	return d.recordingAgentDriver.Run(ctx, opts)
}
func (d *quotaNoticeDriver) Resume(ctx context.Context, id, prompt string, model subagent.Model) (*subagent.TurnResult, error) {
	d.before()
	return d.recordingAgentDriver.Resume(ctx, id, prompt, model)
}

func TestAgentQuotaNoticeBeforeStartAndResumeJSON(t *testing.T) {
	for _, mode := range []string{"start", "resume"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(agentRoleEnv, "")
			t.Setenv(agentSessionEnv, "")
			repo := t.TempDir()
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			cmd := &cobra.Command{}
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			driver := &quotaNoticeDriver{before: func() {
				if !strings.HasPrefix(stderr.String(), "quota: codex 5h 86% (read ") {
					t.Errorf("warning missing before provider turn: %q", stderr.String())
				}
			}}
			old := agentDriver
			agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
			defer func() { agentDriver = old }()
			deps := agentDeps{
				store:  func() (*subagent.FileSessionStore, error) { return store, nil },
				parent: func() string { return "" },
				quota:  func(context.Context, string, bool) usage.TurnQuotaReading { return usage.TurnQuotaReading{} },
				availability: func(string, string) usage.ProviderQuotaAvailability {
					return usage.ProviderQuotaAvailability{State: "available", Windows: []usagestore.Window{{Key: "five_hour", UsedFraction: .86, ObservedAt: time.Now().Add(-40 * time.Second), Freshness: "fresh"}}}
				},
				find: func(_ *cobra.Command, s *subagent.FileSessionStore, name string) (*subagent.Session, error) {
					return s.Find(name)
				},
			}
			if mode == "start" {
				err = runStart(t.Context(), cmd, deps, startRequest{Prompt: "task", ModelSpec: "codex:luna", Dir: repo, JSON: true, StreamMode: streamFull})
			} else {
				if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: repo, Status: "completed", ContextTokens: 100}); err != nil {
					t.Fatal(err)
				}
				err = runResume(t.Context(), cmd, deps, resumeRequest{Name: "worker", Prompt: "continue", JSON: true, StreamMode: streamFull})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(stdout.Bytes()) {
				t.Fatalf("invalid JSON stdout: %q", stdout.String())
			}
			if strings.Count(stderr.String(), "quota: ") != 1 {
				t.Fatalf("expected one notice: %q", stderr.String())
			}
		})
	}
}
