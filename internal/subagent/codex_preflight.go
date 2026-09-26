package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	codexPreflightTTL     = time.Minute
	codexPreflightTimeout = 5 * time.Second
)

// CodexPreflight verifies that the credentials used by Codex are accepted by
// its authentication service. Successful results are cached for one minute.
type CodexPreflight struct {
	mu      sync.Mutex
	checked time.Time
	probe   func(context.Context) error
	now     func() time.Time
}

// NewCodexPreflight creates a preflight checker. A nil probe uses the default
// HTTP check; an injected probe is useful for deterministic tests.
func NewCodexPreflight(probe func(context.Context) error) *CodexPreflight {
	if probe == nil {
		probe = probeCodexAuth
	}
	return &CodexPreflight{probe: probe, now: time.Now}
}

// Check returns a clear actionable error if Codex credentials or the auth
// service cannot be reached. Failed checks are not cached.
func (p *CodexPreflight) Check(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	if !p.checked.IsZero() && now.Sub(p.checked) < codexPreflightTTL {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, codexPreflightTimeout)
	err := p.probe(probeCtx)
	cancel()
	if err == nil {
		p.checked = now
		return nil
	}
	var authRejected *codexAuthRejectedError
	if errors.As(err, &authRejected) {
		return errors.New("Codex authentication rejected (401 Unauthorized). OpenAI auth service may be down or local credentials expired. Run `codex login` to re-authenticate. If OpenAI is offline, halt the current goal loop or switch to an alternate provider (claude/agy, noting cost differences).")
	}
	return fmt.Errorf("Codex authentication preflight failed (auth service unavailable): %w. Check connectivity, halt the current goal loop, or switch to an alternate provider (claude/agy, noting cost differences)", err)
}

type codexAuthRejectedError struct{ status string }

func (e *codexAuthRejectedError) Error() string { return e.status }

func probeCodexAuth(ctx context.Context) error {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return err
		}
		home = filepath.Join(home, ".codex")
	}
	data, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		return fmt.Errorf("read Codex credentials: %w", err)
	}
	var auth struct {
		AuthMode string            `json:"auth_mode"`
		APIKey   string            `json:"OPENAI_API_KEY"`
		Tokens   map[string]string `json:"tokens"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return fmt.Errorf("decode Codex credentials: %w", err)
	}
	token, endpoint := auth.Tokens["access_token"], "https://chatgpt.com/backend-api/wham/usage"
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		token = key
		endpoint = "https://api.openai.com/v1/models"
	} else if auth.AuthMode == "api_key" {
		token = auth.APIKey
		endpoint = "https://api.openai.com/v1/models"
	}
	if token == "" {
		return errors.New("Codex credentials contain no access token")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "codex")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: codexPreflightTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusUnauthorized {
		return &codexAuthRejectedError{status: resp.Status}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Codex auth endpoint returned %s", resp.Status)
	}
	return nil
}

var defaultCodexPreflight = NewCodexPreflight(nil)

// CheckCodexAuth runs the process-wide cached Codex authentication check.
func CheckCodexAuth(ctx context.Context) error { return defaultCodexPreflight.Check(ctx) }
