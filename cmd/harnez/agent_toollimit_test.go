package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

func toolLimitTestDeps(store *subagent.FileSessionStore) agentDeps {
	now := time.Now().UTC()
	return agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return "" },
		quota:  func(context.Context, string, bool) usage.TurnQuotaReading { return usage.TurnQuotaReading{} },
		availability: func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{
				State: "available",
				Windows: []usagestore.Window{
					{Key: "five_hour", UsedFraction: 0.1, ObservedAt: now, Freshness: "fresh"},
					{Key: "weekly", UsedFraction: 0.1, ObservedAt: now, Freshness: "fresh"},
				},
			}
		},
		preflight: func(context.Context) error { return nil },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, name string) (*subagent.Session, error) {
			return resolveSession(s, name, "")
		},
	}
}

func setupFakeProviderEnv(t *testing.T, scripts map[string]string) (binDir, stateDir string) {
	t.Helper()
	root := t.TempDir()
	binDir = filepath.Join(root, "bin")
	stateDir = filepath.Join(root, "state")
	homeDir := filepath.Join(root, "home")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	t.Setenv("HARNEZ_LAUNCHER_PID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	for name, body := range scripts {
		p := filepath.Join(binDir, name)
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return binDir, stateDir
}

func blockForeverFIFO(fifoPath string) string {
	return fmt.Sprintf(`fifo=%q
rm -f "$fifo"
mkfifo "$fifo"
exec 3<>"$fifo"
cat <&3
`, fifoPath)
}

func TestToolCallLimitLiveCancelAndSynthesisCodex(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		name := "stream"
		if jsonMode {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			fifo := filepath.Join(t.TempDir(), "codex-block.fifo")
			script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
if printf '%%s\n' "$*" | grep -q 'resume'; then
  printf '{"type":"thread.started","thread_id":"th-codex-1"}\n'
  printf '{"type":"turn.started"}\n'
  printf '{"type":"item.completed","item":{"id":"msg-synth","type":"agent_message","text":"synthesized final answer"}}\n'
  printf '{"type":"turn.completed","usage":{"input_tokens":50,"cached_input_tokens":10,"output_tokens":12}}\n'
  exit 0
fi
printf '{"type":"thread.started","thread_id":"th-codex-1"}\n'
printf '{"type":"turn.started"}\n'
printf '{"type":"item.completed","item":{"id":"msg-1","type":"agent_message","text":"partial finding"}}\n'
printf '{"type":"item.started","item":{"id":"tc-1","type":"command_execution","command":"ls"}}\n'
printf '{"type":"item.completed","item":{"id":"tc-1","type":"command_execution","command":"ls"}}\n'
printf '{"type":"item.started","item":{"id":"tc-2","type":"mcp_tool_call"}}\n'
printf '{"type":"item.started","item":{"id":"tc-3","type":"file_change"}}\n'
%s`, blockForeverFIFO(fifo))

			setupFakeProviderEnv(t, map[string]string{"codex": script})
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			cmd := &cobra.Command{}
			cmd.Flags().Duration("timeout", 0, "")
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)

			err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
				Name:            "codex-synth",
				Role:            "advisor",
				Prompt:          "inspect repo",
				StoredPrompt:    "inspect repo",
				ModelSpec:       "codex:luna:low",
				Dir:             t.TempDir(),
				StreamMode:      streamStats,
				JSON:            jsonMode,
				MaxToolCalls:    2,
				MaxToolCallsSet: true,
			})
			if err != nil {
				t.Fatalf("runStart failed: %v (stderr=%s)", err, errOut.String())
			}

			sess, err := store.Get("th-codex-1")
			if err != nil {
				t.Fatal(err)
			}
			if sess.Status != "completed" {
				t.Fatalf("status = %q, want completed", sess.Status)
			}
			if len(sess.TurnRecords) != 1 || sess.TurnRecords[0].ToolCalls == nil || *sess.TurnRecords[0].ToolCalls != 3 || sess.TurnRecords[0].ToolCallLimit != 2 || !sess.TurnRecords[0].ToolCallLimitReached {
				t.Fatalf("unexpected TurnRecords: %+v", sess.TurnRecords)
			}

			if jsonMode {
				var payload struct {
					Response             string `json:"response"`
					ToolCalls            *int   `json:"tool_calls"`
					ToolCallLimit        int    `json:"tool_call_limit"`
					ToolCallLimitReached bool   `json:"tool_call_limit_reached"`
				}
				if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
					t.Fatalf("decode JSON: %v\nraw=%s", err, out.String())
				}
				if payload.Response != "synthesized final answer" || payload.ToolCalls == nil || *payload.ToolCalls != 3 || payload.ToolCallLimit != 2 || !payload.ToolCallLimitReached {
					t.Fatalf("unexpected JSON output: %+v", payload)
				}
			} else {
				got := out.String()
				for _, want := range []string{"[tool-call limit 2 reached; requesting final answer]", "synthesized final answer", "tool calls: 3/2 (limit reached)"} {
					if !strings.Contains(got, want) {
						t.Fatalf("stream output missing %q:\n%s", want, got)
					}
				}
			}
		})
	}
}

func TestToolCallLimitSynthesisNoAnswerClaude(t *testing.T) {
	tmp := t.TempDir()
	startFIFO := filepath.Join(tmp, "claude-start.fifo")
	synthFIFO := filepath.Join(tmp, "claude-synth.fifo")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
if printf '%%s\n' "$*" | grep -q -- '--resume'; then
  printf '{"type":"assistant","session_id":"cl-1","message":{"content":[{"type":"text","text":"unfinished synthesis note"},{"type":"tool_use","id":"toolu_4","name":"Read","input":{}}]}}\n'
  %s
  exit 0
fi
printf '{"type":"system","subtype":"init","session_id":"cl-1"}\n'
printf '{"type":"assistant","session_id":"cl-1","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{}},{"type":"tool_use","id":"toolu_2","name":"Read","input":{}},{"type":"tool_use","id":"toolu_3","name":"Grep","input":{}}]}}\n'
%s`, blockForeverFIFO(synthFIFO), blockForeverFIFO(startFIFO))

	setupFakeProviderEnv(t, map[string]string{"claude": script})
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := &cobra.Command{}
	cmd.Flags().Duration("timeout", 0, "")
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
		Name:            "claude-stop",
		Role:            "advisor",
		Prompt:          "check files",
		StoredPrompt:    "check files",
		ModelSpec:       "claude:sonnet:low",
		Dir:             t.TempDir(),
		StreamMode:      streamStats,
		JSON:            true,
		MaxToolCalls:    2,
		MaxToolCallsSet: true,
	})
	if err == nil {
		t.Fatal("expected error when synthesis produces no final answer")
	}

	sess, err := resolveSession(store, "claude-stop", "")
	if err != nil {
		t.Fatal(err)
	}
	if sess.ProviderSessionID != "cl-1" {
		t.Fatalf("ProviderSessionID = %q, want cl-1", sess.ProviderSessionID)
	}
	wantLastErr := "tool-call limit reached: 4 calls (limit 2), no answer; session can be resumed"
	if sess.Status != "stopped" || sess.LastError != wantLastErr || sess.Response != "" {
		t.Fatalf("unexpected stopped session: status=%q lastErr=%q response=%q", sess.Status, sess.LastError, sess.Response)
	}
	if len(sess.TurnRecords) != 1 || !sess.TurnRecords[0].ToolCallLimitReached || sess.TurnRecords[0].ToolCalls == nil || *sess.TurnRecords[0].ToolCalls != 4 {
		t.Fatalf("unexpected turn record: %+v", sess.TurnRecords)
	}
}

