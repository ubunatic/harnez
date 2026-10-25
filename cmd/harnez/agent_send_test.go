package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/channel"
	"ubunatic.com/harnez/internal/subagent"
)

func TestChannelNameResolution(t *testing.T) {
	endpoints := []channel.Endpoint{{SessionID: "a", WorkingDir: "/a"}, {SessionID: "b", WorkingDir: "/b"}}
	identities := []channelIdentity{{"harnez-id", "a", "worker"}, {"a", "a", "claude-a"}, {"duplicate", "a", "worker"}, {"b", "b", "shared"}, {"a", "a", "shared"}, {"offline", "offline", "offline"}}
	for _, tc := range []struct{ name, id, dir, want, failure string }{
		{"worker", "", "", "a", ""}, {"claude-a", "", "", "a", ""}, {"", "b", "", "b", ""},
		{"shared", "", "/b", "b", ""}, {"shared", "", "", "", "ambiguous"}, {"missing", "", "", "", "unknown"}, {"offline", "", "", "", "unknown"},
	} {
		t.Run(tc.name+tc.id+tc.dir, func(t *testing.T) {
			endpoint, err := resolveChannelTarget(endpoints, identities, tc.name, tc.id, tc.dir)
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) || !strings.Contains(err.Error(), "candidates:") || !strings.Contains(err.Error(), "id=a") || !strings.Contains(err.Error(), "id=b") {
					t.Fatalf("candidates missing: %v", err)
				}
			} else if err != nil || endpoint.SessionID != tc.want {
				t.Fatalf("resolved %#v, %v", endpoint, err)
			}
		})
	}
}

func TestSendWarnsWhenCurrentPolicyDisablesStoredSession(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "registry-id", ProviderSessionID: "provider-id", Provider: "claude", Model: "opus", Tier: "med", Name: "worker"}); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	stderr := new(bytes.Buffer)
	cmd.SetErr(stderr)
	target := agentSendTarget{Provider: "claude", Endpoint: channel.Endpoint{SessionID: "provider-id"}}
	check := func(string, string, string) (bool, error) { return false, nil }
	if err := warnStoredSendModel(cmd, storeDir, target, check); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "disabled by current policy") || !strings.Contains(stderr.String(), "claude:opus:med") {
		t.Fatalf("send warning = %q", stderr.String())
	}
}

