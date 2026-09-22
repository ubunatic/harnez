package release

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// ForgeInfo contains repository remote coordinates.
type ForgeInfo struct {
	RemoteName string
	Host       string
	Owner      string
	Repo       string
}

var (
	sshRemoteRegex   = regexp.MustCompile(`(?i)^(?:ssh://)?(?:[^@]+@)?([^:/]+)[:/]([^/]+)/([^/]+?)(?:\.git)?$`)
	httpsRemoteRegex = regexp.MustCompile(`(?i)^https?://([^/]+)/([^/]+)/([^/]+?)(?:\.git)?$`)
)

// IsSupportedForge checks if the given host is an authorized forge (codeberg.org or github.com).
func IsSupportedForge(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "codeberg.org" || h == "github.com"
}

func parseForgeURL(raw string) (host, owner, repo string, ok bool) {
	raw = strings.TrimSpace(raw)
	if m := sshRemoteRegex.FindStringSubmatch(raw); m != nil {
		return m[1], m[2], m[3], true
	}
	if m := httpsRemoteRegex.FindStringSubmatch(raw); m != nil {
		return m[1], m[2], m[3], true
	}
	return "", "", "", false
}

type remoteEntry struct {
	name  string
	url   string
	host  string
	owner string
	repo  string
	valid bool
}

// DetectPrioritizedForgeInfo inspects all configured git remotes based on URL, prioritizing which
// remote to push and release against in the following order:
//  1. Priority 1: origin if its URL points to codeberg.org or github.com.
//  2. Priority 2: Any remote (e.g. codeberg, forgejo, upstream) whose URL points to codeberg.org.
//  3. Priority 3: Any remote (e.g. github, mirror) whose URL points to github.com.
//
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

		host, owner, repo, ok := parseForgeURL(rawURL)
		entries = append(entries, remoteEntry{
			name:  name,
			url:   rawURL,
			host:  host,
			owner: owner,
			repo:  repo,
			valid: ok && IsSupportedForge(host),
		})
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
		if e.valid && strings.EqualFold(e.host, "codeberg.org") {
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
		if e.valid && strings.EqualFold(e.host, "github.com") {
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

	// 1. Attempt fj release create with title, tag, and body
	args := []string{"release", "create", title, "--tag", tagName, "-b", title}
	if remoteName != "" && remoteName != "origin" {
		args = append(args, "-R", remoteName)
	}
	for _, a := range attachments {
		args = append(args, "-a", a)
	}

	cmd := exec.Command("fj", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := string(out)

	// If create failed (e.g. 500 on multi-attachment or release already exists), try creating bare release if needed
	if !isContinue && !strings.Contains(outStr, "already exists") && !strings.Contains(outStr, "release exists") {
		bareArgs := []string{"release", "create", title, "--tag", tagName, "-b", title}
		if remoteName != "" && remoteName != "origin" {
			bareArgs = append(bareArgs, "-R", remoteName)
		}
		bareCmd := exec.Command("fj", bareArgs...)
		bareCmd.Dir = dir
		bareCmd.Env = os.Environ()
		_, _ = bareCmd.CombinedOutput()
	}

	// Attach assets individually via fj release asset create
	var attachErr error
	for _, a := range attachments {
		// Try release title first, then tagName
		attachArgs := []string{"release", "asset", "create"}
		if remoteName != "" && remoteName != "origin" {
			attachArgs = append(attachArgs, "-R", remoteName)
		}
		attachArgs = append(attachArgs, title, a)
		attachCmd := exec.Command("fj", attachArgs...)
		attachCmd.Dir = dir
		attachCmd.Env = os.Environ()
		if aOut, aErr := attachCmd.CombinedOutput(); aErr != nil {
			if strings.Contains(string(aOut), "already exists") {
				continue
			}
			// Fallback with tagName
			fallbackArgs := []string{"release", "asset", "create"}
			if remoteName != "" && remoteName != "origin" {
				fallbackArgs = append(fallbackArgs, "-R", remoteName)
			}
			fallbackArgs = append(fallbackArgs, tagName, a)
			fallbackCmd := exec.Command("fj", fallbackArgs...)
			fallbackCmd.Dir = dir
			fallbackCmd.Env = os.Environ()
			if fbOut, fbErr := fallbackCmd.CombinedOutput(); fbErr != nil {
				if !strings.Contains(string(fbOut), "already exists") {
					attachErr = fmt.Errorf("attach %s to %s: %w (%s)", a, title, aErr, strings.TrimSpace(string(aOut)))
				}
			}
		}
	}

	if attachErr == nil {
		return nil
	}

	return fmt.Errorf("fj release publish failed: %w\nOutput: %s", err, outStr)
}
