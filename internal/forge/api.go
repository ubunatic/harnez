package forge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/forgetoken"
)

// Forge describes one supported forge host from spec/pr.yaml.
type Forge struct {
	Host     string   `yaml:"host"`
	Kind     string   `yaml:"kind"`
	APIBase  string   `yaml:"api_base"`
	TokenEnv []string `yaml:"token_env"`
	// IssueCLI lists the CLIs `harnez issues new` tries, in order, to file a ticket on
	// this forge when the repo has no local issues/ dir (issue 1195).
	IssueCLI []string `yaml:"issue_cli"`
}

// APIError is a failed forge API call. Transient errors (network, 429, 5xx) may be retried.
// Detail never carries a token.
type APIError struct {
	Status    int // 0 when no HTTP response was received
	Transient bool
	Detail    string
}

func (e *APIError) Error() string {
	if e.Status == 0 {
		return "forge API request failed: " + e.Detail
	}
	return "forge API request failed: HTTP " + strconv.Itoa(e.Status) + " " + e.Detail
}

// TokenSource finds an optional token for a forge: for Forgejo the token stored by
// `harnez forge token create` (issue 1118), then the environment variables named in the spec,
// then, for GitHub only, the output of `gh auth token`. Nothing is ever printed.
type TokenSource struct {
	Getenv  func(string) string
	GHToken func() (string, error)
	Stored  func(host string) string // nil: no stored tokens
}

// SystemTokenSource reads the stored token, the real environment and the gh CLI when installed.
func SystemTokenSource(getenv func(string) string) TokenSource {
	return TokenSource{Getenv: getenv, GHToken: ghAuthToken, Stored: forgetoken.Lookup}
}

// For returns the token for forge, or "" when none is configured.
func (ts TokenSource) For(fg Forge) string {
	if fg.Kind == "forgejo" && ts.Stored != nil {
		if v := strings.TrimSpace(ts.Stored(fg.Host)); v != "" {
			return v
		}
	}
	for _, name := range fg.TokenEnv {
		if v := strings.TrimSpace(ts.Getenv(name)); v != "" {
			return v
		}
	}
	if fg.Kind == "github" && ts.GHToken != nil {
		if tok, err := ts.GHToken(); err == nil {
			return strings.TrimSpace(tok)
		}
	}
	return ""
}

func ghAuthToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		// Discard stderr on purpose: gh errors are not needed and must not be echoed.
		return "", err
	}
	return string(out), nil
}

// GetJSON does one GET of rawURL and decodes the JSON answer into v. It sends token as the
// bearer header when non-empty. what names the request in errors; it never carries a token.
//
// A 403 sent with a token is retried once without it: a token that lacks a scope still reads
// public repositories anonymously. If that retry fails too, the error is a 403 that says the
// token likely lacks a scope, and it replaces the retry's error: a private repo answers 404
// anonymously, and that 404 would send the user to the wrong hint.
func GetJSON(client *http.Client, fg Forge, rawURL, token, what, settingName string, v any) error {
	err := getOnce(client, fg, rawURL, token, what, settingName, v)
	var apiErr *APIError
	if token == "" || !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		return err
	}
	if getOnce(client, fg, rawURL, "", what, settingName, v) == nil {
		return nil
	}
	return &APIError{Status: http.StatusForbidden, Detail: what + "; the token likely lacks a scope: " + scopeAdvice(fg)}
}

// scopeAdvice says how to fix a token that a forge refused. Forgejo tokens are stored by harnez;
// GitHub tokens come from the environment or gh.
func scopeAdvice(fg Forge) string {
	if fg.Kind == "github" {
		return "set a token with read access in " + strings.Join(fg.TokenEnv, " or ") + " (or gh auth)"
	}
	return "run harnez forge token rotate or harnez forge token import"
}

func getOnce(client *http.Client, fg Forge, rawURL, token, what, settingName string, v any) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return &APIError{Detail: "building request: " + err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	if fg.Kind == "github" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	if token != "" {
		if fg.Kind == "github" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			req.Header.Set("Authorization", "token "+token)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		// The transport error may name the URL, never the header, so the token cannot leak here.
		if isTimeout(err) {
			return &APIError{Transient: true, Detail: "request to " + fg.Host + " " + timedOut(settingName)}
		}
		return &APIError{Transient: true, Detail: "request to " + fg.Host + " failed"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		transient := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return &APIError{Status: resp.StatusCode, Transient: transient, Detail: what}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(v); err != nil {
		if isTimeout(err) {
			return &APIError{Status: resp.StatusCode, Transient: true, Detail: "decoding response from " + fg.Host + ": " + timedOut(settingName)}
		}
		return &APIError{Status: resp.StatusCode, Detail: "decoding response from " + fg.Host}
	}
	return nil
}

// isTimeout reports whether err comes from a request deadline, either while waiting for the
// response or while reading its body (issue 1201).
func isTimeout(err error) bool {
	var te interface{ Timeout() bool }
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) ||
		(errors.As(err, &te) && te.Timeout())
}

// timedOut names the spec setting that bounds the request, so the user knows what to raise.
func timedOut(settingName string) string {
	if settingName == "" {
		return "timed out"
	}
	return "timed out (raise spec/pr.yaml " + settingName + ")"
}