func TestDefensiveClaudeMetadataAndSender(t *testing.T) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.ClaudeMetadataDir = t.TempDir()
	cfg.MaxMetadataBytes = 100
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "registry-host", ProviderSessionID: "claude-host", Name: "harnez-host", Provider: "claude"}); err != nil {
		t.Fatal(err)
	}
	for file, data := range map[string]string{
		"valid.json": `{"sessionId":"target","name":"target-name","future":"ignored"}`,
		"bad.json":   `{`, "wrong.json": `{"sessionId":42,"name":"bad"}`, "empty.json": `{}`,
		"oversize.json": strings.Repeat("x", 101), "trailing.json": `{"sessionId":"bad","name":"bad"} {}`,
	} {
		if err := os.WriteFile(filepath.Join(cfg.ClaudeMetadataDir, file), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	identities := channelIdentities(cfg, storeDir)
	if len(identities) != 2 {
		t.Fatalf("metadata did not skip bad files: %#v", identities)
	}
	if sender := channelSender(cfg, identities, "registry-host"); !strings.Contains(sender, "harnez-host (registry-host) via harnez agent send") {
		t.Fatal(sender)
	}
	if sender := channelSender(cfg, nil, ""); !strings.Contains(sender, cfg.TerminalSender) {
		t.Fatal(sender)
	}
	if sender := channelSender(cfg, []channelIdentity{{"x", "x", "line\nbreak"}}, "x"); strings.Contains(sender, "\n") {
		t.Fatal("sender framing spans lines")
	}
	cfg.ClaudeMetadataDir = filepath.Join(cfg.ClaudeMetadataDir, "missing")
	if len(channelIdentities(cfg, storeDir)) != 1 {
		t.Fatal("missing metadata prevented Harnez lookup")
	}
}

func TestAgentSendLeafAndValidation(t *testing.T) {
	t.Setenv(agentRoleEnv, "developer")
	if err := guardLeafRole("send"); err != nil {
		t.Fatalf("leaf cannot send: %v", err)
	}
	if err := guardLeafRole("start"); err == nil {
		t.Fatal("leaf can spawn")
	}
	for _, args := range [][]string{{"send"}, {"send", "--id", "id"}, {"send", "--id", "id", "msg", "-m", "also"}, {"send", "--id", "id", "-m", "a", "-p", "b"}, {"send", "--id", "id", "--name", "also", "msg"}, {"send", "--id", "id", "msg", "--timeout", "1s"}, {"send", "--id", "id", "msg", "--wait", "--timeout", "0s"}} {
		cmd := newAgentCmd()
		cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error { return guardLeafRole(c.Name()) }
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
}

func TestAgentSendAcceptedAndOptionalWait(t *testing.T) {
	for _, mode := range []string{"default", "received", "timeout", "old-server", "flag-m", "flag-p", "by-id", "no-target"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
			t.Setenv("HARNEZ_SESSION_ID", "host")
			t.Setenv(agentRoleEnv, "developer")
			cfg, err := channel.LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			var endpoint channel.Endpoint
			var delivered string
			receiptRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing authentication")
				}
				switch r.URL.Path {
				case "/health":
					_ = json.NewEncoder(w).Encode(endpoint)
				case "/message":
					var message struct {
						SessionID string `json:"session_id"`
						Content   string `json:"content"`
					}
					if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
						t.Error(err)
					}
					delivered = message.Content
					if message.SessionID != "target" {
						t.Error("wrong target")
					}
					w.WriteHeader(http.StatusAccepted)
					_ = json.NewEncoder(w).Encode(channel.Accepted{Status: "accepted", Receipt: "unconfirmed", SessionID: "target", MessageID: "message"})
				case "/receipt":
					receiptRequests++
					if mode == "old-server" {
						http.NotFound(w, r)
						return
					}
					status := "unconfirmed"
					if mode == "received" {
						status = "received"
					}
					_ = json.NewEncoder(w).Encode(channel.Receipt{Status: status, SessionID: "target", MessageID: "message", Text: "optional reply"})
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			endpoint = channel.Endpoint{SessionID: "target", PID: os.Getpid(), URL: server.URL, WorkingDir: t.TempDir()}
			if err := os.MkdirAll(cfg.RegistryDir, 0700); err != nil {
				t.Fatal(err)
			}
			reg, _ := json.Marshal(map[string]any{"session_id": endpoint.SessionID, "pid": endpoint.PID, "url": endpoint.URL, "working_dir": endpoint.WorkingDir, "token": "test-token"})
			if err := os.WriteFile(filepath.Join(cfg.RegistryDir, fmt.Sprintf("%x.json", sha256.Sum256([]byte("target")))), reg, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(cfg.ClaudeMetadataDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cfg.ClaudeMetadataDir, "target.json"), []byte(`{"sessionId":"target","name":"target-name"}`), 0600); err != nil {
				t.Fatal(err)
			}
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(&subagent.Session{ID: "registry-worker", Name: "target-name", Provider: "codex", Status: "running", HarnessType: "harnez"}); err != nil {
				t.Fatal(err)
			}
			cmd := newAgentCmd()
			cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error { return guardLeafRole(c.Name()) }
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(new(bytes.Buffer))
			args := []string{"send", "--name", "target-name", "message", "body"}
			switch mode {
			case "flag-m", "flag-p":
				args = []string{"send", "--name", "target-name", "-" + mode[5:], "message body"}
			case "by-id":
				args = []string{"send", "--id", "target", "message body"}
			case "no-target":
				args = []string{"send", "message body"}
			case "received", "timeout", "old-server":
				args = append(args, "--wait", "--timeout", "25ms")
			}
			cmd.SetArgs(append(args, "--store-dir", storeDir))
			err = cmd.ExecuteContext(context.Background())
			worker, getErr := store.Get("registry-worker")
			if getErr != nil || len(worker.PendingMessages) != 0 {
				t.Fatalf("live send used registry fallback: %#v, %v", worker, getErr)
			}
			if mode == "no-target" {
				if err == nil || !strings.Contains(err.Error(), "live sessions:\n  target-name\n") || strings.Contains(err.Error(), "  target\n") || delivered != "" {
					t.Fatalf("no-target error: %v (delivered %q)", err, delivered)
				}
				return
			}
			if mode == "old-server" {
				if err == nil || !strings.Contains(err.Error(), "restart") {
					t.Fatalf("old-server error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(delivered, "From host (host) via harnez agent send\n\nmessage body") {
				t.Fatalf("sender missing: %q", delivered)
			}
			if !strings.Contains(out.String(), `"status":"accepted"`) {
				t.Fatal(out.String())
			}
			if mode != "received" && mode != "timeout" && mode != "old-server" && (receiptRequests != 0 || strings.Count(out.String(), "\n") != 1) {
				t.Fatal("default send waited")
			}
			if mode == "received" && !strings.Contains(out.String(), `"status":"received"`) {
				t.Fatal(out.String())
			}
			if (mode == "timeout" || mode == "old-server") && !strings.Contains(out.String(), `"status":"unconfirmed"`) {
				t.Fatal(out.String())
			}
		})
	}
}

func TestCodexAgentSendTargetResolution(t *testing.T) {
	endpoints := []channel.Endpoint{{SessionID: "claude-id", WorkingDir: "/work/claude"}}
	identities := []channelIdentity{{ID: "claude-id", ProviderID: "claude-id", Name: "cati2"}}
	threads := []channel.CodexThread{{ID: "thread-uuid", Name: "cati1", CWD: "/work/codex"}}
	for _, tc := range []struct {
		name, id string
	}{
		{name: "cati1"},
		{id: "thread-uuid"},
	} {
		target, err := resolveAgentSendTarget(endpoints, identities, threads, nil, tc.name, tc.id, "")
		if err != nil || target.Provider != "codex" || target.Endpoint.SessionID != "thread-uuid" {
			t.Fatalf("target for name=%q id=%q = %#v, %v", tc.name, tc.id, target, err)
		}
	}
	if names := liveAgentNames(endpoints, identities, threads, nil, ""); strings.Join(names, ",") != "cati1,cati2" {
		t.Fatalf("live names = %#v", names)
	}
	_, err := resolveAgentSendTarget(endpoints, identities, threads, nil, "missing", "", "")
	if err == nil || !strings.Contains(err.Error(), `"cati1" id=thread-uuid provider=codex`) || !strings.Contains(err.Error(), `"cati2" id=claude-id provider=claude`) {
		t.Fatalf("candidate error omitted a provider: %v", err)
	}
}

func TestAgyAgentSendTargetResolution(t *testing.T) {
	endpoints := []channel.Endpoint{{SessionID: "claude-id", WorkingDir: "/work"}}
	identities := []channelIdentity{{ID: "claude-id", ProviderID: "claude-id", Name: "shared"}}
	threads := []channel.CodexThread{}
	agy := []*subagent.Session{
		{ID: "agy-id", ProviderSessionID: "agy-conversation", Name: "agy-worker", Provider: "agy", WorkingDir: "/work", ProviderPID: os.Getpid(), ControlSocket: "/tmp/agy.sock"},
		{ID: "agy-shared", Name: "shared", Provider: "agy", WorkingDir: "/work", ProviderPID: os.Getpid(), ControlSocket: "/tmp/shared.sock"},
	}
	for _, tc := range []struct{ name, id, want string }{{name: "agy-worker", want: "agy-id"}, {id: "agy-id", want: "agy-id"}} {
		target, err := resolveAgentSendTarget(endpoints, identities, threads, agy, tc.name, tc.id, "")
		if err != nil || target.Provider != "agy" || target.Endpoint.SessionID != tc.want {
			t.Fatalf("target = %#v, %v", target, err)
		}
	}
	_, err := resolveAgentSendTarget(endpoints, identities, threads, agy, "shared", "", "")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "provider=agy") {
		t.Fatalf("ambiguous AGY name error = %v", err)
	}
	if names := liveAgentNames(endpoints, identities, threads, agy, ""); strings.Join(names, ",") != "agy-worker,shared" {
		t.Fatalf("live names = %#v", names)
	}
}

func TestSendAgyMessageQueuesControlRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agy.sock")
	sessionID := "agy-session-id"
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	got := make(chan subagent.ControlRequest, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var request subagent.ControlRequest
		_ = json.NewDecoder(conn).Decode(&request)
		got <- request
		_ = json.NewEncoder(conn).Encode(subagent.ControlResponse{MessageID: "msg-123"})
	}()
	var output bytes.Buffer
	if err := sendAgyMessage(context.Background(), sessionID, path, "framed body", &output); err != nil {
		t.Fatal(err)
	}
	if request := <-got; request.Action != "enqueue" || request.Prompt != "framed body" {
		t.Fatalf("request = %#v", request)
	}
	var result map[string]string
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "queued" || result["provider"] != "agy" || result["session_id"] != sessionID || result["message_id"] != "msg-123" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAgentSendAgyWaitAndUnsupervisedErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
	t.Setenv(agentRoleEnv, "developer")
	for _, tc := range []struct {
		name, args, want string
		live             bool
	}{
		{name: "wait", args: "--wait", want: "--wait is supported only for Claude sessions", live: true},
		{name: "unsupervised", args: "", want: "not supervised; use harnez-agy --meter", live: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			session := &subagent.Session{ID: "agy-test-id", Name: "agy-test", Provider: "agy", WorkingDir: "."}
			if tc.live {
				socket := filepath.Join(t.TempDir(), "agy.sock")
				listener, err := net.Listen("unix", socket)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				session.ControlSocket = socket
				session.ProcessPID = os.Getpid()
			}
			if err := store.Save(session); err != nil {
				t.Fatal(err)
			}
			cmd := newAgentCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			args := []string{"--store-dir", storeDir, "send", "--id", "agy-test-id", "message"}
			if tc.args != "" {
				args = append(args, tc.args)
			}
			cmd.SetArgs(args)
			err = cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCodexAgentSendFramesAndQueuesMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake queue executable uses a POSIX shell")
	}
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	threads := []channel.CodexThread{
		{ID: "sender-uuid", Name: "host-session"},
		{ID: "thread-uuid", Name: "cati1"},
	}
	frame := codexMessageFrame(cfg, nil, threads, "sender-uuid", "Please review this change.")
	for _, want := range []string{
		"From host-session (sender-uuid) via harnez agent send",
		"Please review this change.",
		"harnez agent send --id sender-uuid \"<message>\"",
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("framed message missing %q: %s", want, frame)
		}
	}
	tempDir := t.TempDir()
	executable := filepath.Join(tempDir, "codex")
	argsPath := filepath.Join(tempDir, "args")
	t.Setenv("CODEX_QUEUE_ARGS", argsPath)
	script := "#!/bin/sh\nif test \"$1\" != queue\nthen exit 10\nfi\nprintf '%s\\0' \"$@\" > \"$CODEX_QUEUE_ARGS\"\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := sendCodexMessage(context.Background(), executable, "thread-uuid", frame, &output); err != nil {
		t.Fatal(err)
	}
	gotArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"queue", "--thread", "thread-uuid", "--message", frame, ""}
	if strings.Join(strings.Split(string(gotArgs), "\x00"), "\n") != strings.Join(wantArgs, "\n") {
		t.Fatalf("queue argv = %#v", strings.Split(string(gotArgs), "\x00"))
	}
	var result struct {
		Status    string `json:"status"`
		Provider  string `json:"provider"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || result.Provider != "codex" || result.SessionID != "thread-uuid" {
		t.Fatalf("queue result = %#v", result)
	}
}
