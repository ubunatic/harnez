package subagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CodexDriver runs codex in non-interactive JSONL mode.
type CodexDriver struct {
	Command func(context.Context, string, ...string) ([]byte, error)
	// Start launches a process for streaming turns; nil uses os/exec.
	Start func(context.Context, string, ...string) (io.Reader, func() error, error)
}

func (d CodexDriver) CheckResumable(providerID string) (bool, string) {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
		home = filepath.Join(home, ".codex")
	}
	sessions := filepath.Join(home, "sessions")
	if _, err := os.Stat(sessions); os.IsNotExist(err) {
		return true, "Codex session state is unknown"
	}
	matches, err := filepath.Glob(filepath.Join(sessions, "*", "*", "*", "rollout-*"+providerID+".jsonl"))
	if err == nil && len(matches) > 0 {
		return true, ""
	}
	return false, "Codex session file is missing"
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
	args := codexRunArgs(o.Model, o.Prompt)
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

func codexEffort(tier string) string {
	if tier == "med" {
		return "medium"
	}
	return tier
}

func codexRunArgs(model Model, prompt string) []string {
	return []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox", "-m", model.Name, "-c", "model_reasoning_effort=" + codexEffort(model.Tier), prompt}
}

// codexResumeArgs mirrors the sandbox settings of Run: a resumed worker must
// not fall back to Codex's default sandbox (read-only tool caches, no sockets,
// no git writes) and must work outside a trusted git directory.
func codexResumeArgs(id, prompt string, model Model) []string {
	args := []string{"exec", "resume", id, "--json", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check"}
	if codexEffort(model.Tier) != "" {
		args = append(args, "-c", "model_reasoning_effort="+codexEffort(model.Tier))
	}
	return append(args, prompt)
}

func (d CodexDriver) Resume(ctx context.Context, id, prompt string, model Model) (*TurnResult, error) {
	return d.runResume(ctx, id, prompt, model)
}
func (d CodexDriver) runResume(ctx context.Context, id, prompt string, model Model) (*TurnResult, error) {
	b, err := d.command(ctx, codexResumeArgs(id, prompt, model)...)
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

// Event is one live occurrence in a streaming turn.
type Event struct {
	Kind  string // "session", "message", "activity" or "other"
	Text  string // thread id, message text or activity description
	Bytes int    // raw size of the provider event, for token estimates
}

// EventFunc receives events while a turn is running.
type EventFunc func(Event)

// StreamingDriver is implemented by drivers that report events as they happen.
type StreamingDriver interface {
	Driver
	RunStream(context.Context, RunOptions, EventFunc) (*TurnResult, error)
	ResumeStream(ctx context.Context, id, prompt string, model Model, fn EventFunc) (*TurnResult, error)
}

// codexParser folds Codex JSONL lines into a TurnResult and live events.
type codexParser struct{ r TurnResult }

func (p *codexParser) feed(line []byte) (Event, bool) {
	var e struct {
		Type     string                               `json:"type"`
		ThreadID string                               `json:"thread_id"`
		Item     struct{ Type, Text, Command string } `json:"item"`
		Usage    struct {
			Input   int `json:"input_tokens"`
			Output  int `json:"output_tokens"`
			Cached  int `json:"cached_input_tokens"`
			Details struct {
				Cached int `json:"cached_input_tokens"`
			} `json:"input_token_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(line, &e) != nil {
		return Event{}, false
	}
	ev := Event{Kind: "other", Bytes: len(line)}
	if e.ThreadID != "" {
		p.r.SessionID = e.ThreadID
		ev.Kind, ev.Text = "session", e.ThreadID
	}
	switch {
	case e.Item.Type == "agent_message":
		p.r.Response = e.Item.Text
		p.r.Messages = append(p.r.Messages, e.Item.Text)
		ev.Kind, ev.Text = "message", e.Item.Text
	case e.Type == "item.started" && e.Item.Type == "command_execution":
		ev.Kind, ev.Text = "activity", "running "+e.Item.Command
	}
	if e.Type == "turn.completed" {
		p.r.InputTokens += e.Usage.Input
		p.r.OutputTokens += e.Usage.Output
		p.r.CachedTokens += e.Usage.Cached
		if e.Usage.Details.Cached > p.r.CachedTokens {
			p.r.CachedTokens = e.Usage.Details.Cached
		}
	}
	return ev, true
}

func (p *codexParser) result() (*TurnResult, error) {
	if p.r.Response == "" {
		return nil, fmt.Errorf("codex output has no agent_message")
	}
	p.r.TokensTurn = p.r.InputTokens + p.r.OutputTokens
	p.r.TokensCumulative = p.r.TokensTurn
	return &p.r, nil
}

func parseCodexStream(rd io.Reader, fn EventFunc) (*TurnResult, error) {
	p := &codexParser{}
	s := bufio.NewScanner(rd)
	s.Buffer(make([]byte, 4096), 8<<20)
	for s.Scan() {
		if ev, ok := p.feed(s.Bytes()); ok && fn != nil {
			fn(ev)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return p.result()
}

func parseCodex(data []byte) (*TurnResult, error) {
	return parseCodexStream(bytes.NewReader(data), nil)
}

// stream runs codex and parses its JSONL output while the process is running.
func (d CodexDriver) stream(ctx context.Context, fn EventFunc, args ...string) (*TurnResult, error) {
	start := d.Start
	if start == nil {
		start = startProcess
	}
	rd, wait, err := start(ctx, "codex", args...)
	if err != nil {
		return nil, err
	}
	r, perr := parseCodexStream(rd, fn)
	if werr := wait(); werr != nil {
		return nil, werr
	}
	return r, perr
}

func startProcess(ctx context.Context, name string, args ...string) (io.Reader, func() error, error) {
	c := exec.CommandContext(ctx, name, args...)
	out, err := c.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	var stderr bytes.Buffer
	c.Stderr = &stderr
	if err := c.Start(); err != nil {
		return nil, nil, err
	}
	return out, func() error {
		err := c.Wait()
		if msg := strings.TrimSpace(stderr.String()); err != nil && msg != "" {
			return fmt.Errorf("%w: %s", err, lastLines(msg, 3))
		}
		return err
	}, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "; ")
}

func (d CodexDriver) RunStream(ctx context.Context, o RunOptions, fn EventFunc) (*TurnResult, error) {
	start := time.Now()
	r, err := d.stream(ctx, fn, codexRunArgs(o.Model, o.Prompt)...)
	if err != nil {
		return nil, fmt.Errorf("codex exec: %w", err)
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}

func (d CodexDriver) ResumeStream(ctx context.Context, id, prompt string, model Model, fn EventFunc) (*TurnResult, error) {
	r, err := d.stream(ctx, fn, codexResumeArgs(id, prompt, model)...)
	if err != nil {
		return nil, fmt.Errorf("codex resume: %w", err)
	}
	return r, nil
}

var _ StreamingDriver = CodexDriver{}
