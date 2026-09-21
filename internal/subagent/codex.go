package subagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// CodexDriver runs codex in non-interactive JSONL mode.
type CodexDriver struct {
	Command func(context.Context, string, ...string) ([]byte, error)
}

func (d CodexDriver) command(ctx context.Context, args ...string) ([]byte, error) {
	if d.Command != nil {
		return d.Command(ctx, "codex", args...)
	}
	c := exec.CommandContext(ctx, "codex", args...)
	return c.Output()
}
func (d CodexDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	start := time.Now()
	args := []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox", "-m", o.Model.Name, o.Prompt}
	b, err := d.command(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("codex exec: %w", err)
	}
	r, err := parseCodex(b)
	if err != nil {
		return nil, err
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}
func (d CodexDriver) Resume(ctx context.Context, id, prompt string) (*TurnResult, error) {
	return d.runResume(ctx, id, prompt)
}
func (d CodexDriver) runResume(ctx context.Context, id, prompt string) (*TurnResult, error) {
	b, err := d.command(ctx, "exec", "resume", id, "--json", prompt)
	if err != nil {
		return nil, fmt.Errorf("codex resume: %w", err)
	}
	return parseCodex(b)
}
func (d CodexDriver) Compact(ctx context.Context, id string) (*TurnResult, error) {
	if _, err := d.command(ctx, "queue", "--thread", id, "--message", "/compact"); err != nil {
		return nil, fmt.Errorf("codex compact: %w", err)
	}
	return &TurnResult{Response: "compaction queued"}, nil
}
func (d CodexDriver) Stop(context.Context, string) error {
	return nil
}
func (d CodexDriver) Delete(ctx context.Context, id string) error {
	if _, err := d.command(ctx, "delete", "--force", id); err != nil {
		return fmt.Errorf("codex delete: %w", err)
	}
	return nil
}

func parseCodex(data []byte) (*TurnResult, error) {
	r := &TurnResult{}
	s := bufio.NewScanner(bytes.NewReader(data))
	s.Buffer(make([]byte, 4096), 8<<20)
	for s.Scan() {
		var e struct {
			Type     string                      `json:"type"`
			ThreadID string                      `json:"thread_id"`
			Item     struct{ Type, Text string } `json:"item"`
			Usage    struct {
				Input   int `json:"input_tokens"`
				Output  int `json:"output_tokens"`
				Cached  int `json:"cached_input_tokens"`
				Details struct {
					Cached int `json:"cached_input_tokens"`
				} `json:"input_token_details"`
			} `json:"usage"`
		}
		if json.Unmarshal(s.Bytes(), &e) != nil {
			continue
		}
		if e.ThreadID != "" {
			r.SessionID = e.ThreadID
		}
		if e.Item.Type == "agent_message" {
			r.Response = e.Item.Text
			r.Messages = append(r.Messages, e.Item.Text)
		}
		if e.Type == "turn.completed" {
			r.InputTokens += e.Usage.Input
			r.OutputTokens += e.Usage.Output
			r.CachedTokens += e.Usage.Cached
			if e.Usage.Details.Cached > r.CachedTokens {
				r.CachedTokens = e.Usage.Details.Cached
			}
		}
	}
	if r.Response == "" {
		return nil, fmt.Errorf("codex output has no agent_message")
	}
	r.TokensTurn = r.InputTokens + r.OutputTokens
	r.TokensCumulative = r.TokensTurn
	return r, nil
}

var _ Driver = CodexDriver{}
