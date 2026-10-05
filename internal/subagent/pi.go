package subagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// PiDriver runs the Pi coding-agent CLI in JSON mode. Pi owns persisted
// sessions; the provider session ID is the ID from its JSON session header.
type PiDriver struct {
	Command      func(context.Context, string, ...string) ([]byte, error)
	CommandInput func(context.Context, string, string, ...string) ([]byte, error)
	Dir          string
}

func (d PiDriver) command(ctx context.Context, args ...string) ([]byte, error) {
	if d.Command != nil {
		return d.Command(ctx, "pi", args...)
	}
	cmd := exec.CommandContext(ctx, "pi", args...)
	cmd.Dir = d.Dir
	b, err := providerOutput(ctx, cmd)
	if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
		return b, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return b, err
}

func (d PiDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	start := time.Now()
	b, err := d.command(ctx, piRunArgs(o.Model, o.Prompt)...)
	if err != nil {
		return nil, fmt.Errorf("pi: %w", err)
	}
	r, err := parsePi(b)
	if err != nil {
		return nil, err
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}

func (d PiDriver) Resume(ctx context.Context, id, prompt string, model Model) (*TurnResult, error) {
	b, err := d.command(ctx, piResumeArgs(id, prompt, model)...)
	if err != nil {
		return nil, fmt.Errorf("pi resume: %w", err)
	}
	return parsePi(b)
}

func piRunArgs(model Model, prompt string) []string {
	args := []string{"--mode", "json"}
	args = append(args, piModelArgs(model)...)
	return append(args, "--", prompt)
}

func piResumeArgs(id, prompt string, model Model) []string {
	args := []string{"--mode", "json", "--session", id}
	args = append(args, piModelArgs(model)...)
	return append(args, "--", prompt)
}

func piModelArgs(model Model) []string {
	var args []string
	if model.Name != "" && model.Name != "default" {
		args = append(args, "--model", model.Name)
	}
	if model.Tier != "" {
		args = append(args, "--thinking", piThinkingLevel(model.Tier))
	}
	return args
}

func piThinkingLevel(tier string) string {
	if tier == "med" {
		return "medium"
	}
	return tier
}

func (d PiDriver) Compact(ctx context.Context, id string) (*TurnResult, error) {
	start := time.Now()
	input := "{\"id\":\"harnez-compact\",\"type\":\"compact\"}\n"
	var b []byte
	var err error
	if d.CommandInput != nil {
		b, err = d.CommandInput(ctx, "pi", input, piCompactArgs(id)...)
	} else if d.Command != nil {
		err = fmt.Errorf("injected Pi command does not support interactive RPC input")
	} else {
		b, err = d.runPiCompactRPC(ctx, input, id)
	}
	if err != nil {
		return nil, fmt.Errorf("pi compact: %w", err)
	}
	result, err := parsePiCompact(b)
	if err != nil {
		return nil, err
	}
	result.SessionID = id
	result.DurationMS = time.Since(start).Milliseconds()
	return result, nil
}

func piCompactArgs(id string) []string {
	return []string{"--mode", "rpc", "--session", id}
}

// runPiCompactRPC keeps stdin open until Pi answers the compact command. EOF
// requests RPC shutdown, so closing it immediately after writing would abort
// Pi's asynchronous summarization before it can finish.
func (d PiDriver) runPiCompactRPC(ctx context.Context, input, id string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "pi", piCompactArgs(id)...)
	cmd.Dir = d.Dir
	isolateProcess(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := observeProcess(ctx, cmd); err != nil {
		_ = KillGroup(cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		return nil, err
	}
	if _, err := io.WriteString(stdin, input); err != nil {
		_ = stdin.Close()
		_ = KillGroup(cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		return nil, fmt.Errorf("write Pi RPC compact command: %w", err)
	}
	var output bytes.Buffer
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		output.Write(line)
		output.WriteByte('\n')
		if isPiCompactResponse(line) {
			_ = stdin.Close()
		}
	}
	if err := scanner.Err(); err != nil {
		_ = stdin.Close()
		_ = KillGroup(cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		return output.Bytes(), fmt.Errorf("read Pi RPC output: %w", err)
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(stderr.Bytes()) > 0 {
			exitErr.Stderr = stderr.Bytes()
			return output.Bytes(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return output.Bytes(), err
	}
	return output.Bytes(), nil
}

func isPiCompactResponse(line []byte) bool {
	var response piRPCResponse
	return json.Unmarshal(line, &response) == nil && response.Type == "response" && response.ID == "harnez-compact" && response.Command == "compact"
}

func (PiDriver) Stop(context.Context, string) error   { return nil }
func (PiDriver) Delete(context.Context, string) error { return nil }

// piEvent is the subset of Pi's JSONL protocol needed to normalize a turn.
type piEvent struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Message struct {
		Role         string          `json:"role"`
		Content      json.RawMessage `json:"content"`
		StopReason   string          `json:"stopReason"`
		ErrorMessage string          `json:"errorMessage"`
		Usage        struct {
			Input      int `json:"input"`
			Output     int `json:"output"`
			CacheRead  int `json:"cacheRead"`
			CacheWrite int `json:"cacheWrite"`
			Total      int `json:"totalTokens"`
		} `json:"usage"`
	} `json:"message"`
}

type piTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// parsePi consumes the documented JSONL event stream and uses completed
// assistant messages (not streaming deltas) as authoritative response text.
func parsePi(data []byte) (*TurnResult, error) {
	result := &TurnResult{ContextTokens: -1}
	var foundSession bool
	var foundAssistant bool
	var foundSettled bool
	var assistantError string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var event piEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("parse pi JSONL event %d: %w", line, err)
		}
		switch event.Type {
		case "agent_settled":
			foundSettled = true
		case "session":
			result.SessionID = event.ID
			foundSession = event.ID != ""
		case "message_end":
			if event.Message.Role != "assistant" {
				continue
			}
			foundAssistant = true
			if event.Message.StopReason == "error" || event.Message.StopReason == "aborted" || event.Message.StopReason == "length" {
				message := event.Message.ErrorMessage
				if message == "" {
					message = event.Message.StopReason
				}
				assistantError = fmt.Sprintf("pi reported %s: %s", event.Message.StopReason, message)
			} else {
				// Pi can emit a failed assistant message before retry or
				// compaction recovery. Only the final assistant result decides
				// whether a settled invocation failed.
				assistantError = ""
			}
			var blocks []piTextBlock
			if len(event.Message.Content) > 0 && string(event.Message.Content) != "null" {
				if err := json.Unmarshal(event.Message.Content, &blocks); err != nil {
					return nil, fmt.Errorf("parse pi assistant content: %w", err)
				}
			}
			var text strings.Builder
			for _, block := range blocks {
				if block.Type == "text" {
					text.WriteString(block.Text)
				}
			}
			response := text.String()
			result.Messages = append(result.Messages, response)
			result.Response = response
			usage := event.Message.Usage
			result.InputTokens += usage.Input
			result.OutputTokens += usage.Output
			result.CachedTokens += usage.CacheRead + usage.CacheWrite
			result.TokensTurn += usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
			result.TokensCumulative = result.TokensTurn
			if usage.Total != 0 || usage.Input != 0 || usage.Output != 0 || usage.CacheRead != 0 || usage.CacheWrite != 0 {
				result.ContextTokens = usage.Input + usage.CacheRead + usage.CacheWrite
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pi JSONL output: %w", err)
	}
	if !foundSession {
		return nil, fmt.Errorf("parse pi JSONL: no session header")
	}
	if !foundAssistant {
		return nil, fmt.Errorf("parse pi JSONL: no completed assistant message")
	}
	if !foundSettled {
		return nil, fmt.Errorf("parse pi JSONL: run did not settle")
	}
	if assistantError != "" {
		return nil, fmt.Errorf("%s", assistantError)
	}
	return result, nil
}

// piRPCResponse is the documented command response envelope used by Pi RPC.
type piRPCResponse struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Command string `json:"command"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Data    struct {
		EstimatedTokensAfter int `json:"estimatedTokensAfter"`
		Usage                struct {
			Input      int `json:"input"`
			Output     int `json:"output"`
			CacheRead  int `json:"cacheRead"`
			CacheWrite int `json:"cacheWrite"`
		} `json:"usage"`
	} `json:"data"`
}

// parsePiCompact reads the correlated RPC compact response. Pi exposes an
// estimate immediately after compaction; exact context usage becomes available
// only after a later assistant response.
func parsePiCompact(data []byte) (*TurnResult, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		var response piRPCResponse
		if err := json.Unmarshal(bytes.TrimSpace(scanner.Bytes()), &response); err != nil {
			return nil, fmt.Errorf("parse pi RPC response %d: %w", line, err)
		}
		if response.Type != "response" || response.ID != "harnez-compact" || response.Command != "compact" {
			continue
		}
		if !response.Success {
			return nil, fmt.Errorf("pi compact failed: %s", response.Error)
		}
		after := response.Data.EstimatedTokensAfter
		if after <= 0 {
			return nil, fmt.Errorf("pi compact returned no positive estimatedTokensAfter")
		}
		usage := response.Data.Usage
		result := &TurnResult{
			Response:         fmt.Sprintf("Pi compact completed; estimated context after compaction: %d tokens", after),
			Messages:         []string{fmt.Sprintf("Pi compact completed; estimated context after compaction: %d tokens", after)},
			InputTokens:      usage.Input,
			OutputTokens:     usage.Output,
			CachedTokens:     usage.CacheRead + usage.CacheWrite,
			ContextTokens:    after,
			TokensTurn:       usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite,
			TokensCumulative: usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite,
		}
		return result, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pi RPC output: %w", err)
	}
	return nil, fmt.Errorf("parse pi RPC output: no compact response")
}

var _ Driver = PiDriver{}
