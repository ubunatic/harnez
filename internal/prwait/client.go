package prwait

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/forge"
)

// PullInfo is the subset of a pull request that pr wait reads. Forgejo and GitHub both
// return these fields. A merged pull request reports state "closed", so merged is the signal.
type PullInfo struct {
	Number         int    `json:"number"`
	State          string `json:"state"`
	Merged         bool   `json:"merged"`
	MergedAt       string `json:"merged_at"`
	MergeCommitSHA string `json:"merge_commit_sha"`
	HTMLURL        string `json:"html_url"`
}

// APIError is a failed pull request lookup. Transient errors (network, 429, 5xx) may be retried.
// The type lives in internal/forge, shared with the open-PR ticket scan.
type APIError = forge.APIError

// Fetcher reads one pull request. Implementations must never include a token in an error.
type Fetcher interface {
	Fetch(t Target) (PullInfo, error)
}

// HTTPFetcher calls the forge REST API. Token returns the bearer token for a forge, or "".
type HTTPFetcher struct {
	Client *http.Client
	Token  func(Forge) string
}

// NewHTTPFetcher builds a fetcher with the per-request timeout from spec/pr.yaml.
func NewHTTPFetcher(timeout time.Duration, token func(Forge) string) *HTTPFetcher {
	return &HTTPFetcher{Client: &http.Client{Timeout: timeout}, Token: token}
}

// PullURL is the REST endpoint for one pull request.
func PullURL(t Target) string {
	return fmt.Sprintf("%s/repos/%s/%s/pulls/%d",
		strings.TrimSuffix(t.Forge.APIBase, "/"),
		url.PathEscape(t.Owner), url.PathEscape(t.Repo), t.Number)
}

// Fetch performs one GET for the pull request and decodes it.
func (f *HTTPFetcher) Fetch(t Target) (PullInfo, error) {
	req, err := http.NewRequest(http.MethodGet, PullURL(t), nil)
	if err != nil {
		return PullInfo{}, &APIError{Detail: "building request: " + err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	if t.Forge.Kind == "github" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	if tok := f.tokenFor(t.Forge); tok != "" {
		if t.Forge.Kind == "github" {
			req.Header.Set("Authorization", "Bearer "+tok)
		} else {
			req.Header.Set("Authorization", "token "+tok)
		}
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		// The transport error names the URL, never the header, so the token cannot leak here.
		return PullInfo{}, &APIError{Transient: true, Detail: "request to " + t.Forge.Host + " failed"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		transient := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return PullInfo{}, &APIError{Status: resp.StatusCode, Transient: transient, Detail: fmt.Sprintf("for %s/%s#%d on %s", t.Owner, t.Repo, t.Number, t.Forge.Host)}
	}
	var info PullInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return PullInfo{}, &APIError{Status: resp.StatusCode, Detail: "decoding response from " + t.Forge.Host}
	}
	return info, nil
}

func (f *HTTPFetcher) tokenFor(fg Forge) string {
	if f.Token == nil {
		return ""
	}
	return f.Token(fg)
}

// TokenSource finds the optional forge token. It lives in internal/forge, shared with the
// open-PR ticket scan; SystemTokenSource stays here for the pr wait callers.
type TokenSource = forge.TokenSource

// SystemTokenSource reads the real environment and the gh CLI when it is installed.
func SystemTokenSource(getenv func(string) string) TokenSource {
	return forge.SystemTokenSource(getenv)
}
