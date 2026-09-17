package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/telemetry"
)

func TestRunAgyToolHook_AllowOutput(t *testing.T) {
	in := bytes.NewBufferString(`{
		"conversationId": "test-conv-123",
		"toolCall": {
			"name": "generate_image",
			"args": {"Prompt": "a cute cat"}
		},
		"stepIdx": 4
	}`)
	var out bytes.Buffer

	var recorded telemetry.ToolCall
	opts := agyHookOptions{
		DBPath: "/tmp/dummy.db",
		Insert: func(dbPath string, call telemetry.ToolCall) error {
			recorded = call
			return nil
		},
	}

	if err := runAgyToolHook(in, &out, opts); err != nil {
		t.Fatalf("runAgyToolHook failed: %v", err)
	}

	gotOut := strings.TrimSpace(out.String())
	wantOut := `{"decision":"allow"}`
	if gotOut != wantOut {
		t.Errorf("output = %q, want %q", gotOut, wantOut)
	}

	if recorded.ToolName != "generate_image" {
		t.Errorf("recorded.ToolName = %q, want %q", recorded.ToolName, "generate_image")
	}
	if recorded.AgentID != "agy" {
		t.Errorf("recorded.AgentID = %q, want %q", recorded.AgentID, "agy")
	}
	if recorded.CallType != "hook:rpc" {
		t.Errorf("recorded.CallType = %q, want %q", recorded.CallType, "hook:rpc")
	}
	if recorded.SessionID != "test-conv-123" {
		t.Errorf("recorded.SessionID = %q, want %q", recorded.SessionID, "test-conv-123")
	}
	if recorded.Score == nil || *recorded.Score != 5 {
		t.Errorf("recorded.Score = %v, want 5", recorded.Score)
	}
	if recorded.Note != "" {
		t.Errorf("recorded.Note = %q, want empty", recorded.Note)
	}
}

func TestRunAgyToolHook_RunCommandPrepsBash(t *testing.T) {
	in := bytes.NewBufferString(`{
		"conversationId": "test-conv-456",
		"toolCall": {
			"name": "run_command",
			"args": {"CommandLine": "git status"}
		},
		"stepIdx": 10
	}`)
	var out bytes.Buffer

	var recorded telemetry.ToolCall
	opts := agyHookOptions{
		DBPath: "/tmp/dummy.db",
		Insert: func(dbPath string, call telemetry.ToolCall) error {
			recorded = call
			return nil
		},
	}

	if err := runAgyToolHook(in, &out, opts); err != nil {
		t.Fatalf("runAgyToolHook failed: %v", err)
	}

	gotOut := strings.TrimSpace(out.String())
	if gotOut != `{"decision":"allow"}` {
		t.Errorf("output = %q, want allow", gotOut)
	}

	if recorded.ToolName != "run_command" {
		t.Errorf("recorded.ToolName = %q, want %q", recorded.ToolName, "run_command")
	}
	if recorded.CallType != "hook:prep" {
		t.Errorf("recorded.CallType = %q, want %q", recorded.CallType, "hook:prep")
	}
	if recorded.Note != "preps:Bash | git status" {
		t.Errorf("recorded.Note = %q, want %q", recorded.Note, "preps:Bash | git status")
	}
}

func TestRunAgyToolHook_EmptyOrInvalidStdinReturnsAllow(t *testing.T) {
	// Empty stdin
	{
		var out bytes.Buffer
		if err := runAgyToolHook(bytes.NewBuffer(nil), &out, agyHookOptions{}); err != nil {
			t.Fatalf("expected nil error on empty input, got %v", err)
		}
		if strings.TrimSpace(out.String()) != `{"decision":"allow"}` {
			t.Errorf("empty input output = %q, want allow", out.String())
		}
	}

	// Invalid json
	{
		var out bytes.Buffer
		in := bytes.NewBufferString("not valid json")
		if err := runAgyToolHook(in, &out, agyHookOptions{}); err == nil {
			t.Fatalf("expected error on invalid json, got nil")
		}
		if strings.TrimSpace(out.String()) != `{"decision":"allow"}` {
			t.Errorf("invalid json output = %q, want allow", out.String())
		}
	}
}

