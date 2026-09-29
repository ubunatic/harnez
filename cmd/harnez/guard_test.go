package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/decide"
)

func TestGuardStatusCmd_TextAndJSON(t *testing.T) {
	// 1. Text mode
	cmd := newGuardCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("guard status failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Decide Guardrail Status") {
		t.Errorf("expected header in output, got: %s", out)
	}
	if !strings.Contains(out, "Backend:      jev") {
		t.Errorf("expected Backend: jev in output, got: %s", out)
	}
	if !strings.Contains(out, "Threshold:    0.85") {
		t.Errorf("expected Threshold: 0.85 in output, got: %s", out)
	}

	// 2. JSON mode
	buf.Reset()
	cmd = newGuardCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"status", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("guard status --json failed: %v", err)
	}

	var report guardStatusReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("failed to decode status JSON: %v", err)
	}
	if report.BackendName != "jev" {
		t.Errorf("got backend %q, want jev", report.BackendName)
	}
	if report.Threshold != 0.85 {
		t.Errorf("got threshold %f, want 0.85", report.Threshold)
	}
}

func TestGuardStatusCmd_SourceReporting(t *testing.T) {
	t.Setenv("HARNEZ_DECIDE_GUARD", "1")
	var buf bytes.Buffer
	if err := runGuardStatus(&buf, false); err != nil {
		t.Fatalf("runGuardStatus failed: %v", err)
	}
	if !strings.Contains(buf.String(), "ENABLED ($HARNEZ_DECIDE_GUARD)") {
		t.Errorf("expected enabled via env, got: %s", buf.String())
	}

	t.Setenv("HARNEZ_DECIDE_GUARD", "0")
	buf.Reset()
	if err := runGuardStatus(&buf, false); err != nil {
		t.Fatalf("runGuardStatus failed: %v", err)
	}
	if !strings.Contains(buf.String(), "DISABLED ($HARNEZ_DECIDE_GUARD)") {
		t.Errorf("expected disabled via env, got: %s", buf.String())
	}
}

func TestGuardCheckCmd_Allowed(t *testing.T) {
	noulVal := 0.05
	mock := &testMockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"rule_violation": {Type: decide.TypeNoul, Noul: &noulVal},
					"violated_rule":  {Type: decide.TypeChoice, Choice: "none"},
				},
			}, nil
		},
	}

	dir := t.TempDir()
	f := filepath.Join(dir, "main.go")
	if err := os.WriteFile(f, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := runGuardCheck(context.Background(), &buf, mock, dir, f, "package main\n", 0.85, "table")
	if err != nil {
		t.Fatalf("runGuardCheck failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Allowed") {
		t.Errorf("expected Allowed in output, got: %s", out)
	}
}

func TestGuardCheckCmd_Blocked(t *testing.T) {
	noulVal := 0.95
	mock := &testMockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"rule_violation": {Type: decide.TypeNoul, Noul: &noulVal},
					"violated_rule":  {Type: decide.TypeChoice, Choice: "Tools.md"},
				},
			}, nil
		},
	}

	dir := t.TempDir()
	f := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(f, []byte("symlink direct edit"), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := runGuardCheck(context.Background(), &buf, mock, dir, f, "symlink direct edit", 0.85, "table")
	if err == nil {
		t.Fatalf("expected error/exit code 1 for blocked check, got nil")
	}

	out := buf.String()
	if !strings.Contains(out, "BLOCKED") {
		t.Errorf("expected BLOCKED in output, got: %s", out)
	}
	if !strings.Contains(out, "Tools.md") {
		t.Errorf("expected Tools.md in output, got: %s", out)
	}
}

func TestGuardCheckCmd_JSONFormat(t *testing.T) {
	noulVal := 0.05
	mock := &testMockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"rule_violation": {Type: decide.TypeNoul, Noul: &noulVal},
					"violated_rule":  {Type: decide.TypeChoice, Choice: "none"},
				},
			}, nil
		},
	}

	var buf bytes.Buffer
	err := runGuardCheck(context.Background(), &buf, mock, ".", "dummy.go", "package main\n", 0.85, "json")
	if err != nil {
		t.Fatalf("runGuardCheck JSON failed: %v", err)
	}

	var v struct {
		Blocked    bool    `json:"blocked"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if v.Blocked {
		t.Errorf("expected Blocked to be false")
	}
}

func TestGuardCheckCmd_InvalidFormat(t *testing.T) {
	mock := &testMockBackend{}
	var buf bytes.Buffer
	err := runGuardCheck(context.Background(), &buf, mock, ".", "dummy.go", "package main\n", 0.85, "yaml")
	if err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected unknown format error, got: %v", err)
	}
}

func TestGuardCheckCmd_CobraFlags(t *testing.T) {
	cmd := newGuardCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Execute with --format invalid to test Cobra flag parsing and validation
	cmd.SetArgs([]string{"check", "guard.go", "--content", "package main\n", "--format", "invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for invalid format, got nil")
	}
}
