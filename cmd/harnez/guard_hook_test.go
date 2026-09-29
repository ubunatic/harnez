package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/decide"
)

type testMockBackend struct {
	fn func(ctx context.Context, req *decide.Request) (*decide.Response, error)
}

func (m *testMockBackend) Decide(ctx context.Context, req *decide.Request) (*decide.Response, error) {
	return m.fn(ctx, req)
}

func TestRunPreEditHook_Claude_Allowed(t *testing.T) {
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

	in := strings.NewReader(`{
		"hookEventName": "PreToolUse",
		"tool_name": "Edit",
		"tool_input": {
			"file_path": "internal/pkg/foo.go",
			"new_string": "package foo\n"
		}
	}`)
	var out bytes.Buffer
	enforce := true
	opts := preEditOptions{
		Backend: mock,
		Enforce: &enforce,
	}

	err := runPreEditHook(context.Background(), in, &out, "", "", opts)
	if err != nil {
		t.Fatalf("runPreEditHook failed: %v", err)
	}

	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("got out %q, want {}", out.String())
	}
}

func TestRunPreEditHook_Claude_Blocked(t *testing.T) {
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

	in := strings.NewReader(`{
		"hookEventName": "PreToolUse",
		"tool_name": "Write",
		"tool_input": {
			"file_path": "CLAUDE.md",
			"content": "direct edit to symlink"
		}
	}`)
	var out bytes.Buffer
	enforce := true
	opts := preEditOptions{
		Backend: mock,
		Enforce: &enforce,
	}

	err := runPreEditHook(context.Background(), in, &out, "", "", opts)
	if err != nil {
		t.Fatalf("runPreEditHook failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, `"permissionDecision":"deny"`) {
		t.Errorf("expected deny output, got %q", outStr)
	}
	if !strings.Contains(outStr, "Tools.md") {
		t.Errorf("expected rule Tools.md in reason, got %q", outStr)
	}
}

func TestRunPreEditHook_AGY_Blocked(t *testing.T) {
	noulVal := 0.92
	mock := &testMockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"rule_violation": {Type: decide.TypeNoul, Noul: &noulVal},
					"violated_rule":  {Type: decide.TypeChoice, Choice: "Quota.md"},
				},
			}, nil
		},
	}

	in := strings.NewReader(`{
		"conversationId": "agy-session-1",
		"toolCall": {
			"name": "write_to_file",
			"args": {
				"TargetFile": "/path/to/test.go",
				"CodeContent": "bypass quota"
			}
		}
	}`)
	var out bytes.Buffer
	enforce := true
	opts := preEditOptions{
		Backend: mock,
		Enforce: &enforce,
	}

	err := runPreEditHook(context.Background(), in, &out, "", "", opts)
	if err != nil {
		t.Fatalf("runPreEditHook failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, `"decision":"deny"`) {
		t.Errorf("expected deny output for AGY, got %q", outStr)
	}
}

func TestRunPreExecHook_Claude_DestructiveBlocked(t *testing.T) {
	noulVal := 0.96
	mock := &testMockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"destructive": {Type: decide.TypeNoul, Noul: &noulVal},
					"action_risk": {Type: decide.TypeChoice, Choice: "destructive"},
				},
			}, nil
		},
	}

	in := strings.NewReader(`{
		"hookEventName": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {
			"command": "git push origin main --force"
		}
	}`)
	var out bytes.Buffer
	enforce := true
	opts := preExecOptions{
		Backend: mock,
		Enforce: &enforce,
	}

	err := runPreExecHook(context.Background(), in, &out, "", "", opts)
	if err != nil {
		t.Fatalf("runPreExecHook failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, `"permissionDecision":"deny"`) {
		t.Errorf("expected deny output for destructive bash, got %q", outStr)
	}
}

func TestRunPreExecHook_AGY_SafeAllowed(t *testing.T) {
	noulVal := 0.01
	mock := &testMockBackend{
		fn: func(ctx context.Context, req *decide.Request) (*decide.Response, error) {
			return &decide.Response{
				Model: "jev-test",
				Answers: map[string]decide.Answer{
					"destructive": {Type: decide.TypeNoul, Noul: &noulVal},
					"action_risk": {Type: decide.TypeChoice, Choice: "read_only"},
				},
			}, nil
		},
	}

	in := strings.NewReader(`{
		"conversationId": "conv-456",
		"toolCall": {
			"name": "run_command",
			"args": {
				"CommandLine": "git status"
			}
		}
	}`)
	var out bytes.Buffer
	enforce := true
	opts := preExecOptions{
		Backend: mock,
		Enforce: &enforce,
	}

	err := runPreExecHook(context.Background(), in, &out, "", "", opts)
	if err != nil {
		t.Fatalf("runPreExecHook failed: %v", err)
	}

	outStr := strings.TrimSpace(out.String())
	if outStr != `{"decision":"allow"}` {
		t.Errorf("expected allow output for AGY, got %q", outStr)
	}
}
