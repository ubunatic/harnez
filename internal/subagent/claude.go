package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// claudeCompactWindowEnv is the Claude Code variable that sets its
// auto-compaction window. harnez sets it on every Claude child to the
// threshold it resolves for the child's model.
const claudeCompactWindowEnv = "CLAUDE_CODE_AUTO_COMPACT_WINDOW"

// claudeChildEnv returns environ with CLAUDE_CODE_AUTO_COMPACT_WINDOW set to
// the harnez compaction threshold for model. A value the caller already set is
// kept, and a warning goes to warn when it is above the threshold. When no
// threshold resolves (for example a config that sets 0), the child gets
// environ unchanged, as the start path disables its watchdog in that case.
func claudeChildEnv(model Model, environ []string, warn io.Writer) []string {
	threshold, err := CompactThreshold(model)
	if err != nil {
		// A zero threshold is an intentional opt-out: stay silent.
		if !errors.Is(err, errCompactDisabled) && warn != nil {
			fmt.Fprintf(warn, "harnez: no %s for %s: %v\n", claudeCompactWindowEnv, model.Spec(), err)
		}
		return environ
	}
	prefix := claudeCompactWindowEnv + "="
	// exec keeps the last duplicate, so the last value is the one Claude sees.
	caller := ""
	found := false
	for _, kv := range environ {
		if strings.HasPrefix(kv, prefix) {
			caller, found = strings.TrimPrefix(kv, prefix), true
		}
	}
	if found {
		n, convErr := strconv.Atoi(caller)
		switch {
		case convErr != nil:
			if warn != nil {
				fmt.Fprintf(warn, "harnez: %s=%q is not a number; Claude gets it unchanged\n", claudeCompactWindowEnv, caller)
			}
		case n > threshold && warn != nil:
			fmt.Fprintf(warn, "harnez: %s=%d is above the compaction threshold %d for %s; Claude compacts at the caller value\n", claudeCompactWindowEnv, n, threshold, model.Spec())
		}
		return environ
	}
	return append(append([]string(nil), environ...), prefix+strconv.Itoa(threshold))
}

// claudeEnv is the environment for one Claude child of this process.
func claudeEnv(model Model) []string {
	return claudeChildEnv(model, os.Environ(), os.Stderr)
}

// ClaudeDriver runs Claude Code in print mode.
type ClaudeDriver struct {
	Command func(context.Context, string, ...string) ([]byte, error)
	Dir     string
}

func (d ClaudeDriver) command(ctx context.Context, env []string, args ...string) ([]byte, error) {
	return d.commandLines(ctx, env, nil, args...)
}

func (d ClaudeDriver) commandLines(ctx context.Context, env []string, fn func([]byte), args ...string) ([]byte, error) {
	return runProviderCLI(ctx, "claude", d.Dir, env, d.Command, fn, args)
}

func (d ClaudeDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	if err := ValidateContextMode("claude", o.ContextMode); err != nil {
		return nil, err
	}
	env := claudeEnv(o.Model)
	start := time.Now()
	b, err := d.command(ctx, env, claudeRunArgs(o.Model, o.Prompt, o.ContextMode)...)
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
	env := claudeEnv(model)
	b, err := d.command(ctx, env, claudeResumeArgs(id, prompt, model)...)
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
func claudeRunArgs(model Model, prompt string, contextMode ...string) []string {
	args := append([]string{"-p", "--dangerously-skip-permissions"}, claudeModelArgs(model)...)
	var mode string
	if len(contextMode) > 0 {
		mode = contextMode[0]
	}
	args = append(args, claudeContextModeArgs(mode)...)
	return append(args, "--output-format", "stream-json", "--verbose", prompt)
}

func claudeContextModeArgs(mode string) []string {
	if mode == "" || mode == "default" {
		return nil
	}
	spec, err := ResolveContextMode(mode)
	if err != nil {
		return nil
	}
	var args []string
	if spec.Skills != nil && !*spec.Skills {
		args = append(args, "--disable-slash-commands")
	}
	if spec.MCP != nil && !*spec.MCP {
		args = append(args, "--strict-mcp-config")
	}
	if spec.ContextFiles != nil && !*spec.ContextFiles {
		args = append(args, "--setting-sources", "")
	}
	if len(spec.Tools) > 0 {
		mapped := make([]string, 0, len(spec.Tools))
		for _, tool := range spec.Tools {
			mapped = append(mapped, claudeToolName(tool))
		}
		args = append(args, "--tools", strings.Join(mapped, ","))
	}
	return args
}

func claudeToolName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "read":
		return "Read"
	case "edit":
		return "Edit"
	case "write":
		return "Write"
	case "grep":
		return "Grep"
	case "find":
		return "Glob"
	case "bash":
		return "Bash"
	default:
		return name
	}
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

