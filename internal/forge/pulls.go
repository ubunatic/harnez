package forge

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ScanOptions bounds the open-pull-request listing: the per-request timeout, the page size
// and the page budget. They come from spec/pr.yaml open_pr_scan.
type ScanOptions struct {
	Timeout     time.Duration
	PageSize    int
	MaxPages    int
	SettingName string
}

// OpenPull is one open pull request and the file paths it changes, as the forge reports them.
type OpenPull struct {
	Number int
	Files  []string
}

// ErrTooManyPages means the open list did not fit the page budget, so the scan is incomplete.
// A full last page also counts, because the scan cannot tell whether another page follows.
var ErrTooManyPages = errors.New("more open pull requests than the scan page budget")

// PullLister lists open pull requests through a forge REST API. Token returns the bearer
// token for a forge, or "" when none is configured.
type PullLister struct {
	Client      *http.Client
	Token       func(Forge) string
	SettingName string
}

// NewPullLister builds a lister whose requests share the per-request timeout from opts.
func NewPullLister(opts ScanOptions, token func(Forge) string) PullLister {
	return PullLister{Client: &http.Client{Timeout: opts.Timeout}, Token: token, SettingName: opts.SettingName}
}

type pullItem struct {
	Number int    `json:"number"`
	State  string `json:"state"`
}

type fileItem struct {
	Filename string `json:"filename"`
}

// OpenPulls returns the open pull requests of owner/repo on fg, each with its changed files.
// It reads the open list once, then the file list of each open pull request. Merged and
// closed-unmerged pull requests are never returned, so their ticket numbers stay free.
func (l PullLister) OpenPulls(fg Forge, owner, repo string, opts ScanOptions) ([]OpenPull, error) {
	base := strings.TrimSuffix(fg.APIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/pulls"
	list, err := pagedGet[pullItem](l, fg, base, url.Values{"state": {"open"}}, opts)
	if err != nil {
		return nil, err
	}
	var out []OpenPull
	for _, p := range list {
		// The server filters by state already; the check guards against one that does not.
		if p.State != "open" || p.Number <= 0 {
			continue
		}
		files, err := pagedGet[fileItem](l, fg, fmt.Sprintf("%s/%d/files", base, p.Number), nil, opts)
		if err != nil {
			return nil, err
		}
		pull := OpenPull{Number: p.Number}
		for _, f := range files {
			pull.Files = append(pull.Files, f.Filename)
		}
		out = append(out, pull)
	}
	return out, nil
}

// pagedGet reads endpoint page by page until a short page, within the page budget.
// Forgejo pages with limit=N and GitHub with per_page=N; both take page=P.
func pagedGet[T any](l PullLister, fg Forge, endpoint string, query url.Values, opts ScanOptions) ([]T, error) {
	var all []T
	for page := 1; page <= opts.MaxPages; page++ {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set(pageParam(fg), strconv.Itoa(opts.PageSize))
		q.Set("page", strconv.Itoa(page))
		var items []T
		if err := l.getJSON(fg, endpoint+"?"+q.Encode(), &items); err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < opts.PageSize {
			return all, nil
		}
	}
	return nil, ErrTooManyPages
}

func pageParam(fg Forge) string {
	if fg.Kind == "github" {
		return "per_page"
	}
	return "limit"
}

// getJSON reads one list or item endpoint with the lister's token; GetJSON retries a 403 anonymously.
func (l PullLister) getJSON(fg Forge, rawURL string, v any) error {
	tok := ""
	if l.Token != nil {
		tok = l.Token(fg)
	}
	return GetJSON(l.Client, fg, rawURL, tok, "API request to "+fg.Host, l.SettingName, v)
}
