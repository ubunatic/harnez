package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ClaudeDriver runs Claude Code in print mode.
type ClaudeDriver struct {
	Command func(context.Context, string, ...string) ([]byte, error)
	Dir     string
}

func (d ClaudeDriver) command(ctx context.Context, args ...string) ([]byte, error) {
	return d.commandLines(ctx, nil, args...)
}

func (d ClaudeDriver) commandLines(ctx context.Context, fn func([]byte), args ...string) ([]byte, error) {
	return runProviderCLI(ctx, "claude", d.Dir, d.Command, fn, args)
}

func (d ClaudeDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	start := time.Now()
	b, err := d.command(ctx, claudeRunArgs(o.Model, o.Prompt)...)
	if err != nil {
		if _, parseErr := parseClaude(b); parseErr != nil && (strings.Contains(parseErr.Error(), "claude reported an error:") || strings.HasPrefix(parseErr.Error(), "claude: ")) {
			return nil, parseErr
		}
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
	b, err := d.command(ctx, claudeResumeArgs(id, prompt, model)...)
	if err != nil {
		if _, parseErr := parseClaude(b); parseErr != nil && (strings.Contains(parseErr.Error(), "claude reported an error:") || strings.HasPrefix(parseErr.Error(), "claude: ")) {
			return nil, parseErr
		}
		return nil, fmt.Errorf("claude resume: %w", err)
	}
	return parseClaude(b)
}

// claudeRunArgs and claudeResumeArgs are shared by the batch and streaming
// paths: both read --output-format stream-json lines.
func claudeRunArgs(model Model, prompt string) []string {
	args := append([]string{"-p", "--dangerously-skip-permissions"}, claudeModelArgs(model)...)
	return append(args, "--output-format", "stream-json", "--verbose", prompt)
}

func claudeResumeArgs(id, prompt string, model Model) []string {
	args := append([]string{"-p", "--resume", id, "--dangerously-skip-permissions"}, claudeModelArgs(model)...)
	return append(args, "--output-format", "stream-json", "--verbose", prompt)
}

// claudeModelArgs omits empty values: claude rejects `--model ""` with a 400
// (issue 673, Compact passes an empty Model).
func claudeModelArgs(m Model) []string {
	var args []string
	if m.Name != "" {
		args = append(args, "--model", m.Name)
	}
	if m.Tier != "" {
		args = append(args, "--effort", claudeEffort(m.Tier))
	}
	return args
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
		PreTokens  int `json:"pre_tokens"`
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
	preCompact := 0
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
			preCompact = e.Compact.PreTokens
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
	// A failed /compact still reports subtype success, with the error as text.
	if strings.HasPrefix(strings.TrimSpace(res.Result), "Error during compaction") {
		return nil, fmt.Errorf("claude: %s", strings.TrimSpace(res.Result))
	}
	v := res.Usage
	r := &TurnResult{SessionID: res.SessionID, Response: res.Result, Messages: []string{res.Result}, InputTokens: v.Input, OutputTokens: v.Output, CachedTokens: v.CacheRead + v.CacheCreate}
	r.CompactionObserved = postCompact > 0
	r.ContextTokensBeforeCompact = preCompact
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

// ClaudeStreamDriver is ClaudeDriver plus live events for harnez tui. It is a
// separate type so batch callers (harnez agent start/resume) that type-assert
// StreamingDriver keep ClaudeDriver's batch behaviour.
type ClaudeStreamDriver struct{ ClaudeDriver }

// RunStream runs a new claude turn and reports events while it runs. The
// arguments and the final result are those of Run.
func (d ClaudeStreamDriver) RunStream(ctx context.Context, o RunOptions, fn EventFunc) (*TurnResult, error) {
	start := time.Now()
	b, err := d.commandLines(ctx, claudeLineFunc(fn), claudeRunArgs(o.Model, o.Prompt)...)
	if err != nil {
		if _, parseErr := parseClaude(b); parseErr != nil && (strings.Contains(parseErr.Error(), "claude reported an error:") || strings.HasPrefix(parseErr.Error(), "claude: ")) {
			return nil, parseErr
		}
		return nil, fmt.Errorf("claude: %w", err)
	}
	r, err := parseClaude(b)
	if err != nil {
		return nil, err
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}

// ResumeStream resumes a claude session and reports events while it runs.
func (d ClaudeStreamDriver) ResumeStream(ctx context.Context, id, prompt string, model Model, fn EventFunc) (*TurnResult, error) {
	b, err := d.commandLines(ctx, claudeLineFunc(fn), claudeResumeArgs(id, prompt, model)...)
	if err != nil {
		if _, parseErr := parseClaude(b); parseErr != nil && (strings.Contains(parseErr.Error(), "claude reported an error:") || strings.HasPrefix(parseErr.Error(), "claude: ")) {
			return nil, parseErr
		}
		return nil, fmt.Errorf("claude resume: %w", err)
	}
	return parseClaude(b)
}

var _ StreamingDriver = ClaudeStreamDriver{}

func claudeLineFunc(fn EventFunc) func([]byte) {
	if fn == nil {
		return nil
	}
	s := &claudeStreamer{}
	return func(line []byte) {
		for _, ev := range s.feed(line) {
			fn(ev)
		}
	}
}

// claudeStreamer maps stream-json lines to live events: the session id
// (init or result), main-thread assistant text and tool_use blocks, local
// command output, and context size. Malformed lines are skipped; the final
// TurnResult still comes from parseClaude over the whole output.
type claudeStreamer struct {
	session string
	texts   int
}

func (s *claudeStreamer) feed(line []byte) []Event {
	var e claudeEvent
	if json.Unmarshal(line, &e) != nil {
		return nil
	}
	var evs []Event
	if e.SessionID != "" && e.SessionID != s.session {
		s.session = e.SessionID
		evs = append(evs, Event{Kind: "session", Text: e.SessionID})
	}
	switch e.Type {
	case "assistant":
		if e.Parent != nil {
			break // subagent (Task) traffic stays out of the main transcript
		}
		var blocks []struct {
			Type  string                     `json:"type"`
			Text  string                     `json:"text"`
			Name  string                     `json:"name"`
			Input map[string]json.RawMessage `json:"input"`
		}
		_ = json.Unmarshal(e.Message.Content, &blocks)
		for _, b := range blocks {
			switch {
			case b.Type == "text" && strings.TrimSpace(b.Text) != "":
				s.texts++
				evs = append(evs, Event{Kind: "message", Text: b.Text})
			case b.Type == "tool_use" && b.Name != "":
				evs = append(evs, Event{Kind: "activity", Text: claudeToolActivity(b.Name, b.Input)})
			}
		}
		if u := e.Message.Usage; u != nil && u.Input+u.CacheRead+u.CacheCreate > 0 {
			evs = append(evs, Event{Kind: "tokens", ContextTokens: u.Input + u.CacheRead + u.CacheCreate})
		}
	case "user":
		var text string
		if json.Unmarshal(e.Message.Content, &text) == nil {
			if _, rest, ok := strings.Cut(text, "<local-command-stdout>"); ok {
				out, _, _ := strings.Cut(rest, "</local-command-stdout>")
				if out = strings.TrimSpace(out); out != "" {
					s.texts++
					evs = append(evs, Event{Kind: "message", Text: out})
				}
			}
		}
	case "result":
		// Normally the result repeats the last assistant text; report it only
		// when the turn produced no text so far.
		if s.texts == 0 && !e.IsError && strings.TrimSpace(e.Result) != "" {
			s.texts++
			evs = append(evs, Event{Kind: "message", Text: e.Result})
		}
	}
	if len(evs) == 0 {
		evs = append(evs, Event{Kind: "other"})
	}
	evs[0].Bytes = len(line)
	return evs
}

// claudeToolActivity describes a tool call in one line, e.g. "running ls".
func claudeToolActivity(name string, input map[string]json.RawMessage) string {
	if name == "Bash" {
		if cmd := claudeInputString(input, "command"); cmd != "" {
			return "running " + cmd
		}
	}
	for _, key := range []string{"file_path", "path", "pattern", "url", "query", "description"} {
		if v := claudeInputString(input, key); v != "" {
			return name + " " + v
		}
	}
	return name
}

func claudeInputString(input map[string]json.RawMessage, key string) string {
	var v string
	if json.Unmarshal(input[key], &v) != nil {
		return ""
	}
	v, _, _ = strings.Cut(strings.TrimSpace(v), "\n")
	return v
}