func (d ClaudeDriver) Compact(ctx context.Context, id string, model Model) (*TurnResult, error) {
	return d.Resume(ctx, id, "/compact", model)
}
func (d ClaudeDriver) Stop(ctx context.Context, id string) error {
	return nil
}
func (d ClaudeDriver) Delete(ctx context.Context, id string) error {
	return nil
}

// claudeUsage is the token usage block of a Claude message or result.
type claudeUsage struct {
	Input         int `json:"input_tokens"`
	Output        int `json:"output_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
	CacheCreate   int `json:"cache_creation_input_tokens"`
	CacheCreation *struct {
		Ephemeral5m int `json:"ephemeral_5m_input_tokens"`
		Ephemeral1h int `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

// requestUsage converts one API call's usage. The TTL split is known only when
// the provider reports it and it adds up to the total cache writes.
func (u claudeUsage) requestUsage(model string) RequestUsage {
	req := RequestUsage{Model: model, Input: u.Input, CacheRead: u.CacheRead, Output: u.Output, CacheWriteTotal: u.CacheCreate, CacheWriteSplitKnown: u.CacheCreate == 0}
	if u.CacheCreation != nil && u.CacheCreation.Ephemeral5m+u.CacheCreation.Ephemeral1h == u.CacheCreate {
		req.CacheWrite5m = u.CacheCreation.Ephemeral5m
		req.CacheWrite1h = u.CacheCreation.Ephemeral1h
		req.CacheWriteSplitKnown = true
	}
	return req
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
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Usage   *claudeUsage    `json:"usage"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Compact struct {
		PreTokens  int `json:"pre_tokens"`
		PostTokens int `json:"post_tokens"`
	} `json:"compact_metadata"`
}

// claudeScan is what one pass over Claude's stream-json output yields. The full
// parse needs the result line; the partial parse of a stopped turn does not.
type claudeScan struct {
	res         *claudeEvent
	last        *claudeUsage
	postCompact int
	preCompact  int
	localOut    []string
	// calls holds one entry per API call, sub-agent calls included, as the
	// result usage totals them.
	calls     []RequestUsage
	mainModel string
	// peak is the largest main-chain prompt seen in the scan, in tokens.
	peak int
}

// scanClaude walks the lines. Strict mode fails on a truncated JSON object;
// lenient mode skips it, because a stopped turn can end in the middle of a line.
func scanClaude(data []byte, lenient bool) (claudeScan, error) {
	sc := claudeScan{postCompact: -1}
	// One API call streams as several assistant events that repeat its usage;
	// the last event of each message id is its final usage (ticket 1301).
	callIndex := map[string]int{}
	for lineNo, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e claudeEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			if strings.HasPrefix(line, "{") && !strings.HasSuffix(line, "}") && !lenient {
				return claudeScan{}, fmt.Errorf("parse claude json: %w", err)
			}
			continue
		}
		switch {
		case e.Type == "assistant" && e.Message.Usage != nil:
			u := e.Message.Usage
			if u.Input+u.CacheRead+u.CacheCreate+u.Output > 0 {
				key := e.Message.ID
				if key == "" {
					key = fmt.Sprintf("line:%d", lineNo)
				}
				req := u.requestUsage(e.Message.Model)
				if idx, seen := callIndex[key]; seen {
					sc.calls[idx] = req
				} else {
					callIndex[key] = len(sc.calls)
					sc.calls = append(sc.calls, req)
				}
			}
			// Synthetic events after /compact carry all-zero usage; skip them.
			if e.Parent == nil && u.Input+u.CacheRead+u.CacheCreate > 0 {
				sc.last = u
				sc.peak = max(sc.peak, u.Input+u.CacheRead+u.CacheCreate)
				if e.Message.Model != "" {
					sc.mainModel = e.Message.Model
				}
			}
		case e.Type == "system" && e.Subtype == "compact_boundary" && e.Compact.PostTokens > 0:
			sc.postCompact = e.Compact.PostTokens
			sc.preCompact = e.Compact.PreTokens
			sc.last = nil
		case e.Type == "user":
			// A local command such as /compact reports only here, not in result.
			var text string
			if json.Unmarshal(e.Message.Content, &text) == nil {
				if _, rest, ok := strings.Cut(text, "<local-command-stdout>"); ok {
					out, _, _ := strings.Cut(rest, "</local-command-stdout>")
					sc.localOut = append(sc.localOut, strings.TrimSpace(out))
				}
			}
		case e.Type == "result" || e.Type == "":
			sc.res = &e
		}
	}
	return sc, nil
}

// fill sets the call, model, compaction and context fields of r from the scan.
// The context is the last main-thread assistant call, or a compact boundary's
// post_tokens; when neither is present it is unknown (-1), as for agy (issues 644, 673).
func (sc claudeScan) fill(r *TurnResult) {
	r.RequestUsage = sc.calls
	r.Model = sc.mainModel
	r.CompactionObserved = sc.postCompact > 0
	r.ContextTokensBeforeCompact = sc.preCompact
	r.ContextTokens = -1
	if sc.last != nil {
		r.ContextTokens = sc.last.Input + sc.last.CacheRead + sc.last.CacheCreate
	} else if sc.postCompact > 0 {
		r.ContextTokens = sc.postCompact
	}
	r.PeakContextTokens = sc.peak
	r.TokensTurn = r.InputTokens + r.OutputTokens + r.CachedTokens + r.ReasoningTokens
	r.TokensCumulative = r.TokensTurn
}

// parseClaude reads Claude's stream-json output (one event per line) or a
// single --output-format json object. The result's usage totals every model
// call of the turn, so the context usage comes from the last main-thread
// assistant call (see fill).
func parseClaude(data []byte) (*TurnResult, error) {
	sc, err := scanClaude(data, false)
	if err != nil {
		return nil, err
	}
	res := sc.res
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
	r := &TurnResult{SessionID: res.SessionID, Response: res.Result, Messages: []string{res.Result}, InputTokens: v.Input, OutputTokens: v.Output, CachedTokens: v.CacheRead + v.CacheCreate, CacheWriteTokens: v.CacheCreate, CacheWriteTokensKnown: true}
	sc.fill(r)
	r.Messages = append(r.Messages, sc.localOut...)
	return r, nil
}

// parseClaudePartial builds the turn of a run that was stopped before its
// result line (ticket 1264 M2). Its totals are the sums over the calls that
// finished, and a truncated last line is skipped. It always returns a result;
// the caller writes a turn record only when RequestUsage is not empty.
func parseClaudePartial(data []byte) *TurnResult {
	sc, _ := scanClaude(data, true) // lenient mode does not fail
	r := &TurnResult{CacheWriteTokensKnown: true}
	for _, c := range sc.calls {
		r.InputTokens += c.Input
		r.OutputTokens += c.Output
		r.CachedTokens += c.CacheRead + c.CacheWriteTotal
		r.CacheWriteTokens += c.CacheWriteTotal
	}
	sc.fill(r)
	return r
}

// PartialTurnError is a turn whose command failed before its result line, for
// example when the token watchdog killed it. Partial holds the usage of the
// calls that finished, so the caller can meter the turn. Error and Unwrap
// are those of Err.
type PartialTurnError struct {
	Err     error
	Partial *TurnResult
}

func (e *PartialTurnError) Error() string { return e.Err.Error() }
func (e *PartialTurnError) Unwrap() error { return e.Err }

// claudeStopError is the error of a run whose command failed. A reported
// error (is_error, a failed /compact) is returned as is. Any other failure
// carries the partial turn from the output read so far.
func claudeStopError(b []byte, err error) error {
	if _, parseErr := parseClaude(b); parseErr != nil && (strings.Contains(parseErr.Error(), "claude reported an error:") || strings.HasPrefix(parseErr.Error(), "claude: ")) {
		return parseErr
	}
	return &PartialTurnError{Err: err, Partial: parseClaudePartial(b)}
}

var _ Driver = ClaudeDriver{}

// ClaudeStreamDriver is ClaudeDriver plus live events for harnez tui. It is a
// separate type so batch callers (harnez agent start/resume) that type-assert
// StreamingDriver keep ClaudeDriver's batch behaviour.
type ClaudeStreamDriver struct{ ClaudeDriver }

// RunStream runs a new claude turn and reports events while it runs. The
// arguments and the final result are those of Run.
func (d ClaudeStreamDriver) RunStream(ctx context.Context, o RunOptions, fn EventFunc) (*TurnResult, error) {
	if err := ValidateContextMode("claude", o.ContextMode); err != nil {
		return nil, err
	}
	env := claudeEnv(o.Model)
	start := time.Now()
	b, err := d.commandLines(ctx, env, claudeLineFunc(fn), claudeRunArgs(o.Model, o.Prompt, o.ContextMode)...)
	if err != nil {
		return nil, claudeStopError(b, fmt.Errorf("claude: %w", err))
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
	env := claudeEnv(model)
	b, err := d.commandLines(ctx, env, claudeLineFunc(fn), claudeResumeArgs(id, prompt, model)...)
	if err != nil {
		return nil, claudeStopError(b, fmt.Errorf("claude resume: %w", err))
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
// command output, and context usage. Malformed lines are skipped; the final
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
			ID    string                     `json:"id"`
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
				evs = append(evs, Event{Kind: "activity", Text: claudeToolActivity(b.Name, b.Input), ToolCall: true, CallID: b.ID})
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
