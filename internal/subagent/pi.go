package subagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PiDriver runs the Pi coding-agent CLI in JSON mode. Pi owns persisted
// sessions; the provider session ID is the ID from its JSON session header.
type PiDriver struct {
	Command func(context.Context, string, ...string) ([]byte, error)
	Dir     string
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
	return d.Resume(ctx, id, "/compact", Model{})
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
			if event.Message.StopReason == "error" || event.Message.StopReason == "aborted" {
				message := event.Message.ErrorMessage
				if message == "" {
					message = event.Message.StopReason
				}
				return nil, fmt.Errorf("pi reported %s: %s", event.Message.StopReason, message)
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
	return result, nil
}

var _ Driver = PiDriver{}
