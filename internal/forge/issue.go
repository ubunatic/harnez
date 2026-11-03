package forge

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RemoteIssue is one forge issue as `harnez issues import` needs it. Forgejo and GitHub
// share these field names.
type RemoteIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	PullRequest *struct{} `json:"pull_request"`
}

// Issue reads issue number n of owner/repo on fg.
func (l PullLister) Issue(fg Forge, owner, repo string, n int) (RemoteIssue, error) {
	var is RemoteIssue
	u := strings.TrimSuffix(fg.APIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/issues/" + strconv.Itoa(n)
	err := l.getJSON(fg, u, &is)
	return is, err
}

// Issues lists the issues of owner/repo on fg in every state, newest first, without
// pull requests. A non-empty query narrows the list on the server where the forge
// supports it (Forgejo's q); callers still filter, because GitHub's list has no query.
func (l PullLister) Issues(fg Forge, owner, repo, query string, opts ScanOptions) ([]RemoteIssue, error) {
	base := strings.TrimSuffix(fg.APIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/issues"
	q := url.Values{"state": {"all"}}
	if fg.Kind != "github" {
		q.Set("type", "issues")
		if query != "" {
			q.Set("q", query)
		}
	}
	list, err := pagedGet[RemoteIssue](l, fg, base, q, opts)
	if err != nil {
		return nil, err
	}
	out := list[:0]
	for _, is := range list {
		if is.PullRequest == nil {
			out = append(out, is)
		}
	}
	return out, nil
}
