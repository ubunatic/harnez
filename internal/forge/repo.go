package forge

import (
	"net/url"
	"strings"
)

// Repo probes owner/repo on fg with the repo endpoint and returns nil when the repo is
// reachable with the token in use (issue 1199). A missing or private repo is an APIError.
func (l PullLister) Repo(fg Forge, owner, repo string) error {
	u := strings.TrimSuffix(fg.APIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	var r struct{}
	return l.getJSON(fg, u, &r)
}
