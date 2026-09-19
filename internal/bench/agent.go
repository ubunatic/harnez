package bench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Agent names supported by the runner.
const (
	AgentClaude = "claude"
	AgentCodex  = "codex"
)

// Default cheap models per agent; "luna" and "haiku" are accepted aliases.
var defaultModels = map[string]string{AgentClaude: "haiku", AgentCodex: "gpt-5.6-luna"}

// ResolveModel maps an empty model or alias to the agent's concrete model flag value.
func ResolveModel(agent, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return defaultModels[agent]
	}
	if model == "luna" {
		return "gpt-5.6-luna"
	}
	return model
}

// Result is one agent invocation's outcome.
type Result struct {
	Text         string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// CommandRunner runs name with args in dir and returns stdout. Tests fake it.
type CommandRunner func(ctx context.Context, dir, name string, args ...string) ([]byte, error)

// ExecRunner is the real CommandRunner.
func ExecRunner(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Invoke sends prompt to the agent in dir using model and parses its JSON output.
func Invoke(ctx context.Context, run CommandRunner, agent, model, dir, prompt string) (Result, error) {
	model = ResolveModel(agent, model)
	switch agent {
	case AgentClaude:
		out, err := run(ctx, dir, "claude", "-p", "--model", model, "--dangerously-skip-permissions", "--output-format", "json", prompt)
		if err != nil {
			return Result{}, err
		}
		return ParseClaude(out)
	case AgentCodex:
		out, err := run(ctx, dir, "codex", "exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "-m", model, "--json", prompt)
		if err != nil {
			return Result{}, err
		}
		return ParseCodex(out)
	}
	return Result{}, fmt.Errorf("bench: unsupported agent %q (supported: claude, codex)", agent)
}

// ParseClaude reads `claude -p --output-format json` output. Input tokens
// include cache reads and cache creation so runs compare on total context.
func ParseClaude(out []byte) (Result, error) {
	var raw struct {
		Result  string  `json:"result"`
		IsError bool    `json:"is_error"`
		Cost    float64 `json:"total_cost_usd"`
		Usage   struct {
			Input       int `json:"input_tokens"`
			Output      int `json:"output_tokens"`
			CacheRead   int `json:"cache_read_input_tokens"`
			CacheCreate int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &raw); err != nil {
		return Result{}, fmt.Errorf("bench: parse claude json: %w", err)
	}
	if raw.IsError {
		return Result{}, fmt.Errorf("bench: claude reported an error: %s", raw.Result)
	}
	return Result{
		Text:         raw.Result,
		InputTokens:  raw.Usage.Input + raw.Usage.CacheRead + raw.Usage.CacheCreate,
		OutputTokens: raw.Usage.Output,
		CostUSD:      raw.Cost,
	}, nil
}

// ParseCodex reads `codex exec --json` JSONL: the last agent_message is the
// answer, usage sums every turn.completed event.
func ParseCodex(out []byte) (Result, error) {
	var res Result
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
			Usage struct {
				Input  int `json:"input_tokens"`
				Output int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		switch {
		case ev.Type == "item.completed" && ev.Item.Type == "agent_message":
			res.Text = ev.Item.Text
		case ev.Type == "turn.completed":
			res.InputTokens += ev.Usage.Input
			res.OutputTokens += ev.Usage.Output
		}
	}
	if res.Text == "" {
		return Result{}, fmt.Errorf("bench: codex output has no agent_message")
	}
	return res, nil
}
