package channel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// CodexThread is the session summary returned by Codex app-server.
type CodexThread struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	CWD  string `json:"cwd"`
}

type codexRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type codexRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type codexThreadListResult struct {
	Data       []CodexThread `json:"data"`
	NextCursor *string       `json:"nextCursor"`
}

// DiscoverCodexThreads starts Codex app-server and lists locally indexed threads.
func DiscoverCodexThreads(ctx context.Context, executable string, timeout time.Duration, maxRPCBytes int) ([]CodexThread, error) {
	if timeout <= 0 || maxRPCBytes <= 0 {
		return nil, fmt.Errorf("codex discovery requires positive limits")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "app-server")
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("codex app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex app-server stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	reader := bufio.NewReader(stdout)
	if err := writeCodexRequest(stdin, codexRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"clientInfo":   map[string]string{"name": "harnez", "title": "Harnez", "version": "1"},
			"capabilities": map[string]any{},
		},
	}); err != nil {
		return nil, fmt.Errorf("initialize codex app-server: %w", err)
	}
	if _, err := readCodexRPCResponse(reader, 1, maxRPCBytes); err != nil {
		return nil, fmt.Errorf("initialize codex app-server: %w", err)
	}
	if _, err := io.WriteString(stdin, "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n"); err != nil {
		return nil, fmt.Errorf("initialize codex app-server: %w", err)
	}

	threads := make([]CodexThread, 0)
	cursor := ""
	seenCursors := map[string]bool{}
	for page := 0; ; page++ {
		params := map[string]any{"useStateDbOnly": true, "archived": false, "limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		id := page + 2
		if err := writeCodexRequest(stdin, codexRPCRequest{JSONRPC: "2.0", ID: id, Method: "thread/list", Params: params}); err != nil {
			return nil, fmt.Errorf("list codex threads: %w", err)
		}
		response, err := readCodexRPCResponse(reader, id, maxRPCBytes)
		if err != nil {
			return nil, fmt.Errorf("list codex threads: %w", err)
		}
		var result codexThreadListResult
		if err := json.Unmarshal(response.Result, &result); err != nil {
			return nil, fmt.Errorf("decode codex thread list: %w", err)
		}
		for _, thread := range result.Data {
			if strings.TrimSpace(thread.ID) != "" {
				threads = append(threads, thread)
			}
		}
		if result.NextCursor == nil || *result.NextCursor == "" {
			return threads, nil
		}
		if seenCursors[*result.NextCursor] {
			return nil, fmt.Errorf("codex thread list repeated cursor %q", *result.NextCursor)
		}
		seenCursors[*result.NextCursor] = true
		cursor = *result.NextCursor
	}
}

func writeCodexRequest(writer io.Writer, request codexRPCRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = writer.Write(data)
	return err
}

func readCodexRPCResponse(reader *bufio.Reader, id int, maxBytes int) (codexRPCResponse, error) {
	for {
		line, err := readCodexLine(reader, maxBytes)
		if err != nil {
			return codexRPCResponse{}, err
		}
		var response codexRPCResponse
		if err := json.Unmarshal(line, &response); err != nil {
			return codexRPCResponse{}, fmt.Errorf("invalid JSON-RPC response: %w", err)
		}
		if (response.JSONRPC != "" && response.JSONRPC != "2.0") || string(response.ID) == "null" || len(response.ID) == 0 {
			continue // Ignore notifications and unrelated server messages.
		}
		var responseID int
		if err := json.Unmarshal(response.ID, &responseID); err != nil || responseID != id {
			continue
		}
		if response.Error != nil {
			return codexRPCResponse{}, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
		}
		if len(response.Result) == 0 || string(response.Result) == "null" {
			return codexRPCResponse{}, errors.New("JSON-RPC response has no result")
		}
		return response, nil
	}
}

func readCodexLine(reader *bufio.Reader, maxBytes int) ([]byte, error) {
	line := make([]byte, 0, min(maxBytes, 4096))
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > maxBytes {
			return nil, fmt.Errorf("codex app-server response exceeds %d bytes", maxBytes)
		}
		line = append(line, part...)
		if err == nil {
			return []byte(strings.TrimSpace(string(line))), nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}
