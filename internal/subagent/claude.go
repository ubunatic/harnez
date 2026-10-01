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
	b, err := providerOutput(ctx, cmd)
	if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
		return b, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return b, err
}
func (d ClaudeDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	start := time.Now()
	args := []string{"-p", "--dangerously-skip-permissions", "--model", o.Model.Name}
	if o.Model.Tier != "" {
		args = append(args, "--effort", claudeEffort(o.Model.Tier))
	}
	args = append(args, "--output-format", "stream-json", "--verbose", o.Prompt)
	b, err := d.command(ctx, args...)
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
	args := []string{"-p", "--resume", id, "--dangerously-skip-permissions", "--model", model.Name}
	if model.Tier != "" {
		args = append(args, "--effort", claudeEffort(model.Tier))
	}
	args = append(args, "--output-format", "stream-json", "--verbose", prompt)
	b, err := d.command(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("claude resume: %w", err)
	}
	return parseClaude(b)
}

func claudeEffort(tier string) string {
	if tier == "med" {
		return "medium"
	}
	return tier
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

// claudeUsage is the token usage block of a Claude message or result.
type claudeUsage struct {
	Input       int `json:"input_tokens"`
	Output      int `json:"output_tokens"`
	CacheRead   int `json:"cache_read_input_tokens"`
	CacheCreate int `json:"cache_creation_input_tokens"`
}

// claudeEvent is one stream-json line, or the single object of --output-format json.
type claudeEvent struct {
	Type      string      `json:"type"`
	Subtype   string      `json:"subtype"`
	SessionID string      `json:"session_id"`
	Result    string      `json:"result"`
	IsError   bool        `json:"is_error"`
	Usage     claudeUsage `json:"usage"`
	Parent    *string     `json:"parent_tool_use_id"`
	Message   struct {
		Usage   *claudeUsage    `json:"usage"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Compact struct {
		PostTokens int `json:"post_tokens"`
	} `json:"compact_metadata"`
}

// parseClaude reads Claude's stream-json output (one event per line) or a
// single --output-format json object. The result's usage totals every model
// call of the turn, so the context size comes from the last main-thread
// assistant call, or from a compact boundary's post_tokens; when neither is
// present it is unknown (-1), as for agy (issues 644, 673).
func parseClaude(data []byte) (*TurnResult, error) {
	var res *claudeEvent
	var last *claudeUsage
	postCompact := -1
	var localOut []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e claudeEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			if strings.HasPrefix(line, "{") && !strings.HasSuffix(line, "}") {
				return nil, fmt.Errorf("parse claude json: %w", err)
			}
			continue
		}
		switch {
		case e.Type == "assistant" && e.Message.Usage != nil && e.Parent == nil:
			// Synthetic events after /compact carry all-zero usage; skip them.
			if u := e.Message.Usage; u.Input+u.CacheRead+u.CacheCreate > 0 {
				last = u
			}
		case e.Type == "system" && e.Subtype == "compact_boundary" && e.Compact.PostTokens > 0:
			postCompact = e.Compact.PostTokens
			last = nil
		case e.Type == "user":
			// A local command such as /compact reports only here, not in result.
			var text string
			if json.Unmarshal(e.Message.Content, &text) == nil {
				if _, rest, ok := strings.Cut(text, "<local-command-stdout>"); ok {
					out, _, _ := strings.Cut(rest, "</local-command-stdout>")
					localOut = append(localOut, strings.TrimSpace(out))
				}
			}
		case e.Type == "result" || e.Type == "":
			res = &e
		}
	}
	if res == nil {
		return nil, fmt.Errorf("parse claude json: no result event")
	}
	if res.IsError {
		return nil, fmt.Errorf("claude reported an error: %s", res.Result)
	}
	v := res.Usage
	r := &TurnResult{SessionID: res.SessionID, Response: res.Result, Messages: []string{res.Result}, InputTokens: v.Input, OutputTokens: v.Output, CachedTokens: v.CacheRead + v.CacheCreate}
	r.Messages = append(r.Messages, localOut...)
	r.ContextTokens = -1
	if last != nil {
		r.ContextTokens = last.Input + last.CacheRead + last.CacheCreate
	} else if postCompact > 0 {
		r.ContextTokens = postCompact
	}
	r.TokensTurn = r.InputTokens + r.OutputTokens + r.CachedTokens + r.ReasoningTokens
	r.TokensCumulative = r.TokensTurn
	return r, nil
}

var _ Driver = ClaudeDriver{}