func TestToolCallLimitPiExtensionBackstopAndEmptyTerminated(t *testing.T) {
	t.Run("passes extension flag and handles empty terminated run", func(t *testing.T) {
		stateOut := t.TempDir()
		argsFile := filepath.Join(stateOut, "args.txt")
		envFile := filepath.Join(stateOut, "env.txt")
		script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
printf '%%s\n' "$@" > %q
printf 'LIMIT=%%s\nPROMPT=%%s\n' "${HARNEZ_TOOL_CALL_LIMIT:-}" "${HARNEZ_TOOL_CALL_LIMIT_PROMPT:-}" > %q
printf '{"type":"session","id":"pi-1"}\n'
printf '{"type":"tool_execution_start","toolCallId":"p1","toolName":"read"}\n'
printf '{"type":"tool_execution_start","toolCallId":"p2","toolName":"read"}\n'
printf '{"type":"tool_execution_start","toolCallId":"p3","toolName":"read"}\n'
printf '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"partial pi trace"}],"usage":{"input":10,"output":4,"cacheRead":0,"cacheWrite":0,"totalTokens":14}}}\n'
printf '{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"stop","usage":{"input":11,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":12}}}\n'
printf '{"type":"agent_settled"}\n'
`, argsFile, envFile)

		setupFakeProviderEnv(t, map[string]string{"pi": script})
		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		cmd := &cobra.Command{}
		cmd.Flags().Duration("timeout", 0, "")
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
			Name:            "pi-empty",
			Role:            "reviewer",
			Prompt:          "review",
			StoredPrompt:    "review",
			ModelSpec:       "lmcoder:coder:low",
			Dir:             t.TempDir(),
			StreamMode:      streamStats,
			JSON:            true,
			MaxToolCalls:    2,
			MaxToolCallsSet: true,
		})
		if err == nil {
			t.Fatal("expected error for empty terminated Pi run")
		}

		argsBytes, _ := os.ReadFile(argsFile)
		argsText := string(argsBytes)
		if !strings.Contains(argsText, "\n-e\n") {
			t.Fatalf("pi argv missing -e flag:\n%s", argsText)
		}
		envBytes, _ := os.ReadFile(envFile)
		if !strings.Contains(string(envBytes), "LIMIT=2\n") || !strings.Contains(string(envBytes), "PROMPT=Tool-call limit reached.") {
			t.Fatalf("unexpected pi env:\n%s", string(envBytes))
		}

		sess, err := resolveSession(store, "pi-empty", "")
		if err != nil {
			t.Fatal(err)
		}
		if sess.ProviderSessionID != "pi-1" {
			t.Fatalf("ProviderSessionID = %q, want pi-1", sess.ProviderSessionID)
		}
		wantErr := "tool-call limit reached: 3 calls (limit 2), no answer; session can be resumed"
		if sess.Status != "stopped" || sess.LastError != wantErr || sess.Response != "" || len(sess.Messages) != 1 || sess.Messages[0] != "partial pi trace" {
			t.Fatalf("unexpected pi stopped session: %+v", sess)
		}
	})

	t.Run("backstop stops pi without synthesis", func(t *testing.T) {
		countFile := filepath.Join(t.TempDir(), "calls.txt")
		fifo := filepath.Join(t.TempDir(), "pi-block.fifo")
		script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
printf 'call\n' >> %q
printf '{"type":"session","id":"pi-backstop"}\n'
printf '{"type":"tool_execution_start","toolCallId":"b1","toolName":"read"}\n'
printf '{"type":"tool_execution_start","toolCallId":"b2","toolName":"read"}\n'
printf '{"type":"tool_execution_start","toolCallId":"b3","toolName":"read"}\n'
printf '{"type":"tool_execution_start","toolCallId":"b4","toolName":"read"}\n'
%s`, countFile, blockForeverFIFO(fifo))

		setupFakeProviderEnv(t, map[string]string{"pi": script})
		home := os.Getenv("HOME")
		if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0o755); err != nil {
			t.Fatal(err)
		}
		cfg := "agent:\n  tool_call_limits:\n    native_backstop_calls: 2\n"
		if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		cmd := &cobra.Command{}
		cmd.Flags().Duration("timeout", 0, "")
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
			Name:            "pi-backstop",
			Role:            "reviewer",
			Prompt:          "review",
			StoredPrompt:    "review",
			ModelSpec:       "lmcoder:coder:low",
			Dir:             t.TempDir(),
			StreamMode:      streamStats,
			JSON:            true,
			MaxToolCalls:    1,
			MaxToolCallsSet: true,
		})
		if err == nil {
			t.Fatal("expected error when Pi backstop triggers")
		}
		rawCount, _ := os.ReadFile(countFile)
		if got := len(strings.Split(strings.TrimSpace(string(rawCount)), "\n")); got != 1 {
			t.Fatalf("expected 1 pi invocation (no synthesis), got %d", got)
		}
		sess, err := resolveSession(store, "pi-backstop", "")
		if err != nil {
			t.Fatal(err)
		}
		wantErr := "tool-call limit reached: 4 calls (limit 1), no answer; session can be resumed"
		if sess.Status != "stopped" || sess.LastError != wantErr {
			t.Fatalf("unexpected backstop session: status=%q lastErr=%q", sess.Status, sess.LastError)
		}
	})

	t.Run("limit zero passes unchanged args and omits tool_calls", func(t *testing.T) {
		argsFile := filepath.Join(t.TempDir(), "args.txt")
		script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
printf '%%s\n' "$@" > %q
printf '{"type":"session","id":"pi-zero"}\n'
printf '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input":10,"output":2,"cacheRead":0,"cacheWrite":0,"totalTokens":12}}}\n'
printf '{"type":"agent_settled"}\n'
`, argsFile)
		setupFakeProviderEnv(t, map[string]string{"pi": script})
		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.Flags().Duration("timeout", 0, "")
		cmd.SetOut(&out)
		cmd.SetErr(new(bytes.Buffer))

		err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
			Name:            "pi-zero",
			Role:            "reviewer",
			Prompt:          "review",
			StoredPrompt:    "review",
			ModelSpec:       "lmcoder:coder:low",
			Dir:             t.TempDir(),
			StreamMode:      streamStats,
			JSON:            true,
			MaxToolCalls:    0,
			MaxToolCallsSet: true,
		})
		if err != nil {
			t.Fatalf("runStart failed: %v", err)
		}
		argsBytes, _ := os.ReadFile(argsFile)
		if strings.Contains(string(argsBytes), "\n-e\n") {
			t.Fatalf("unexpected -e flag when limit=0:\n%s", string(argsBytes))
		}
		if strings.Contains(out.String(), `"tool_calls"`) || strings.Contains(out.String(), `"tool_call_limit"`) {
			t.Fatalf("unexpected tool_call fields in JSON when limit=0: %s", out.String())
		}
	})
}

func TestToolCallLimitFlagsWorkersAndAgy(t *testing.T) {
	t.Run("interactive rejects max-tool-calls", func(t *testing.T) {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"start", "-i", "--max-tool-calls", "5", "--model", "codex:luna:low", "hi"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--max-tool-calls") {
			t.Fatalf("expected --max-tool-calls interactive rejection error, got %v", err)
		}
	})

	t.Run("detached worker forwards max-tool-calls", func(t *testing.T) {
		dir := t.TempDir()
		storeDir := t.TempDir()
		argsFile := filepath.Join(dir, "worker-args.txt")
		fakeExe := filepath.Join(dir, "harnez")
		script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
printf '%%s\n' "$@" > %q
sid=""
for ((i=1; i<=$#; i++)); do
  if [ "${!i}" = "--worker-session" ]; then
    j=$((i+1))
    sid="${!j}"
  fi
done
python3 - "$sid" %q <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[2]) / (sys.argv[1] + ".json")
data = json.loads(p.read_text())
data["status"] = "completed"
data["response"] = "done"
p.write_text(json.dumps(data))
PY
`, argsFile, storeDir)
		if err := os.WriteFile(fakeExe, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		oldExe := agentExecutable
		agentExecutable = func() (string, error) { return fakeExe, nil }
		defer func() { agentExecutable = oldExe }()

		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		cmd.SetErr(new(bytes.Buffer))
		if err := launchDetachedWithPreflight(cmd, startRequest{
			Name:            "w-det",
			Prompt:          "task",
			StoredPrompt:    "task",
			ModelSpec:       "codex:luna:low",
			Dir:             t.TempDir(),
			JSON:            true,
			MaxToolCalls:    0,
			MaxToolCallsSet: true,
		}, storeDir, "", func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		var created subagent.Session
		if err := json.Unmarshal(out.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		store, err := subagent.NewSessionStore(storeDir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := waitForAgent(context.Background(), store, created.ID, 2*time.Second); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "--max-tool-calls\n0\n") {
			t.Fatalf("worker args missing --max-tool-calls 0:\n%s", string(b))
		}
	})

	t.Run("resume enforces max-tool-calls and records counts", func(t *testing.T) {
		fifo := filepath.Join(t.TempDir(), "codex-resume.fifo")
		script := fmt.Sprintf(`#!/usr/bin/env bash
set -eu
if printf '%%s\n' "$*" | grep -q 'synthesis'; then
  printf '{"type":"thread.started","thread_id":"th-res-1"}\n'
  printf '{"type":"turn.started"}\n'
  printf '{"type":"item.completed","item":{"id":"msg-s","type":"agent_message","text":"resumed synthesis answer"}}\n'
  printf '{"type":"turn.completed","usage":{"input_tokens":70,"cached_input_tokens":20,"output_tokens":15}}\n'
  exit 0
fi
printf '{"type":"thread.started","thread_id":"th-res-1"}\n'
printf '{"type":"turn.started"}\n'
printf '{"type":"item.started","item":{"id":"r1","type":"command_execution","command":"ls"}}\n'
printf '{"type":"item.started","item":{"id":"r2","type":"web_search"}}\n'
%s`, blockForeverFIFO(fifo))
		setupFakeProviderEnv(t, map[string]string{"codex": script})

		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		if err := store.Create(&subagent.Session{
			ID:                "th-res-1",
			ProviderSessionID: "th-res-1",
			Name:              "res-worker",
			Role:              "advisor",
			Provider:          "codex",
			Model:             "gpt-6-luna",
			Tier:              "low",
			WorkingDir:        t.TempDir(),
			HarnessType:       "harnez",
			Status:            "completed",
			Turn:              1,
			CreatedAt:         now,
			LastActiveAt:      now,
		}); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		cmd := &cobra.Command{}
		cmd.Flags().Duration("timeout", 0, "")
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)

		// Override synthesis_prompt with a distinct marker so fake codex distinguishes initial resume vs synthesis resume.
		home := os.Getenv("HOME")
		_ = os.MkdirAll(filepath.Join(home, ".harnez"), 0o755)
		_ = os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte("agent:\n  tool_call_limits:\n    synthesis_prompt: \"synthesis final now\"\n"), 0o600)

		err = runResume(context.Background(), cmd, toolLimitTestDeps(store), resumeRequest{
			Name:            "res-worker",
			Prompt:          "continue",
			StreamMode:      streamStats,
			JSON:            true,
			MaxToolCalls:    1,
			MaxToolCallsSet: true,
		})
		if err != nil {
			t.Fatalf("runResume failed: %v (stderr=%s)", err, errOut.String())
		}
		var payload struct {
			Response             string `json:"response"`
			ToolCalls            *int   `json:"tool_calls"`
			ToolCallLimit        int    `json:"tool_call_limit"`
			ToolCallLimitReached bool   `json:"tool_call_limit_reached"`
		}
		if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
			t.Fatalf("decode JSON: %v\nraw=%s", err, out.String())
		}
		if payload.Response != "resumed synthesis answer" || payload.ToolCalls == nil || *payload.ToolCalls != 2 || payload.ToolCallLimit != 1 || !payload.ToolCallLimitReached {
			t.Fatalf("unexpected resume JSON output: %+v", payload)
		}
	})

	t.Run("agy warns and omits tool_calls", func(t *testing.T) {
		oldDriver := agentDriver
		agentDriver = func(subagent.Model, string) subagent.Driver { return &stubCostDriver{} }
		defer func() { agentDriver = oldDriver }()

		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		cmd := &cobra.Command{}
		cmd.Flags().Duration("timeout", 0, "")
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)

		err = runStart(context.Background(), cmd, toolLimitTestDeps(store), startRequest{
			Name:            "agy-warn",
			Role:            "advisor",
			Prompt:          "check",
			StoredPrompt:    "check",
			ModelSpec:       "agy:argon:low",
			Dir:             t.TempDir(),
			StreamMode:      streamStats,
			JSON:            true,
			MaxToolCalls:    3,
			MaxToolCallsSet: true,
		})
		if err != nil {
			t.Fatalf("runStart failed: %v", err)
		}
		if !strings.Contains(errOut.String(), "does not enforce --max-tool-calls") {
			t.Fatalf("expected agy warning on stderr, got %q", errOut.String())
		}
		if strings.Contains(out.String(), `"tool_calls"`) {
			t.Fatalf("expected agy JSON output to omit tool_calls: %s", out.String())
		}
	})
}
