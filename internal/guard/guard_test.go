package guard

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/decide"
)

// mockBackend implements decide.Backend for deterministic testing.
type mockBackend struct {
	fn func(ctx context.Context, req *decide.Request) (*decide.Response, error)
}

func (m *mockBackend) Decide(ctx context.Context, req *decide.Request) (*decide.Response, error) {
	return m.fn(ctx, req)
}

func TestCollectRules(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, ".harnez", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	agentsMD := "# AGENTS\n## CLI Scope\nDo not merge apply and init.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsMD), 0644); err != nil {
		t.Fatal(err)
	}

	toolsRule := "# Tools\nPrefer loaded mcp tools.\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "Tools.md"), []byte(toolsRule), 0644); err != nil {
		t.Fatal(err)
	}

	rules, err := CollectRules(dir)
	if err != nil {
		t.Fatalf("CollectRules failed: %v", err)
	}

	if len(rules) == 0 {
		t.Fatalf("expected rules to be collected, got 0")
	}

	var foundTools, foundAgents bool
	for name, content := range rules {
		if name == "Tools.md" && content != "" {
			foundTools = true
		}
		if name == "AGENTS.md" && content != "" {
			foundAgents = true
		}
	}
	if !foundTools || !foundAgents {
		t.Errorf("missing collected rules: foundTools=%v, foundAgents=%v", foundTools, foundAgents)
	}
}

func TestCheckFileEdit_Allowed(t *testing.T) {
	noulVal := 0.05
	mock := &mockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"rule_violation": {
						Type: decide.TypeNoul,
						Noul: &noulVal,
					},
					"violated_rule": {
						Type:   decide.TypeChoice,
						Choice: "none",
					},
				},
			}, nil
		},
	}

	v, err := CheckFileEdit(context.Background(), mock, ".", "internal/pkg/file.go", "package pkg\n", 0.90)
	if err != nil {
		t.Fatalf("CheckFileEdit failed: %v", err)
	}
	if v.Blocked {
		t.Errorf("expected edit to be allowed, got blocked: %+v", v)
	}
}

func TestCheckFileEdit_Blocked(t *testing.T) {
	noulVal := 0.95
	mock := &mockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"rule_violation": {
						Type: decide.TypeNoul,
						Noul: &noulVal,
					},
					"violated_rule": {
						Type:   decide.TypeChoice,
						Choice: "Tools.md",
					},
				},
			}, nil
		},
	}

	v, err := CheckFileEdit(context.Background(), mock, ".", "CLAUDE.md", "direct edit to symlink", 0.90)
	if err != nil {
		t.Fatalf("CheckFileEdit failed: %v", err)
	}
	if !v.Blocked {
		t.Fatalf("expected edit to be blocked")
	}
	if v.Rule != "Tools.md" {
		t.Errorf("got violated rule %q, want Tools.md", v.Rule)
	}
	if v.Confidence < 0.90 {
		t.Errorf("got confidence %f, want >= 0.90", v.Confidence)
	}
}

func TestCheckCommand_Safe(t *testing.T) {
	noulVal := 0.02
	mock := &mockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"destructive": {
						Type: decide.TypeNoul,
						Noul: &noulVal,
					},
					"action_risk": {
						Type:   decide.TypeChoice,
						Choice: "read_only",
					},
				},
			}, nil
		},
	}

	v, err := CheckCommand(context.Background(), mock, "/home/user/project", "git status", 0.85)
	if err != nil {
		t.Fatalf("CheckCommand failed: %v", err)
	}
	if v.Blocked {
		t.Errorf("expected command to be safe, got blocked: %+v", v)
	}
}

func TestCheckCommand_DestructiveBlocked(t *testing.T) {
	noulVal := 0.98
	mock := &mockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"destructive": {
						Type: decide.TypeNoul,
						Noul: &noulVal,
					},
					"action_risk": {
						Type:   decide.TypeChoice,
						Choice: "destructive",
					},
				},
			}, nil
		},
	}

	v, err := CheckCommand(context.Background(), mock, "/home/user/project", "git push origin main --force", 0.85)
	if err != nil {
		t.Fatalf("CheckCommand failed: %v", err)
	}
	if !v.Blocked {
		t.Fatalf("expected command to be blocked")
	}
	if v.Risk != "destructive" {
		t.Errorf("got risk %q, want destructive", v.Risk)
	}
}
