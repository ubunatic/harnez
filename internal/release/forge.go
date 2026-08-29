package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
	sshRemoteRegex   = regexp.MustCompile(`^(?:ssh://)?(?:[^@]+@)?([^:/]+)[:/]([^/]+)/([^/]+?)(?:\.git)?$`)
	httpsRemoteRegex = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/([^/]+?)(?:\.git)?$`)
)

// DetectForgeInfo extracts host, owner, and repo from the git origin remote.
func DetectForgeInfo(dir string) (*ForgeInfo, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git remote get-url origin: %w", err)
	}

	raw := strings.TrimSpace(string(out))
	if m := sshRemoteRegex.FindStringSubmatch(raw); m != nil {
		return &ForgeInfo{
			Host:  m[1],
			Owner: m[2],
			Repo:  m[3],
		}, nil
	}
	if m := httpsRemoteRegex.FindStringSubmatch(raw); m != nil {
		return &ForgeInfo{
			Host:  m[1],
			Owner: m[2],
			Repo:  m[3],
		}, nil
	}

	return nil, fmt.Errorf("unable to parse git remote origin URL %q", raw)
}

// GetForgeToken retrieves an API token from environment variables.
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

	// First attempt standard fj release create
	args := []string{"release", "create", title, "--tag", tagName}
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
	// If the release already exists (common with --continue), attach missing assets individually
	if isContinue || strings.Contains(outStr, "already exists") || strings.Contains(outStr, "release exists") {
		var attachErr error
		for _, a := range attachments {
			attachCmd := exec.Command("fj", "release", "asset", "create", tagName, a)
			attachCmd.Dir = dir
			attachCmd.Env = os.Environ()
			if aOut, aErr := attachCmd.CombinedOutput(); aErr != nil {
				// Don't fail if asset already exists
				if !strings.Contains(string(aOut), "already exists") {
					attachErr = fmt.Errorf("attach %s to %s: %w (%s)", a, tagName, aErr, strings.TrimSpace(string(aOut)))
				}
			}
		}
		if attachErr == nil {
			return nil
		}
	}

	return fmt.Errorf("fj release create failed: %w\nOutput: %s", err, outStr)
}
