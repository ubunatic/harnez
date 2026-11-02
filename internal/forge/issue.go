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
