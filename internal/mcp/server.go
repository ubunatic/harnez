// Package mcp exposes Harnez agent lifecycle operations over MCP stdio.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// Server implements the JSON-RPC subset required by MCP stdio clients.
type Server struct {
	In      io.Reader
	Out     io.Writer
	Command string
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

var tools = []map[string]any{
	{"name": "harnez_spawn_agent", "description": "Start a Harnez subagent and return its result and session record.", "inputSchema": schema(map[string]any{"prompt": stringProp("Task prompt"), "model": stringProp("Optional provider:model[:tier] or model alias"), "role": stringProp("Optional role"), "dir": stringProp("Optional working directory"), "name": stringProp("Optional unique session name")}, "prompt")},
	{"name": "harnez_list_agents", "description": "List agent sessions visible to the caller.", "inputSchema": schema(map[string]any{"dir": stringProp("Optional working directory filter")})},
	{"name": "harnez_agent_status", "description": "Get the status record for a session in the caller's lineage.", "inputSchema": schema(map[string]any{"session_id": stringProp("Session ID or name")}, "session_id")},
	{"name": "harnez_resume_agent", "description": "Resume a session in the caller's lineage with a prompt.", "inputSchema": schema(map[string]any{"session_id": stringProp("Session ID or name"), "prompt": stringProp("Prompt for the next turn")}, "session_id", "prompt")},
	{"name": "harnez_stop_agent", "description": "Stop a session in the caller's lineage.", "inputSchema": schema(map[string]any{"session_id": stringProp("Session ID or name")}, "session_id")},
}

func stringProp(description string) map[string]string {
	return map[string]string{"type": "string", "description": description}
}
func schema(props map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

// Run reads and writes newline-delimited JSON-RPC messages until EOF or cancellation.
func (s Server) Run(ctx context.Context) error {
	if s.In == nil || s.Out == nil {
		return errors.New("MCP stdio requires input and output streams")
	}
	if s.Command == "" {
		return errors.New("harnez executable path is empty")
	}
	w := bufio.NewWriter(s.Out)
	r := bufio.NewScanner(s.In)
	r.Buffer(make([]byte, 4096), 16*1024*1024)
	for r.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(r.Bytes(), &req); err != nil {
			if e := write(w, rpcResponse{JSONRPC: "2.0", ID: nil, Error: &rpcError{-32700, "parse error"}}); e != nil {
				return e
			}
			continue
		}
		if req.JSONRPC != "2.0" {
			if len(req.ID) > 0 {
				_ = write(w, rpcResponse{JSONRPC: "2.0", ID: decodeID(req.ID), Error: &rpcError{-32600, "invalid request"}})
			}
			continue
		}
		result, rpcErr := s.handle(ctx, req)
		if len(req.ID) == 0 {
			continue
		}
		resp := rpcResponse{JSONRPC: "2.0", ID: decodeID(req.ID), Result: result, Error: rpcErr}
		if err := write(w, resp); err != nil {
			return err
		}
	}
	return r.Err()
}

func write(w *bufio.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err = w.Write(append(b, '\n')); err != nil {
		return err
	}
	return w.Flush()
}
func decodeID(raw json.RawMessage) any {
	var v any
	if len(raw) == 0 {
		return nil
	}
	_ = json.Unmarshal(raw, &v)
	return v
}

func (s Server) handle(ctx context.Context, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "harnez", "version": "1"}}, nil
	case "notifications/initialized":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var call toolCall
		if err := json.Unmarshal(req.Params, &call); err != nil {
			return nil, &rpcError{-32602, "invalid tools/call parameters"}
		}
		result, err := s.call(ctx, call)
		if err != nil {
			return map[string]any{"content": []any{map[string]string{"type": "text", "text": err.Error()}}, "isError": true}, nil
		}
		b, err := json.Marshal(result)
		if err != nil {
			return nil, &rpcError{-32603, "encode tool result"}
		}
		return map[string]any{"content": []any{map[string]string{"type": "text", "text": string(b)}}, "structuredContent": result}, nil
	default:
		return nil, &rpcError{-32601, "method not found"}
	}
}

func (s Server) call(ctx context.Context, c toolCall) (any, error) {
	a := c.Arguments
	if a == nil {
		a = map[string]any{}
	}
	arg := func(key string, required bool) (string, error) {
		v, _ := a[key].(string)
		if required && strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("%s is required", key)
		}
		return v, nil
	}
	base := []string{"agent"}
	switch c.Name {
	case "harnez_spawn_agent":
		prompt, e := arg("prompt", true)
		if e != nil {
			return nil, e
		}
		base = append(base, "start", "--json", "--stream", "stats")
		for _, key := range []string{"model", "role", "dir", "name"} {
			v, _ := arg(key, false)
			if v != "" {
				flag := map[string]string{"model": "--model", "role": "--role", "dir": "--dir", "name": "--name"}[key]
				base = append(base, flag, v)
			}
		}
		base = append(base, "--", prompt)
	case "harnez_list_agents":
		base = append(base, "list", "--json")
		if d, _ := arg("dir", false); d != "" {
			base = append(base, "--dir", d)
		}
	case "harnez_agent_status":
		id, e := arg("session_id", true)
		if e != nil {
			return nil, e
		}
		base = append(base, "status", "--json", "--name", id)
	case "harnez_resume_agent":
		id, e := arg("session_id", true)
		if e != nil {
			return nil, e
		}
		p, e := arg("prompt", true)
		if e != nil {
			return nil, e
		}
		base = append(base, "resume", "--json", "--stream", "stats", "--name", id, "--", p)
	case "harnez_stop_agent":
		id, e := arg("session_id", true)
		if e != nil {
			return nil, e
		}
		base = append(base, "stop", "--name", id)
	default:
		return nil, fmt.Errorf("unknown tool %q", c.Name)
	}
	cmd := exec.CommandContext(ctx, s.Command, base[1:]...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("harnez %s: %s", strings.Join(base, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var v any
	if json.Unmarshal(out, &v) == nil {
		if c.Name == "harnez_list_agents" {
			if dir, _ := arg("dir", false); dir != "" {
				abs, pathErr := filepath.Abs(dir)
				if pathErr != nil {
					return nil, pathErr
				}
				rows, ok := v.([]any)
				if !ok {
					return nil, errors.New("harnez agent list returned unexpected JSON")
				}
				filtered := make([]any, 0, len(rows))
				for _, row := range rows {
					item, ok := row.(map[string]any)
					if ok && item["working_dir"] == abs {
						filtered = append(filtered, row)
					}
				}
				v = filtered
			}
		}
		return v, nil
	}
	return map[string]string{"result": strings.TrimSpace(string(out))}, nil
}
