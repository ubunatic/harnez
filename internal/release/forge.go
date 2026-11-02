package release

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ubunatic.com/harnez/internal/forge"
)

// ForgeInfo contains repository remote coordinates.
type ForgeInfo struct {
	RemoteName string
	Host       string
	Owner      string
	Repo       string
}

// IsSupportedForge checks if the given host is an authorized forge (codeberg.org or github.com).
func IsSupportedForge(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "codeberg.org" || h == "github.com"
}

type remoteEntry struct {
	name  string
	url   string
	host  string
	owner string
	repo  string
	valid bool
	// fanOut marks a remote with several push URLs (a local `both`, issue 1074).
	// It is not a copy of one forge, so it is chosen only as the last resort.
	fanOut bool
}

// DetectPrioritizedForgeInfo inspects all configured git remotes based on URL, prioritizing which
// remote to push and release against in the following order:
//  1. Priority 1: origin if its URL points to codeberg.org or github.com.
//  2. Priority 2: Any remote (e.g. codeberg, forgejo, upstream) whose URL points to codeberg.org.
//  3. Priority 3: Any remote (e.g. github, mirror) whose URL points to github.com.
//  4. Last resort: a fan-out remote (several push URLs) pointing at either forge.
//
// Fan-out remotes are skipped in priorities 2 and 3, so a `both` never stands in for the real remote.
// Priority 1 takes origin even when it fans out, because origin is the release remote by convention.
// If no remote matches any of these criteria, it returns an error.
func DetectPrioritizedForgeInfo(dir string) (*ForgeInfo, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)")
	}

	lines := strings.Split(string(out), "\n")
	seen := make(map[string]bool)
	var entries []remoteEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		rawURL := fields[1]
		key := name + "\x00" + rawURL
		if seen[key] {
			continue
		}
		seen[key] = true

		host, owner, repo, ok := forge.ParseRemoteURL(rawURL)
		entries = append(entries, remoteEntry{
			name:  name,
			url:   rawURL,
			host:  host,
			owner: owner,
			repo:  repo,
			valid: ok && IsSupportedForge(host),
		})
	}

	// Push URL counts per remote name: a name with several push URLs is a fan-out.
	fanOutByName := make(map[string]bool)
	for i := range entries {
		isFanOut, known := fanOutByName[entries[i].name]
		if !known {
			urls, _ := pushURLs(dir, entries[i].name)
			isFanOut = len(urls) > 1
			fanOutByName[entries[i].name] = isFanOut
		}
		entries[i].fanOut = isFanOut
	}

	// Priority 1: Remote named origin pointing to codeberg.org or github.com
	for _, e := range entries {
		if strings.EqualFold(e.name, "origin") && e.valid {
			return &ForgeInfo{
				RemoteName: e.name,
				Host:       e.host,
				Owner:      e.owner,
				Repo:       e.repo,
			}, nil
		}
	}

	// Priority 2: Any remote pointing to codeberg.org
	for _, e := range entries {
		if e.valid && !e.fanOut && strings.EqualFold(e.host, "codeberg.org") {
			return &ForgeInfo{
				RemoteName: e.name,
				Host:       e.host,
				Owner:      e.owner,
				Repo:       e.repo,
			}, nil
		}
	}

	// Priority 3: Any remote pointing to github.com
	for _, e := range entries {
		if e.valid && !e.fanOut && strings.EqualFold(e.host, "github.com") {
			return &ForgeInfo{
				RemoteName: e.name,
				Host:       e.host,
				Owner:      e.owner,
				Repo:       e.repo,
			}, nil
		}
	}

	// Last resort: a fan-out remote that points at a supported forge.
	for _, e := range entries {
		if e.valid {
			return &ForgeInfo{
				RemoteName: e.name,
				Host:       e.host,
				Owner:      e.owner,
				Repo:       e.repo,
			}, nil
		}
	}

	return nil, fmt.Errorf("no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)")
}

// DetectForgeInfo extracts host, owner, repo, and remote name by applying prioritized remote discovery.
func DetectForgeInfo(dir string) (*ForgeInfo, error) {
	return DetectPrioritizedForgeInfo(dir)
}

// PublishForgejoRelease creates or updates a release using fj.
func PublishForgejoRelease(dir string, tagName string, title string, attachments []string, remoteName string, dryRun, isContinue bool) error {
	if dryRun {
		return nil
	}
	return publishForgejoRelease(dir, tagName, title, attachments, os.Stdout, fjCommand(dir, remoteName))
}
