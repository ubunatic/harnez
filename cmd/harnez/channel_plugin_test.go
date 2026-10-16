package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/channel"
	"ubunatic.com/harnez/internal/claude"
)

func TestChannelSetupPluginRoute(t *testing.T) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := printChannelSetup(&out, cfg, "/usr/bin/harnez"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Run harnez apply",
		fmt.Sprintf("claude --channels plugin:harnez-channel@harnez-local --append-system-prompt %q", cfg.Instructions),
		`{"permissions":{"allow":["mcp__plugin_harnez-channel_harnez-channel__reply"]}}`,
		"harnez on PATH", "fresh session", "allowlisted",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("setup missing %q", want)
		}
	}
	if strings.Contains(out.String(), "/plugin marketplace add") || strings.Contains(out.String(), "/plugin install") {
		t.Fatal("setup still requires manual plugin installation")
	}
}

func TestChannelPluginMarketplace(t *testing.T) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := claude.ApplyAll(target, &claude.Config{}, nil, false, false, false); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(target, "plugins", "marketplaces", cfg.MarketplaceName)
	readJSON := func(path string, value any) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, value); err != nil {
			t.Fatal(err)
		}
	}
	var marketplace struct {
		Name  string `json:"name"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	readJSON(filepath.Join(root, ".claude-plugin", "marketplace.json"), &marketplace)
	if _, err := os.Stat(filepath.Join(root, "marketplace.json")); !os.IsNotExist(err) {
		t.Fatalf("stale root marketplace.json: %v", err)
	}
	if len(marketplace.Plugins) != 2 || marketplace.Owner.Name == "" {
		t.Fatalf("invalid marketplace: %#v", marketplace)
	}
	entry := marketplace.Plugins[1]
	if !strings.HasPrefix(entry.Source, "./") || strings.Contains(entry.Source, "..") {
		t.Fatalf("source must stay inside marketplace: %q", entry.Source)
	}
	pluginRoot := filepath.Join(root, entry.Source)
	var plugin struct {
		Name string `json:"name"`
	}
	readJSON(filepath.Join(pluginRoot, ".claude-plugin/plugin.json"), &plugin)
	validNames := func(market, entry, manifest string) bool {
		return entry == manifest && entry == cfg.PluginName && market == cfg.MarketplaceName && entry+"@"+market == "harnez-channel@harnez-local"
	}
	if !validNames(marketplace.Name, entry.Name, plugin.Name) {
		t.Fatalf("wrong plugin identity: %s / %s@%s", plugin.Name, entry.Name, marketplace.Name)
	}
	for _, names := range [][3]string{
		{"wrong", entry.Name, plugin.Name},
		{marketplace.Name, "wrong", plugin.Name},
		{marketplace.Name, entry.Name, "wrong"},
		{marketplace.Name, "wrong", "wrong"},
	} {
		if validNames(names[0], names[1], names[2]) {
			t.Errorf("accepted wrong names: %v", names)
		}
	}
	var mcp struct {
		Servers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	readJSON(filepath.Join(pluginRoot, ".mcp.json"), &mcp)
	server, ok := mcp.Servers[cfg.ServerName]
	if !ok || len(mcp.Servers) != 1 || server.Type != "stdio" || server.Command != "harnez" || len(server.Args) != 1 || server.Args[0] != "channel" {
		t.Fatalf("wrong channel subprocess: %#v", mcp)
	}
}
