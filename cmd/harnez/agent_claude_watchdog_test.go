package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
)

// Stream shapes come from a live `claude -p --model haiku --output-format
// stream-json --verbose` run (claude-code 2.1.330, 2026-11-27, stdout only).
// An assistant line carries message.id and message.usage with input_tokens,
// cache_creation_input_tokens, cache_read_input_tokens and cache_creation, plus
// a top-level parent_tool_use_id (null for main-chain calls). The result line
// carries a top-level usage that totals every call of the turn (ticket 1264 M1).

func claudeLiveInit() string {
	return `{"type":"system","subtype":"init","cwd":"/tmp/work","session_id":"sess-live","model":"claude-haiku-5-5","permissionMode":"default"}`
}

// claudeLiveAssistant is one assistant line whose request prompt is total
// tokens: input 2, cache creation 4694, the rest cache read. parent is the raw
// JSON value of parent_tool_use_id ("null" or a quoted tool id).
func claudeLiveAssistant(msgID string, total int, parent string) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"model":"claude-haiku-5-5","id":%q,"type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":2,"cache_creation_input_tokens":4694,"cache_read_input_tokens":%d,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":4694},"output_tokens":4,"service_tier":"standard"}},"parent_tool_use_id":%s,"session_id":"sess-live"}`, msgID, total-2-4694, parent)
}

// claudeLiveAssistantNoUsage is an assistant line without a usage block.
func claudeLiveAssistantNoUsage(msgID string) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"model":"claude-haiku-5-5","id":%q,"type":"message","role":"assistant","content":[{"type":"text","text":"ok"}]},"parent_tool_use_id":null,"session_id":"sess-live"}`, msgID)
}

// claudeLiveResult is the final result line; its usage totals all calls.
func claudeLiveResult(total int) string {
	return fmt.Sprintf(`{"type":"result","subtype":"success","is_error":false,"session_id":"sess-live","result":"ok","usage":{"input_tokens":2,"cache_creation_input_tokens":4694,"cache_read_input_tokens":%d,"output_tokens":4}}`, total-2-4694)
}

func claudeLiveStream(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// fakeClaudeLive puts a `claude` on PATH that prints stream and then runs tail.
// A tail of "sleep 5" keeps the turn open until the watchdog kills the group;
// the sleep is bounded so a regression fails instead of hanging.
func fakeClaudeLive(t *testing.T, stream, tail string) {
	t.Helper()
	dir := t.TempDir()
	streamPath := filepath.Join(dir, "stream.jsonl")
	if err := os.WriteFile(streamPath, []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}
	setupFakeProviderEnv(t, map[string]string{"claude": "#!/bin/sh\ncat " + streamPath + "\n" + tail + "\n"})
}

func claudeLiveCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Duration("timeout", 0, "")
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	return cmd
}

func TestClaudeWatchdogStopsStartAtCrossing(t *testing.T) {
	for _, tc := range []struct {
		name         string
		total        int
		wantObserved int
	}{
		{name: "exact threshold", total: 100000, wantObserved: 100000},
		{name: "live capture 134k request", total: 134000, wantObserved: 134000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", tc.total, "null")), "sleep 5")
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			err = runStart(context.Background(), claudeLiveCmd(), toolLimitTestDeps(store), startRequest{
				Name: "claude-live", Role: "advisor", Prompt: "say ok", StoredPrompt: "say ok",
				ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull,
			})
			want := fmt.Sprintf("crossed live token threshold at %d tokens (limit 100000)", tc.wantObserved)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("runStart error = %v, want %q", err, want)
			}
			sess, err := resolveSession(store, "claude-live", "")
			if err != nil {
				t.Fatal(err)
			}
			if sess.Status != "stopped" || sess.ProviderSessionID != "sess-live" {
				t.Fatalf("status=%q provider=%q, want stopped and sess-live", sess.Status, sess.ProviderSessionID)
			}
			if sess.ContextTokens != tc.wantObserved || sess.LastContextTokens() != tc.wantObserved {
				t.Fatalf("ContextTokens=%d LastContextTokens=%d, want %d so the resume gate compacts", sess.ContextTokens, sess.LastContextTokens(), tc.wantObserved)
			}
		})
	}
}

func TestClaudeWatchdogDoesNotStopBelowOrWithoutLiveUsage(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stream string
		config string
	}{
		{
			name:   "just under threshold",
			stream: claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_a", 99999, "null"), claudeLiveResult(99999)),
		},
		{
			// Repeated lines for one message id carry the same prompt; three
			// lines at 40k must not add up to a 120k crossing.
			name: "one message id repeated",
			stream: claudeLiveStream(claudeLiveInit(),
				claudeLiveAssistant("msg_rep", 40000, "null"),
				claudeLiveAssistant("msg_rep", 40000, "null"),
				claudeLiveAssistant("msg_rep", 40000, "null"),
				claudeLiveResult(40000)),
		},
		{
			// The result usage sums every call of the turn; it is not a live prompt size.
			name:   "no assistant usage, result usage 300k",
			stream: claudeLiveStream(claudeLiveInit(), claudeLiveAssistantNoUsage("msg_nu"), claudeLiveResult(300000)),
		},
		{
			// Sub-agent (Task) traffic has its own context and is not the main thread's.
			name:   "sub-agent call at 150k",
			stream: claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_sub", 150000, `"toolu_1"`), claudeLiveResult(150000)),
		},
		{
			name:   "threshold disabled by config 0",
			stream: claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null"), claudeLiveResult(134000)),
			config: "agent:\n  compact_thresholds:\n    claude:haiku:low: 0\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeClaudeLive(t, tc.stream, "")
			if tc.config != "" {
				dir := filepath.Join(os.Getenv("HOME"), ".harnez")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(tc.config), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			err = runStart(context.Background(), claudeLiveCmd(), toolLimitTestDeps(store), startRequest{
				Name: "claude-live", Role: "advisor", Prompt: "say ok", StoredPrompt: "say ok",
				ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull,
			})
			if err != nil {
				t.Fatalf("runStart stopped a turn that must run to completion: %v", err)
			}
			sess, err := resolveSession(store, "claude-live", "")
			if err != nil {
				t.Fatal(err)
			}
			if sess.Status != "completed" {
				t.Fatalf("status = %q, want completed", sess.Status)
			}
		})
	}
}

func TestClaudeWatchdogStopsResumeAtCrossing(t *testing.T) {
	fakeClaudeLive(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 134000, "null")), "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid-live-resume", Name: "claude-live-resume", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-live", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1}); err != nil {
		t.Fatal(err)
	}
	err = runResume(context.Background(), claudeLiveCmd(), toolLimitTestDeps(store), resumeRequest{Name: "claude-live-resume", Prompt: "continue", StreamMode: streamFull})
	want := "crossed live token threshold at 134000 tokens (limit 100000)"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("runResume error = %v, want %q", err, want)
	}
	sess, err := resolveSession(store, "claude-live-resume", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "stopped" || sess.LastContextTokens() != 134000 {
		t.Fatalf("status=%q LastContextTokens=%d, want stopped and 134000", sess.Status, sess.LastContextTokens())
	}
}
