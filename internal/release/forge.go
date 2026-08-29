package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ForgeInfo contains repository remote coordinates.
type ForgeInfo struct {
	Host  string
	Owner string
	Repo  string
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

// DetectForgeInfo extracts host, owner, and repo from the git origin remote and validates that the host is supported.
func DetectForgeInfo(dir string) (*ForgeInfo, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("unsupported or missing forge remote host %q for origin (must be on codeberg.org or github.com to publish releases)", "")
	}

	raw := strings.TrimSpace(string(out))
	var host, owner, repo string
	if m := sshRemoteRegex.FindStringSubmatch(raw); m != nil {
		host, owner, repo = m[1], m[2], m[3]
	} else if m := httpsRemoteRegex.FindStringSubmatch(raw); m != nil {
		host, owner, repo = m[1], m[2], m[3]
	}

	if !IsSupportedForge(host) {
		return nil, fmt.Errorf("unsupported or missing forge remote host %q for origin (must be on codeberg.org or github.com to publish releases)", host)
	}

	return &ForgeInfo{
		Host:  host,
		Owner: owner,
		Repo:  repo,
	}, nil
}

// GetForgeToken retrieves an API token from environment variables or ~/.local/share/forgejo-cli/keys.json.
func GetForgeToken(host string) string {
	switch strings.ToLower(host) {
	case "codeberg.org":
		if t := os.Getenv("CODEBERG_TOKEN"); t != "" {
			return t
		}
	}
	for _, env := range []string{"FORGEJO_TOKEN", "CODEBERG_TOKEN", "FJ_TOKEN", "GITEA_TOKEN"} {
		if t := os.Getenv(env); t != "" {
			return t
		}
	}

	// Fallback: check fj configuration (~/.local/share/forgejo-cli/keys.json)
	home, err := os.UserHomeDir()
	if err == nil {
		keysFile := filepath.Join(home, ".local", "share", "forgejo-cli", "keys.json")
		if data, err := os.ReadFile(keysFile); err == nil {
			var parsed struct {
				Hosts map[string]struct {
					Token string `json:"token"`
				} `json:"hosts"`
			}
			if err := json.Unmarshal(data, &parsed); err == nil {
				if hostEntry, ok := parsed.Hosts[host]; ok && hostEntry.Token != "" {
					return hostEntry.Token
				}
				if hostEntry, ok := parsed.Hosts[strings.ToLower(host)]; ok && hostEntry.Token != "" {
					return hostEntry.Token
				}
			}
		}
	}

	return ""
}


type repoAPIResponse struct {
	HasReleases bool   `json:"has_releases"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
}

// EnsureHasReleases checks and auto-enables the 'has_releases' unit on Codeberg / Forgejo repos.
func EnsureHasReleases(forge *ForgeInfo, token string, dryRun bool) (bool, error) {
	if forge == nil || token == "" {
		return false, nil
	}

	apiURL := fmt.Sprintf("https://%s/api/v1/repos/%s/%s", forge.Host, forge.Owner, forge.Repo)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "token "+token)

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("query forge API %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Not found or units hidden
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("forge API %s returned status %d: %s", apiURL, resp.StatusCode, string(body))
	}

	var repoInfo repoAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return false, fmt.Errorf("decode forge API response: %w", err)
	}

	if repoInfo.HasReleases {
		return false, nil
	}

	// Releases are disabled -> enable them via PATCH
	if dryRun {
		return true, nil
	}

	patchPayload, _ := json.Marshal(map[string]bool{"has_releases": true})
	patchReq, err := http.NewRequest(http.MethodPatch, apiURL, bytes.NewReader(patchPayload))
	if err != nil {
		return false, err
	}
	patchReq.Header.Set("Authorization", "token "+token)
	patchReq.Header.Set("Content-Type", "application/json")

	patchResp, err := client.Do(patchReq)
	if err != nil {
		return false, fmt.Errorf("enable has_releases on %s: %w", apiURL, err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK && patchResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(patchResp.Body)
		return false, fmt.Errorf("enable has_releases failed with status %d: %s", patchResp.StatusCode, string(body))
	}

	return true, nil
}

// PublishForgejoRelease creates or updates a release using fj.
func PublishForgejoRelease(dir string, tagName string, title string, attachments []string, dryRun, isContinue bool) error {
	if dryRun {
		return nil
	}

	// 1. Attempt fj release create with title, tag, and body
	args := []string{"release", "create", title, "--tag", tagName, "-b", title}
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
		bareCmd := exec.Command("fj", "release", "create", title, "--tag", tagName, "-b", title)
		bareCmd.Dir = dir
		bareCmd.Env = os.Environ()
		_, _ = bareCmd.CombinedOutput()
	}

	// Attach assets individually via fj release asset create
	var attachErr error
	for _, a := range attachments {
		// Try release title first, then tagName
		attachCmd := exec.Command("fj", "release", "asset", "create", title, a)
		attachCmd.Dir = dir
		attachCmd.Env = os.Environ()
		if aOut, aErr := attachCmd.CombinedOutput(); aErr != nil {
			if strings.Contains(string(aOut), "already exists") {
				continue
			}
			// Fallback with tagName
			fallbackCmd := exec.Command("fj", "release", "asset", "create", tagName, a)
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
