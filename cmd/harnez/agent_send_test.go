package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	for _, args := range [][]string{{"send"}, {"send", "id"}, {"send", "id", "--name", "also", "-m", "msg"}, {"send", "id", "-m", "msg", "--timeout", "1s"}, {"send", "id", "-m", "msg", "--wait", "--timeout", "0s"}} {
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
	for _, mode := range []string{"default", "received", "timeout", "old-server"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
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
			cmd := newAgentCmd()
			cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error { return guardLeafRole(c.Name()) }
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(new(bytes.Buffer))
			args := []string{"send", "--name", "target-name", "-m", "message body"}
			if mode != "default" {
				args = append(args, "--wait", "--timeout", "25ms")
			}
			cmd.SetArgs(args)
			err = cmd.ExecuteContext(context.Background())
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
			if mode == "default" && (receiptRequests != 0 || strings.Count(out.String(), "\n") != 1) {
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
