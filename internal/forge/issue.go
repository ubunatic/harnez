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

// Issues lists the issues of owner/repo on fg in the given state ("open", "closed" or "all";
// "" means all), newest first, without pull requests. It reads at most opts.MaxPages pages
// and reports truncated=true when more exist. It sends no server-side search: Forgejo's q is
// not a substring match (canary 2026-11-03: "asskey" found fewer issues than "passkey"), so
// callers filter.
func (l PullLister) Issues(fg Forge, owner, repo, state string, opts ScanOptions) (issues []RemoteIssue, truncated bool, err error) {
	base := strings.TrimSuffix(fg.APIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/issues"
	if state == "" {
		state = "all"
	}
	for page := 1; page <= opts.MaxPages; page++ {
		q := url.Values{"state": {state}}
		if fg.Kind != "github" {
			q.Set("type", "issues")
		}
		q.Set(pageParam(fg), strconv.Itoa(opts.PageSize))
		q.Set("page", strconv.Itoa(page))
		var items []RemoteIssue
		if err := l.getJSON(fg, base+"?"+q.Encode(), &items); err != nil {
			return nil, false, err
		}
		for _, is := range items {
			if is.PullRequest == nil {
				issues = append(issues, is)
			}
		}
		if len(items) < opts.PageSize {
			return issues, false, nil
		}
	}
	return issues, true, nil
}
