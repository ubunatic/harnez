package subagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/claude"
)

// AgyDriver runs agy (Antigravity CLI) in print mode, pinned to Dir.
type AgyDriver struct {
	Command   func(context.Context, string, ...string) ([]byte, error)
	Dir       string
	SessionID string
}

func (d AgyDriver) command(ctx context.Context, args ...string) ([]byte, error) {
	if d.Command != nil {
		return d.Command(ctx, "agy", args...)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("agy: resolve home directory: %w", err)
	}
	if _, _, err := claude.EnsureBashShim(home); err != nil {
		return nil, fmt.Errorf("agy: ensure bash shim: %w", err)
	}
	env := AgyLaunchEnv(os.Environ(), home)
	if d.SessionID != "" {
		env = replaceEnvironmentValue(env, "HARNEZ_SESSION_ID", d.SessionID)
		env = replaceEnvironmentValue(env, "HARNEZ_AGY_METER_SESSION_ID", d.SessionID)
	}
	var stdout, stderr bytes.Buffer
	err = agymeter.RunWithEnvDir(ctx, home, "agy", args, env, d.Dir, nil, &stdout, &stderr)
	if err != nil && stderr.Len() > 0 {
		return stdout.Bytes(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), err
}

// AgyLaunchEnv prepares an environment for launching the real agy executable.
// Keep this shared with the installed harnez-agy POSIX launcher.
func AgyLaunchEnv(environ []string, home string) []string {
	shimDir := filepath.Join(home, ".harnez", "shims")
	path := environmentValue(environ, "PATH")
	pathEntries := filepath.SplitList(path)
	if len(pathEntries) == 0 || filepath.Clean(pathEntries[0]) != filepath.Clean(shimDir) {
		if path == "" {
			path = shimDir
		} else {
			path = shimDir + string(os.PathListSeparator) + path
		}
	}
	environ = replaceEnvironmentValue(environ, "PATH", path)
	return replaceEnvironmentValue(environ, "ANTIGRAVITY_AGENT", "1")
}

func environmentValue(environ []string, name string) string {
	prefix := name + "="
	for i := len(environ) - 1; i >= 0; i-- {
		if strings.HasPrefix(environ[i], prefix) {
			return strings.TrimPrefix(environ[i], prefix)
		}
	}
	return ""
}

func replaceEnvironmentValue(environ []string, name, value string) []string {
	prefix := name + "="
	updated := make([]string, 0, len(environ)+1)
	replaced := false
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				updated = append(updated, prefix+value)
				replaced = true
			}
			continue
		}
		updated = append(updated, entry)
	}
	if !replaced {
		updated = append(updated, prefix+value)
	}
	return updated
}

func agyEffort(tier string) string {
	if tier == "med" {
		return "medium"
	}
	return tier
}

func (d AgyDriver) Run(ctx context.Context, o RunOptions) (*TurnResult, error) {
	if d.Dir == "" {
		return nil, errors.New("agy: refusing to run with an empty Dir (agy would write to ~/.gemini scratch)")
	}
	start := time.Now()
	args := []string{"--add-dir", d.Dir}
	if o.Model.Name != "" {
		args = append(args, "--model", o.Model.Name)
		if o.Model.SupportsEffort() && o.Model.Tier != "" {
			args = append(args, "--effort", agyEffort(o.Model.Tier))
		}
	}
	args = append(args, "--output-format", "json", "-p", o.Prompt)
	b, err := d.command(ctx, args...)
	if err != nil {
		return nil, agyRunError("agy", b, err)
	}
	r, err := parseAgy(b)
	if err != nil {
		return nil, err
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}

func (d AgyDriver) Resume(ctx context.Context, id, prompt string, model Model) (*TurnResult, error) {
	if d.Dir == "" {
		return nil, errors.New("agy: refusing to resume with an empty Dir (agy would write to ~/.gemini scratch)")
	}
	args := []string{"--conversation", id, "--add-dir", d.Dir}
	if model.Name != "" {
		args = append(args, "--model", model.Name)
		if model.SupportsEffort() && model.Tier != "" {
			args = append(args, "--effort", agyEffort(model.Tier))
		}
	}
	args = append(args, "--output-format", "json", "-p", prompt)
	b, err := d.command(ctx, args...)
	if err != nil {
		return nil, agyRunError("agy resume", b, err)
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

type agyResult struct {
	ConversationID string  `json:"conversation_id"`
	Status         string  `json:"status"`
	Response       string  `json:"response"`
	Error          string  `json:"error"`
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

// agyRunError builds the error for a non-zero agy exit: agy prints
// {"status":"ERROR","error":"…"} on stdout even on exit 1, so parse it before
// falling back to stderr from the exec error.
func agyRunError(op string, b []byte, err error) error {
	var v agyResult
	if len(b) > 0 && json.Unmarshal(b, &v) == nil && v.Error != "" {
		return fmt.Errorf("%s: %s", op, v.Error)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("%s: %w: %s", op, err, ee.Stderr)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func parseAgy(data []byte) (*TurnResult, error) {
	var v agyResult
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse agy json: %w", err)
	}
	if v.Status != "SUCCESS" {
		msg := v.Response
		if v.Error != "" {
			msg = v.Error
		}
		return nil, fmt.Errorf("agy reported status %q: %s", v.Status, msg)
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
