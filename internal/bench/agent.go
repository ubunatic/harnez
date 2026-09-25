package bench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/subagent"
)

// Agent names supported by the runner.
const (
	AgentClaude = "claude"
	AgentCodex  = "codex"
	AgentAgy    = "agy"
)

// provider defines the command line and output parser for one agent CLI.
// Adding a provider requires one entry here and its output parser.
type provider struct {
	defaultModel string
	command      string
	args         func(model, prompt string) []string
	parse        func([]byte) (Result, error)
}

var providers = map[string]provider{
	AgentClaude: {
		defaultModel: "haiku", command: "claude", parse: ParseClaude,
		args: func(model, prompt string) []string {
			return []string{"-p", "--model", model, "--dangerously-skip-permissions", "--output-format", "json", prompt}
		},
	},
	AgentCodex: {
		defaultModel: "gpt-6-luna", command: "codex", parse: ParseCodex,
		args: func(model, prompt string) []string {
			return []string{"exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", "-m", model, "--json", prompt}
		},
	},
	AgentAgy: {
		defaultModel: "gemini-3.7-flash", command: "agy", parse: ParseAgy,
		args: func(model, prompt string) []string {
			return []string{"-p", prompt, "--model", model, "--effort", "low", "--dangerously-skip-permissions", "--output-format", "json"}
		},
	},
}

// ResolveModel maps an empty model or alias to the agent's concrete model flag value.
func ResolveModel(agent, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return providers[agent].defaultModel
	}
	switch {
	case model == "luna":
		return "gpt-6-luna"
	case model == "flash" && agent == AgentAgy:
		return providers[AgentAgy].defaultModel
	case model == "flash37" && agent == AgentAgy:
		return "gemini-3.7-flash"
	case model == "flash38" && agent == AgentAgy:
		return "gemini-3.8-flash"
	}
	return model
}

// Result is one agent invocation's outcome.
type Result struct {
	Text         string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Turns        int // agent steps: tool calls plus the final answer
}

// CommandRunner runs name with args in dir and returns stdout. Tests fake it.
type CommandRunner func(ctx context.Context, dir, name string, args ...string) ([]byte, error)

// ExecRunner is the real CommandRunner.
func ExecRunner(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if name == "agy" {
		return execMeteredAgy(ctx, dir, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	// Select the agent's harnez read profile (image vs text cost estimates).
	cmd.Env = append(os.Environ(), "HARNEZ_AGENT_HARNESS="+name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

type agySessionContextKey struct{}

func execMeteredAgy(ctx context.Context, dir string, args ...string) ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("bench: resolve home directory: %w", err)
	}
	sessionID, _ := ctx.Value(agySessionContextKey{}).(string)
	if sessionID == "" {
		return nil, fmt.Errorf("bench: agy invocation is missing its meter session id")
	}
	env := subagent.AgyLaunchEnv(os.Environ(), home)
	env = setEnvValue(env, "HARNEZ_SESSION_ID", sessionID)
	env = setEnvValue(env, "HARNEZ_AGY_METER_SESSION_ID", sessionID)
	var stdout, stderr bytes.Buffer
	err = agymeter.RunWithEnvDir(ctx, home, "agy", args, env, dir, nil, &stdout, &stderr)
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("agy: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func setEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}

func newMeterSessionID() string { return uuid.NewString() }

func readAgyUsage(sessionID string) ([]agymeter.Record, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".harnez", "agymeter", "usage.jsonl")
	return agymeter.ReadUsageRecords(path, sessionID)
}

func aggregateAgyUsage(rows []agymeter.Record) (input, total, turns int) {
	for _, row := range rows {
		input += int(row.Prompt)
		total += int(row.Total)
		turns++
	}
	return input, total, turns
}

func splitAgyUsage(rows []agymeter.Record) (input, total, turns int, calls []int64, helpers []HelperUsage) {
	promptByModel := make(map[string]int64)
	for _, row := range rows {
		promptByModel[row.Model] += row.Prompt
	}
	main := ""
	var maxPrompt int64 = -1
	for _, row := range rows {
		if promptByModel[row.Model] > maxPrompt {
			main, maxPrompt = row.Model, promptByModel[row.Model]
		}
	}
	helperMap := make(map[string]*HelperUsage)
	for _, row := range rows {
		if row.Model == main {
			input += int(row.Prompt)
			total += int(row.Total)
			turns++
			calls = append(calls, row.Prompt)
			continue
		}
		h := helperMap[row.Model]
		if h == nil {
			h = &HelperUsage{Model: row.Model}
			helperMap[row.Model] = h
		}
		h.Calls++
		h.InputTokens += row.Prompt
		h.TotalTokens += row.Total
	}
	for _, row := range rows {
		if h := helperMap[row.Model]; h != nil {
			found := false
			for _, existing := range helpers {
				if existing.Model == h.Model {
					found = true
					break
				}
			}
			if !found {
				helpers = append(helpers, *h)
			}
		}
	}
	return
}

// Invoke sends prompt to the agent in dir using model and parses its JSON output.
func Invoke(ctx context.Context, run CommandRunner, agent, model, dir, prompt string) (Result, error) {
	p, ok := providers[agent]
	if !ok {
		return Result{}, fmt.Errorf("bench: unsupported agent %q (supported: claude, codex, agy)", agent)
	}
	model = ResolveModel(agent, model)
	out, err := run(ctx, dir, p.command, p.args(model, prompt)...)
	if err != nil {
		return Result{}, err
	}
	return p.parse(out)
}

// ParseClaude reads `claude -p --output-format json` output. Input tokens
// include cache reads and cache creation so runs compare on total context.
func ParseClaude(out []byte) (Result, error) {
	var raw struct {
		Result  string  `json:"result"`
		IsError bool    `json:"is_error"`
		Turns   int     `json:"num_turns"`
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
		Turns:        raw.Turns,
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
		case ev.Type == "item.completed" && ev.Item.Type != "reasoning":
			res.Turns++ // a tool call
		case ev.Type == "turn.completed":
			res.InputTokens += ev.Usage.Input
			res.OutputTokens += ev.Usage.Output
		}
	}
	res.Turns++ // the final answer
	if res.Text == "" {
		return Result{}, fmt.Errorf("bench: codex output has no agent_message")
	}
	return res, nil
}

// ParseAgy reads `agy -p --output-format json` output. Input tokens include
// cache reads and output tokens include thinking, so runs compare on totals.
func ParseAgy(out []byte) (Result, error) {
	var raw struct {
		Status   string `json:"status"`
		Response string `json:"response"`
		Turns    int    `json:"num_turns"`
		Usage    struct {
			Input     int `json:"input_tokens"`
			Output    int `json:"output_tokens"`
			Thinking  int `json:"thinking_tokens"`
			CacheRead int `json:"cache_read_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &raw); err != nil {
		return Result{}, fmt.Errorf("bench: parse agy json: %w", err)
	}
	if raw.Status != "SUCCESS" {
		return Result{}, fmt.Errorf("bench: agy status %q: %s", raw.Status, raw.Response)
	}
	return Result{
		Text:         raw.Response,
		InputTokens:  raw.Usage.Input + raw.Usage.CacheRead,
		OutputTokens: raw.Usage.Output + raw.Usage.Thinking,
		Turns:        raw.Turns,
	}, nil
}
