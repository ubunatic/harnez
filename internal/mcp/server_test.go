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
	want := []string{"harnez_spawn_agent", "harnez_command", "harnez_wait_agent", "harnez_list_agents", "harnez_agent_status", "harnez_resume_agent", "harnez_stop_agent"}
	if len(listing.Result.Tools) != len(want) {
		t.Fatalf("tools = %#v", listing.Result.Tools)
	}
	for i, name := range want {
		if listing.Result.Tools[i].Name != name {
			t.Errorf("tool[%d] = %q, want %q", i, listing.Result.Tools[i].Name, name)
		}
	}
}

func TestHarnezCommandFormatsActionsAndQuotesShellArguments(t *testing.T) {
	server := Server{Command: "/path/harnez tool's"}
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"start", map[string]any{"action": "start", "prompt": "inspect $(touch nope) 'quoted'", "model": "luna", "role": "advisor", "dir": "/tmp/work dir", "name": "worker"}, `'/path/harnez tool'\''s' 'agent' 'start' '--json' '--stream' 'stats' '--model' 'luna' '--role' 'advisor' '--dir' '/tmp/work dir' '--name' 'worker' '--' 'inspect $(touch nope) '\''quoted'\'''`},
		{"resume", map[string]any{"action": "resume", "prompt": "continue", "session_id": "worker"}, `'/path/harnez tool'\''s' 'agent' 'resume' '--json' '--stream' 'stats' '--name' 'worker' '--' 'continue'`},
		{"wait", map[string]any{"action": "wait", "session_id": "worker"}, `'/path/harnez tool'\''s' 'agent' 'wait' '--json' 'worker'`},
		{"status", map[string]any{"action": "status", "session_id": "worker"}, `'/path/harnez tool'\''s' 'agent' 'status' '--json' '--name' 'worker'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := server.call(context.Background(), toolCall{Name: "harnez_command", Arguments: tt.args})
			if err != nil {
				t.Fatal(err)
			}
			result := got.(map[string]string)
			if result["command"] != tt.want {
				t.Errorf("command = %q, want %q", result["command"], tt.want)
			}
			if !strings.Contains(result["instruction"], "backgrounding enabled") || !strings.Contains(result["instruction"], "reactive wakeups") {
				t.Errorf("instruction lacks host background guidance: %q", result["instruction"])
			}
		})
	}
}

func TestHarnezCommandValidationAndProtocol(t *testing.T) {
	server := Server{Command: "harnez"}
	for _, args := range []map[string]any{
		{"action": "launch", "prompt": "x"},
		{"action": "start"},
		{"action": "resume", "prompt": "x"},
		{"action": "wait"},
		{"action": "start", "prompt": "x", "stream": "invalid"},
	} {
		if _, err := server.call(context.Background(), toolCall{Name: "harnez_command", Arguments: args}); err == nil {
			t.Errorf("invalid arguments accepted: %#v", args)
		}
	}
	input := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"harnez_command","arguments":{"action":"start","prompt":"a ' b"}}}` + "\n"
	var out bytes.Buffer
	if err := (Server{In: strings.NewReader(input), Out: &out, Command: "harnez"}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			Structured struct {
				Command     string `json:"command"`
				Instruction string `json:"instruction"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Result.Structured.Command, `'a '\'' b'`) || response.Result.Structured.Instruction == "" {
		t.Fatalf("protocol result = %#v", response.Result.Structured)
	}
}

func TestAsyncSpawnAndWaitToolArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harnez")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	server := Server{Command: path}
	got, err := server.call(context.Background(), toolCall{Name: "harnez_spawn_agent", Arguments: map[string]any{"prompt": "inspect", "async": true}})
	if err != nil || !strings.Contains(got.(map[string]string)["result"], "--detach") {
		t.Fatalf("async spawn = %#v, %v", got, err)
	}
	got, err = server.call(context.Background(), toolCall{Name: "harnez_wait_agent", Arguments: map[string]any{"session_id": "session-1", "timeout_seconds": 2.0}})
	if err != nil || got.(map[string]string)["result"] != "agent wait --json session-1 --timeout 2s" {
		t.Fatalf("wait tool = %#v, %v", got, err)
	}
	if _, err := server.call(context.Background(), toolCall{Name: "harnez_spawn_agent", Arguments: map[string]any{"prompt": "x", "async": "yes"}}); err == nil {
		t.Fatal("non-boolean async accepted")
	}
}

func TestToolCallInvokesAgentAndReturnsStructuredResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "harnez")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n[ \"$1\" = agent ] && [ \"$2\" = start ] || { echo \"unexpected args: $*\" >&2; exit 9; }\nprintf '{\"id\":\"session-1\",\"name\":\"worker\"}'\n"), 0700); err != nil {
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