func TestRunAgyToolHook_DBIntegration(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.sqlite"

	db, err := telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	in := bytes.NewBufferString(`{
		"conversationId": "agy-sess-789",
		"toolCall": {
			"name": "view_file",
			"args": {"AbsolutePath": "/foo/bar.go"}
		}
	}`)
	var out bytes.Buffer

	opts := agyHookOptions{
		DBPath: dbPath,
	}

	if err := runAgyToolHook(in, &out, opts); err != nil {
		t.Fatalf("runAgyToolHook: %v", err)
	}

	// Verify row in DB
	db, err = telemetry.Open(dbPath)
	if err != nil {
		t.Fatalf("re-open db: %v", err)
	}
	defer db.Close()

	calls, err := db.Query(telemetry.Filter{SessionID: "agy-sess-789"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	if calls[0].ToolName != "view_file" || calls[0].CallType != "hook:rpc" || calls[0].AgentID != "agy" {
		t.Errorf("unexpected call: %+v", calls[0])
	}
}

func createTestFile(t *testing.T, dir, name string, lines int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	var buf bytes.Buffer
	for i := 1; i <= lines; i++ {
		fmt.Fprintf(&buf, "line %d\n", i)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
	return path
}

func TestRunAgyToolHook_ReadingDiscipline(t *testing.T) {
	tempDir := t.TempDir()
	smallFile := createTestFile(t, tempDir, "small.txt", 50)
	largeFile := createTestFile(t, tempDir, "large.txt", 150)

	tests := []struct {
		name         string
		toolCallJSON string
		wantDecision string
		wantInReason string
	}{
		{
			name: "small file unconstrained allow",
			toolCallJSON: fmt.Sprintf(`{
				"toolCall": {
					"name": "view_file",
					"args": {"AbsolutePath": %q}
				}
			}`, smallFile),
			wantDecision: "allow",
		},
		{
			name: "large file unconstrained deny",
			toolCallJSON: fmt.Sprintf(`{
				"toolCall": {
					"name": "view_file",
					"args": {"AbsolutePath": %q}
				}
			}`, largeFile),
			wantDecision: "deny",
			wantInReason: "violates Reading & Context Discipline",
		},
		{
			name: "large file bounded small slice allow",
			toolCallJSON: fmt.Sprintf(`{
				"toolCall": {
					"name": "view_file",
					"args": {"AbsolutePath": %q, "StartLine": 10, "EndLine": 30}
				}
			}`, largeFile),
			wantDecision: "allow",
		},
		{
			name: "large file slice >= 100 lines deny",
			toolCallJSON: fmt.Sprintf(`{
				"toolCall": {
					"name": "view_file",
					"args": {"AbsolutePath": %q, "StartLine": 1, "EndLine": 110}
				}
			}`, largeFile),
			wantDecision: "deny",
			wantInReason: "violates Reading & Context Discipline",
		},
		{
			name: "large file start line unbounded remaining deny",
			toolCallJSON: fmt.Sprintf(`{
				"toolCall": {
					"name": "view_file",
					"args": {"AbsolutePath": %q, "StartLine": 1}
				}
			}`, largeFile),
			wantDecision: "deny",
			wantInReason: "violates Reading & Context Discipline",
		},
		{
			name: "large file start line small remaining slice allow",
			toolCallJSON: fmt.Sprintf(`{
				"toolCall": {
					"name": "view_file",
					"args": {"AbsolutePath": %q, "StartLine": 130}
				}
			}`, largeFile),
			wantDecision: "allow",
		},
		{
			name: "non-reading tool allow",
			toolCallJSON: `{
				"toolCall": {
					"name": "replace_file_content",
					"args": {"TargetFile": "/some/file.txt"}
				}
			}`,
			wantDecision: "allow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.NewBufferString(tc.toolCallJSON)
			var out bytes.Buffer
			var recorded telemetry.ToolCall
			opts := agyHookOptions{
				BaseDir: tempDir,
				DBPath:  filepath.Join(tempDir, "test.sqlite"),
				Insert: func(dbPath string, call telemetry.ToolCall) error {
					recorded = call
					return nil
				},
			}

			if err := runAgyToolHook(in, &out, opts); err != nil {
				t.Fatalf("runAgyToolHook failed: %v", err)
			}

			var resp map[string]any
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal hook response: %v, raw: %s", err, out.String())
			}

			if resp["decision"] != tc.wantDecision {
				t.Errorf("decision = %v, want %v (raw: %s)", resp["decision"], tc.wantDecision, out.String())
			}
			if tc.wantInReason != "" {
				reason, _ := resp["reason"].(string)
				if !strings.Contains(reason, tc.wantInReason) {
					t.Errorf("reason = %q, want containing %q", reason, tc.wantInReason)
				}
			}
			if recorded.ToolName == "" {
				t.Errorf("expected telemetry insert to be called, got empty ToolName")
			}
		})
	}
}

func TestRunClaudeReadHook_ReadingDiscipline(t *testing.T) {
	tempDir := t.TempDir()
	smallFile := createTestFile(t, tempDir, "small.txt", 40)
	largeFile := createTestFile(t, tempDir, "large.txt", 160)

	tests := []struct {
		name         string
		payloadJSON  string
		wantDecision string
		wantInSystem string
	}{
		{
			name: "View small file unbounded allow",
			payloadJSON: fmt.Sprintf(`{
				"hookEventName": "PreToolUse",
				"tool_name": "View",
				"tool_input": {"file_path": %q}
			}`, smallFile),
			wantDecision: "",
		},
		{
			name: "View large file unbounded deny",
			payloadJSON: fmt.Sprintf(`{
				"hookEventName": "PreToolUse",
				"tool_name": "View",
				"tool_input": {"file_path": %q}
			}`, largeFile),
			wantDecision: "deny",
			wantInSystem: "violates Reading & Context Discipline",
		},
		{
			name: "View large file bounded slice allow",
			payloadJSON: fmt.Sprintf(`{
				"hookEventName": "PreToolUse",
				"tool_name": "View",
				"tool_input": {"file_path": %q, "view_range": [10, 40]}
			}`, largeFile),
			wantDecision: "",
		},
		{
			name: "View large file slice >= 100 lines deny",
			payloadJSON: fmt.Sprintf(`{
				"hookEventName": "PreToolUse",
				"tool_name": "View",
				"tool_input": {"file_path": %q, "view_range": [1, 120]}
			}`, largeFile),
			wantDecision: "deny",
			wantInSystem: "violates Reading & Context Discipline",
		},
		{
			name: "ReadMultipleFiles small files allow",
			payloadJSON: fmt.Sprintf(`{
				"hookEventName": "PreToolUse",
				"tool_name": "ReadMultipleFiles",
				"tool_input": {"paths": [%q, %q]}
			}`, smallFile, smallFile),
			wantDecision: "",
		},
		{
			name: "ReadMultipleFiles containing large file deny",
			payloadJSON: fmt.Sprintf(`{
				"hookEventName": "PreToolUse",
				"tool_name": "ReadMultipleFiles",
				"tool_input": {"paths": [%q, %q]}
			}`, smallFile, largeFile),
			wantDecision: "deny",
			wantInSystem: "violates Reading & Context Discipline",
		},
		{
			name: "Unrelated tool allow",
			payloadJSON: `{
				"hookEventName": "PreToolUse",
				"tool_name": "Bash",
				"tool_input": {"command": "ls -l"}
			}`,
			wantDecision: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.NewBufferString(tc.payloadJSON)
			var out bytes.Buffer
			opts := readHookOptions{
				BaseDir: tempDir,
			}

			if err := runClaudeReadHook(in, &out, opts); err != nil {
				t.Fatalf("runClaudeReadHook failed: %v", err)
			}

			var resp claudeHookOutput
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal hook response: %v, raw: %s", err, out.String())
			}

			if resp.HookSpecificOutput.PermissionDecision != tc.wantDecision {
				t.Errorf("permissionDecision = %q, want %q", resp.HookSpecificOutput.PermissionDecision, tc.wantDecision)
			}
			if tc.wantInSystem != "" {
				if !strings.Contains(resp.SystemMessage, tc.wantInSystem) {
					t.Errorf("systemMessage = %q, want containing %q", resp.SystemMessage, tc.wantInSystem)
				}
			}
		})
	}
}

func TestRunClaudeReadHook_EmptyAndInvalidInput(t *testing.T) {
	// Empty input
	{
		var out bytes.Buffer
		if err := runClaudeReadHook(bytes.NewBuffer(nil), &out, readHookOptions{}); err != nil {
			t.Fatalf("empty input err: %v", err)
		}
		var resp claudeHookOutput
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.HookSpecificOutput.PermissionDecision != "" {
			t.Errorf("empty input decision = %q, want native permission passthrough", resp.HookSpecificOutput.PermissionDecision)
		}
	}

	// Invalid JSON
	{
		var out bytes.Buffer
		if err := runClaudeReadHook(bytes.NewBufferString("not valid json"), &out, readHookOptions{}); err == nil {
			t.Fatalf("expected error on invalid json, got nil")
		}
		var resp claudeHookOutput
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.HookSpecificOutput.PermissionDecision != "" {
			t.Errorf("invalid json decision = %q, want native permission passthrough", resp.HookSpecificOutput.PermissionDecision)
		}
	}
}
