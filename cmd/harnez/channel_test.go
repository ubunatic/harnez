package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/channel"
)

func TestChannelSetupPrintsManualConfiguration(t *testing.T) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	executable := "/a path with spaces/harnez"
	if err := printChannelSetup(&output, cfg, executable); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	start := strings.Index(text, "{\n")
	end := strings.Index(text, "\n\nOptional (for unattended receipts")
	if start < 0 || end < start {
		t.Fatalf("missing MCP snippet: %s", text)
	}
	var config struct {
		Servers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(text[start:end]), &config); err != nil {
		t.Fatal(err)
	}
	server := config.Servers[cfg.ServerName]
	if server.Type != "stdio" || server.Command != executable || len(server.Args) != 1 || server.Args[0] != "channel" {
		t.Fatalf("incorrect subprocess configuration: %#v", server)
	}

	settingsStart := strings.Index(text, "{\n  \"permissions\":")
	settingsEnd := strings.Index(text, "\n\nIn that directory")
	if settingsStart < 0 || settingsEnd < settingsStart {
		t.Fatalf("missing settings snippet: %s", text)
	}
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(text[settingsStart:settingsEnd]), &settings); err != nil {
		t.Fatal(err)
	}
	expectedPermission := "mcp__" + cfg.ServerName + "__" + cfg.ReplyToolName
	if len(settings.Permissions.Allow) != 1 || settings.Permissions.Allow[0] != expectedPermission {
		t.Fatalf("wrong permissions allow entry: %#v (expected %q)", settings.Permissions.Allow, expectedPermission)
	}

	expectedLaunch := `claude --model ` + cfg.CanaryModel + ` --dangerously-load-development-channels server:` + cfg.ServerName + ` --append-system-prompt "`
	if !strings.Contains(text, expectedLaunch) || !strings.Contains(text, cfg.RegistryDir) {
		t.Fatalf("missing manual launch with append-system-prompt or registry path: %s", text)
	}
}

func TestChannelServeRequiresHostSessionID(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	cmd := newChannelCmd()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "CLAUDE_CODE_SESSION_ID") {
		t.Fatalf("server accepted missing host identity: %v", err)
	}
}

func TestChannelHelpExplainsProjectApprovalAndRestart(t *testing.T) {
	cmd := newChannelCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"New MCP server found in this project", "Use this MCP server", "restart", "reply tool"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("missing help %q: %s", text, out.String())
		}
	}
}

func TestMergeChannelSessionsAndCodexFailureFallback(t *testing.T) {
	endpoints := []channel.Endpoint{
		{SessionID: "claude-2", PID: 22, URL: "http://127.0.0.1:2", WorkingDir: "/work/claude"},
		{SessionID: "claude-1", PID: 11, URL: "http://127.0.0.1:1", WorkingDir: "/work/claude"},
	}
	listed := mergeChannelSessions(endpoints, func() ([]channel.CodexThread, error) {
		return []channel.CodexThread{
			{ID: "thread-2", Name: "cati1", CWD: "/work/codex"},
			{ID: "thread-1", Name: "", CWD: "/work/other"},
		}, nil
	})
	if len(listed) != 4 || listed[0].Provider != "claude" || listed[0].SessionID != "claude-1" || listed[1].SessionID != "claude-2" || listed[2].Provider != "codex" || listed[2].SessionID != "thread-1" || listed[3].Name != "cati1" {
		t.Fatalf("merged sessions = %#v", listed)
	}
	listed = mergeChannelSessions(endpoints[:1], func() ([]channel.CodexThread, error) {
		return nil, errors.New("codex app-server unavailable")
	})
	if len(listed) != 1 || listed[0].Provider != "claude" || listed[0].SessionID != "claude-2" {
		t.Fatalf("Codex failure lost Claude sessions: %#v", listed)
	}
}
