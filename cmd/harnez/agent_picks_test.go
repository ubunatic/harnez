package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestModelPicksGolden(t *testing.T) {
	rows := []subagent.ModelPick{
		{Role: "developer", Trait: "ai", Chain: []string{}, Error: "unset"},
		{Role: "developer", Trait: "smart", Chain: []string{"sol:med", "opus:med"}, Resolved: "codex:sol:med", Default: true},
		{Role: "reviewer", Trait: "smart", Chain: []string{"sol:low"}, Resolved: "codex:sol:low"},
	}
	var out bytes.Buffer
	if err := writeModelPicks(&out, rows); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/agent-picks.golden")
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestModelPicksCommand(t *testing.T) {
	run := func(args ...string) string {
		cmd := newAgentCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(append([]string{"models"}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	text := run("--picks")
	if !strings.HasPrefix(text, "ROLE") || !strings.Contains(text, "developer") || strings.Contains(text, "SPEC  MODEL") {
		t.Fatalf("picks table: %s", text)
	}
	if !strings.Contains(run(), "ROLE  ") {
		t.Fatal("plain listing lacks picks table")
	}
	var picks []subagent.ModelPick
	if err := json.Unmarshal([]byte(run("--picks", "--json")), &picks); err != nil {
		t.Fatal(err)
	}
	var smart, ai, defaultLean bool
	for i, p := range picks {
		if i > 0 && (p.Role < picks[i-1].Role || p.Role == picks[i-1].Role && p.Trait < picks[i-1].Trait) {
			t.Fatal("picks are not sorted")
		}
		if p.Role == "developer" && p.Trait == "smart" {
			smart = len(p.Chain) == 2 && p.Resolved == "codex:sol:med" && p.Error == "" && !p.Default
		}
		if p.Role == "developer" && p.Trait == "ai" {
			ai = len(p.Chain) == 0 && strings.Contains(p.Error, "unset") && !p.Default
		}
		if p.Role == "developer" && p.Trait == "lean" {
			defaultLean = p.Default
		}
	}
	if !smart || !ai || !defaultLean {
		t.Fatalf("missing picks: %+v", picks)
	}
	var models []subagent.ModelEntry
	if err := json.Unmarshal([]byte(run("--json")), &models); err != nil || len(models) == 0 {
		t.Fatalf("model JSON changed: %v", err)
	}
	if strings.Contains(run("--names"), "developer") {
		t.Fatal("--names contains picks")
	}
}
