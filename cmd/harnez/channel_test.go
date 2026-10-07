package main

import (
	"bytes"
	"encoding/json"
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

	launch := "claude --model " + cfg.CanaryModel + " --dangerously-load-development-channels server:" + cfg.ServerName
	appendPrompt := "--append-system-prompt "
	if !strings.Contains(text, launch) || !strings.Contains(text, appendPrompt) || !strings.Contains(text, cfg.RegistryDir) {
		t.Fatalf("missing manual launch, append-system-prompt or registry path: %s", text)
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
