package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

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

// Silence unused import warnings in tests if any
var _ = time.Now
