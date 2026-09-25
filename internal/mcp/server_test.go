package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtocolInitializeToolsAndNotifications(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	err := (Server{In: strings.NewReader(in), Out: &out, Command: "unused"}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d responses: %s", len(lines), out.String())
	}
	var initialize map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &initialize); err != nil {
		t.Fatal(err)
	}
	if initialize["jsonrpc"] != "2.0" || initialize["id"].(float64) != 1 {
		t.Fatalf("bad initialize response: %#v", initialize)
	}
	var listing struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &listing); err != nil {
		t.Fatal(err)
	}
	want := []string{"harnez_spawn_agent", "harnez_list_agents", "harnez_agent_status", "harnez_resume_agent", "harnez_stop_agent"}
	if len(listing.Result.Tools) != len(want) {
		t.Fatalf("tools = %#v", listing.Result.Tools)
	}
	for i, name := range want {
		if listing.Result.Tools[i].Name != name {
			t.Errorf("tool[%d] = %q, want %q", i, listing.Result.Tools[i].Name, name)
		}
	}
}

func TestToolCallInvokesAgentAndReturnsStructuredResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harnez")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '{\"id\":\"session-1\",\"name\":\"worker\"}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	input := `{"jsonrpc":"2.0","id":"call-1","method":"tools/call","params":{"name":"harnez_spawn_agent","arguments":{"prompt":"inspect"}}}` + "\n"
	if err := (Server{In: strings.NewReader(input), Out: &out, Command: path}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Result struct {
			Structured map[string]string `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Result.Structured["id"] != "session-1" {
		t.Fatalf("result = %#v", got.Result.Structured)
	}
}

func TestToolCallValidationAndToolErrors(t *testing.T) {
	server := Server{Command: "/missing/harnez"}
	if _, err := server.call(context.Background(), toolCall{Name: "harnez_spawn_agent", Arguments: map[string]any{}}); err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("missing argument error = %v", err)
	}
	if _, err := server.call(context.Background(), toolCall{Name: "harnez_nope"}); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("unknown tool error = %v", err)
	}
	result, err := server.handle(context.Background(), rpcRequest{JSONRPC: "2.0", Method: "tools/call", Params: json.RawMessage(`{"name":"harnez_spawn_agent","arguments":{}}`)})
	if err != nil {
		t.Fatalf("tool failures should be MCP isError results: %v", err)
	}
	encoded, _ := json.Marshal(result)
	if !strings.Contains(string(encoded), `"isError":true`) {
		t.Fatalf("result = %s", encoded)
	}
}

func TestProtocolParseAndUnknownMethod(t *testing.T) {
	input := "not-json\n" + `{"jsonrpc":"2.0","id":9,"method":"missing"}` + "\n"
	var out bytes.Buffer
	if err := (Server{In: strings.NewReader(input), Out: &out, Command: "unused"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"code":-32700`) || !strings.Contains(lines[1], `"code":-32601`) {
		t.Fatalf("responses = %s", out.String())
	}
}
