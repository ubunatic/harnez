package prwait

import (
	"fmt"
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

// Fetch performs one GET for the pull request and decodes it. forge.GetJSON sends the token and
// retries a 403 anonymously, as the open-PR scan does.
func (f *HTTPFetcher) Fetch(t Target) (PullInfo, error) {
	what := fmt.Sprintf("for %s/%s#%d on %s", t.Owner, t.Repo, t.Number, t.Forge.Host)
	var info PullInfo
	if err := forge.GetJSON(f.Client, t.Forge, PullURL(t), f.tokenFor(t.Forge), what, "", &info); err != nil {
		return PullInfo{}, err
	}
	return info, nil
}

// Comparer answers --contains through the forge API: is sha reachable from branch.
type Comparer interface {
	OnBranch(t Target, branch, sha string) (bool, error)
}

// CompareURL is the REST endpoint comparing branch (base) with sha (head). Forgejo and GitHub
// share the path and report the commits in sha that branch lacks as total_commits.
func CompareURL(t Target, branch, sha string) string {
	return fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s",
		strings.TrimSuffix(t.Forge.APIBase, "/"),
		url.PathEscape(t.Owner), url.PathEscape(t.Repo), url.PathEscape(branch), url.PathEscape(sha))
}

// OnBranch reports whether sha is on branch: the compare lists no commits that branch lacks.
// An unknown sha (for example one never pushed) is an error, so the caller falls back to git.
func (f *HTTPFetcher) OnBranch(t Target, branch, sha string) (bool, error) {
	what := fmt.Sprintf("comparing %s...%s for %s/%s on %s", branch, sha, t.Owner, t.Repo, t.Forge.Host)
	var cmp struct {
		TotalCommits *int `json:"total_commits"`
	}
	if err := forge.GetJSON(f.Client, t.Forge, CompareURL(t, branch, sha), f.tokenFor(t.Forge), what, "", &cmp); err != nil {
		return false, err
	}
	if cmp.TotalCommits == nil {
		return false, fmt.Errorf("compare response %s lacks total_commits", what)
	}
	return *cmp.TotalCommits == 0, nil
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
