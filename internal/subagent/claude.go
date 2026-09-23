package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ClaudeDriver runs Claude Code in print mode.
type ClaudeDriver struct {
	Command func(context.Context, string, ...string) ([]byte, error)
	Dir     string
}

func (d ClaudeDriver) command(ctx context.Context, args ...string) ([]byte, error) {
	if d.Command != nil {
		return d.Command(ctx, "claude", args...)
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = d.Dir
	b, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
		return b, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return b, err
}
func (d ClaudeDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	start := time.Now()
	b, err := d.command(ctx, "-p", "--dangerously-skip-permissions", "--model", o.Model.Name, "--output-format", "json", o.Prompt)
	if err != nil {
		return nil, fmt.Errorf("claude: %w", err)
	}
	r, err := parseClaude(b)
	if err != nil {
		return nil, err
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}
func (d ClaudeDriver) Resume(ctx context.Context, id, prompt string, model Model) (*TurnResult, error) {
	b, err := d.command(ctx, "-p", "--resume", id, "--dangerously-skip-permissions", "--model", model.Name, "--output-format", "json", prompt)
	if err != nil {
		return nil, fmt.Errorf("claude resume: %w", err)
	}
	return parseClaude(b)
}
func (d ClaudeDriver) Compact(ctx context.Context, id string) (*TurnResult, error) {
	return d.Resume(ctx, id, "/compact", Model{})
}
func (d ClaudeDriver) Stop(ctx context.Context, id string) error {
	return nil
}
func (d ClaudeDriver) Delete(ctx context.Context, id string) error {
	return nil
}
func parseClaude(data []byte) (*TurnResult, error) {
	var v struct {
		SessionID string `json:"session_id"`
		Result    string `json:"result"`
		IsError   bool   `json:"is_error"`
		Usage     struct {
			Input       int `json:"input_tokens"`
			Output      int `json:"output_tokens"`
			CacheRead   int `json:"cache_read_input_tokens"`
			CacheCreate int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse claude json: %w", err)
	}
	if v.IsError {
		return nil, fmt.Errorf("claude reported an error: %s", v.Result)
	}
	r := &TurnResult{SessionID: v.SessionID, Response: v.Result, Messages: []string{v.Result}, InputTokens: v.Usage.Input, OutputTokens: v.Usage.Output, CachedTokens: v.Usage.CacheRead + v.Usage.CacheCreate}
	r.TokensTurn = r.InputTokens + r.OutputTokens + r.CachedTokens
	r.TokensCumulative = r.TokensTurn
	return r, nil
}

var _ Driver = ClaudeDriver{}
