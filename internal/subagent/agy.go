package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// AgyDriver runs agy (Antigravity CLI) in print mode, pinned to Dir.
type AgyDriver struct {
	Command func(context.Context, string, ...string) ([]byte, error)
	Dir     string
}

func (d AgyDriver) command(ctx context.Context, args ...string) ([]byte, error) {
	if d.Command != nil {
		return d.Command(ctx, "agy", args...)
	}
	c := exec.CommandContext(ctx, "agy", args...)
	c.Dir = d.Dir
	return c.Output()
}

func agyEffort(tier string) string {
	if tier == "med" {
		return "medium"
	}
	return tier
}

func (d AgyDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	start := time.Now()
	args := []string{"--add-dir", d.Dir, "--model", o.Model.Name, "--effort", agyEffort(o.Model.Tier), "--output-format", "json", "-p", o.Prompt}
	b, err := d.command(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("agy: %w", err)
	}
	r, err := parseAgy(b)
	if err != nil {
		return nil, err
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}

func (d AgyDriver) Resume(ctx context.Context, id, prompt string, model Model) (*TurnResult, error) {
	args := []string{"--conversation", id, "--add-dir", d.Dir, "--model", model.Name, "--effort", agyEffort(model.Tier), "--output-format", "json", "-p", prompt}
	b, err := d.command(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("agy resume: %w", err)
	}
	return parseAgy(b)
}

func (d AgyDriver) Compact(ctx context.Context, id string) (*TurnResult, error) {
	return d.Resume(ctx, id, "/compact", Model{})
}

func (d AgyDriver) Stop(ctx context.Context, id string) error {
	return nil
}

func (d AgyDriver) Delete(ctx context.Context, id string) error {
	return nil
}

func parseAgy(data []byte) (*TurnResult, error) {
	var v struct {
		ConversationID string  `json:"conversation_id"`
		Status         string  `json:"status"`
		Response       string  `json:"response"`
		DurationSecs   float64 `json:"duration_seconds"`
		NumTurns       int     `json:"num_turns"`
		Usage          struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
			Think  int `json:"thinking_tokens"`
			Cache  int `json:"cache_read_tokens"`
			Total  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse agy json: %w", err)
	}
	if v.Status != "SUCCESS" {
		return nil, fmt.Errorf("agy reported status %q: %s", v.Status, v.Response)
	}
	r := &TurnResult{
		SessionID:    v.ConversationID,
		Response:     v.Response,
		Messages:     []string{v.Response},
		InputTokens:  v.Usage.Input,
		OutputTokens: v.Usage.Output + v.Usage.Think,
		CachedTokens: v.Usage.Cache,
	}
	r.TokensTurn = r.InputTokens + r.OutputTokens + r.CachedTokens
	r.TokensCumulative = r.TokensTurn
	return r, nil
}

var _ Driver = AgyDriver{}
