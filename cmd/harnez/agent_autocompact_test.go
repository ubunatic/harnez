package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
)

// autocompactClaude runs a fake claude that writes the window it received to
// the returned path, prints stream, then runs tail.
func autocompactClaude(t *testing.T, stream, tail string) string {
	t.Helper()
	dir := t.TempDir()
	streamPath := filepath.Join(dir, "stream.jsonl")
	if err := os.WriteFile(streamPath, []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(dir, "window.txt")
	setupFakeProviderEnv(t, map[string]string{"claude": "#!/bin/sh\nprintf '%s' \"$CLAUDE_CODE_AUTO_COMPACT_WINDOW\" > " + envPath + "\ncat " + streamPath + "\n" + tail + "\n"})
	return envPath
}

func readWindow(t *testing.T, envPath string) string {
	t.Helper()
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func autocompactFlagCmd(args ...string) (*cobra.Command, error) {
	cmd := &cobra.Command{}
	cmd.Flags().String("autocompact", "", "")
	if err := cmd.ParseFlags(args); err != nil {
		return nil, err
	}
	return cmd, nil
}

func TestAutocompactFlagValues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantSet bool
		want    int
		wantErr bool
	}{
		{name: "not passed", args: nil},
		{name: "auto", args: []string{"--autocompact", "auto"}, wantSet: true, want: 0},
		{name: "token count", args: []string{"--autocompact", "150000"}, wantSet: true, want: 150000},
		{name: "k suffix", args: []string{"--autocompact", "120k"}, wantSet: true, want: 120000},
		{name: "invalid", args: []string{"--autocompact", "120K"}, wantErr: true},
		{name: "zero", args: []string{"--autocompact", "0"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := autocompactFlagCmd(tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			set, tokens, err := autocompactFlag(cmd, cmd.Flags().Lookup("autocompact").Value.String())
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if set != tc.wantSet || tokens != tc.want {
				t.Fatalf("set=%v tokens=%d, want set=%v tokens=%d", set, tokens, tc.wantSet, tc.want)
			}
		})
	}
}

func TestCheckAutocompactProvider(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		if err := checkAutocompactProvider(provider); err != nil {
			t.Errorf("checkAutocompactProvider(%q) = %v, want nil", provider, err)
		}
	}
	for _, provider := range []string{"agy", "jules", "pi", "lmcoder"} {
		err := checkAutocompactProvider(provider)
		if err == nil || !strings.Contains(err.Error(), "claude and codex only") {
			t.Errorf("checkAutocompactProvider(%q) = %v, want a claude-and-codex-only error", provider, err)
		}
	}
}

// TestAutocompactStartAcceptance is the M2 acceptance check: --autocompact 120k
// gives the child CLAUDE_CODE_AUTO_COMPACT_WINDOW=120000 over a caller value,
// and the watchdog stops the turn at 120k.
func TestAutocompactStartAcceptance(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "50000")
	envPath := autocompactClaude(t, claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 120000, "null")), "sleep 5")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = runStart(context.Background(), claudeLiveCmd(), toolLimitTestDeps(store), startRequest{
		Name: "claude-auto", Role: "advisor", Prompt: "say ok", StoredPrompt: "say ok",
		ModelSpec: "claude:haiku:low", Dir: t.TempDir(), StreamMode: streamFull,
		AutocompactSet: true, AutocompactTokens: 120000,
	})
	want := "crossed live token threshold at 120000 tokens (limit 120000)"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("runStart error = %v, want %q", err, want)
	}
	if got := readWindow(t, envPath); got != "120000" {
		t.Fatalf("child CLAUDE_CODE_AUTO_COMPACT_WINDOW = %q, want 120000", got)
	}
	sess, err := resolveSession(store, "claude-auto", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.CompactTokens != 120000 {
		t.Fatalf("stored CompactTokens = %d, want 120000", sess.CompactTokens)
	}
}

// TestAutocompactResumePersisted covers the stored value on resume: reused
// without the flag, replaced by it, and cleared by auto.
func TestAutocompactResumePersisted(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stream     string
		tail       string
		req        resumeRequest
		wantStored int
		wantWindow string
		wantErr    string
	}{
		{
			name:       "stored value reused without flag",
			stream:     claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_live", 120000, "null")),
			tail:       "sleep 5",
			req:        resumeRequest{},
			wantStored: 120000,
			wantWindow: "120000",
			wantErr:    "crossed live token threshold at 120000 tokens (limit 120000)",
		},
		{
			name:       "flag replaces stored value",
			stream:     claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_small", 1000, "null"), claudeLiveResult(1000)),
			req:        resumeRequest{AutocompactSet: true, AutocompactTokens: 150000},
			wantStored: 150000,
			wantWindow: "150000",
		},
		{
			name:       "auto clears stored value",
			stream:     claudeLiveStream(claudeLiveInit(), claudeLiveAssistant("msg_small", 1000, "null"), claudeLiveResult(1000)),
			req:        resumeRequest{AutocompactSet: true, AutocompactTokens: 0},
			wantStored: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envPath := autocompactClaude(t, tc.stream, tc.tail)
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(&subagent.Session{ID: "sid-auto-resume", Name: "claude-auto-resume", Provider: "claude", Model: "haiku", Tier: "low", Status: "completed", ProviderSessionID: "sess-live", WorkingDir: t.TempDir(), Role: "advisor", HarnessType: "harnez", Turn: 1, CompactTokens: 120000}); err != nil {
				t.Fatal(err)
			}
			req := tc.req
			req.Name, req.Prompt, req.StreamMode = "claude-auto-resume", "continue", streamFull
			err = runResume(context.Background(), claudeLiveCmd(), toolLimitTestDeps(store), req)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("runResume error = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("runResume: %v", err)
			}
			sess, err := resolveSession(store, "claude-auto-resume", "")
			if err != nil {
				t.Fatal(err)
			}
			if sess.CompactTokens != tc.wantStored {
				t.Fatalf("stored CompactTokens = %d, want %d", sess.CompactTokens, tc.wantStored)
			}
			if tc.wantWindow != "" {
				if got := readWindow(t, envPath); got != tc.wantWindow {
					t.Fatalf("child window = %q, want %s", got, tc.wantWindow)
				}
			}
		})
	}
}
