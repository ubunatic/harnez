package prwait

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/forge"
)

// Target identifies one pull request on one configured forge.
type Target struct {
	Forge  Forge
	Owner  string
	Repo   string
	Number int
}

// ParsePRURL splits a pull request page URL such as
// https://codeberg.org/owner/repo/pulls/78 or https://github.com/owner/repo/pull/78.
func ParsePRURL(raw string) (host, owner, repo string, number int, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", "", "", 0, fmt.Errorf("not a pull request URL: %q (expected https://<host>/<owner>/<repo>/pulls/<n>)", raw)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || (parts[2] != "pulls" && parts[2] != "pull") {
		return "", "", "", 0, fmt.Errorf("not a pull request URL: %q (expected https://<host>/<owner>/<repo>/pulls/<n>)", raw)
	}
	n, err := parseNumber(parts[3])
	if err != nil {
		return "", "", "", 0, err
	}
	return u.Host, parts[0], parts[1], n, nil
}

func parseNumber(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid pull request number %q", s)
	}
	return n, nil
}

// ResolveTarget turns the command argument into a Target. A URL names its own forge and repository.
// A bare number takes the forge and repository from remoteURL, the URL of the chosen git remote.
func ResolveTarget(spec Spec, arg, remoteURL string) (Target, error) {
	arg = strings.TrimSpace(arg)
	var host, owner, repo string
	var number int
	var err error
	if strings.Contains(arg, "://") {
		host, owner, repo, number, err = ParsePRURL(arg)
		if err != nil {
			return Target{}, err
		}
	} else {
		number, err = parseNumber(arg)
		if err != nil {
			return Target{}, err
		}
		var ok bool
		host, owner, repo, ok = forge.ParseRemoteURL(remoteURL)
		if !ok {
			return Target{}, fmt.Errorf("remote URL %q is not an ssh or https forge URL", remoteURL)
		}
	}
	f, ok := spec.ForgeFor(host)
	if !ok {
		return Target{}, fmt.Errorf("forge host %q is not configured in spec/pr.yaml", host)
	}
	return Target{Forge: f, Owner: owner, Repo: repo, Number: number}, nil
}

// PageURL is the human-facing pull request page. GitHub says "pull", Forgejo says "pulls".
func (t Target) PageURL() string {
	segment := "pulls"
	if t.Forge.Kind == "github" {
		segment = "pull"
	}
	return fmt.Sprintf("https://%s/%s/%s/%s/%d", t.Forge.Host, t.Owner, t.Repo, segment, t.Number)
}
