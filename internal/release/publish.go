package release

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type forgeCommand func(...string) ([]byte, error)

func fjCommand(dir, remote string) forgeCommand {
	return func(args ...string) ([]byte, error) {
		if remote != "" {
			args = append(args, "-R", remote)
		}
		cmd := exec.Command("fj", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LANGUAGE=en_US")
		return cmd.CombinedOutput()
	}
}

func forgeError(action string, out []byte, err error) error {
	text := strings.TrimSpace(string(out))
	hint := ""
	lower := strings.ToLower(text)
	if strings.Contains(lower, "token") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "401") || strings.Contains(lower, "authentication") {
		hint = "; authenticate again with `fj auth login`"
	}
	return fmt.Errorf("fj %s: %w (%s)%s", action, err, text, hint)
}

// fj has no JSON output. Its minimal view ends with a counted asset block,
// including two generated source archives. Parse from the end, never the body.
func releaseAssets(out []byte) (map[string]bool, error) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	assets := make(map[string]bool)
	end := len(lines)
	start := end
	for start > 0 && strings.HasPrefix(lines[start-1], "- ") {
		start--
	}
	if start == end {
		return assets, nil
	}
	if start == 0 {
		return nil, fmt.Errorf("invalid fj release asset listing")
	}
	count := strings.Fields(lines[start-1])
	if len(count) != 2 || count[1] != "assets" {
		return nil, fmt.Errorf("invalid fj release asset listing")
	}
	n, err := strconv.Atoi(count[0])
	if err != nil || n != end-start || n < 3 || lines[end-2] != "- source.zip" || lines[end-1] != "- source.tar.gz" {
		return nil, fmt.Errorf("incomplete fj release asset listing")
	}
	for _, line := range lines[start : end-2] {
		assets[strings.TrimPrefix(line, "- ")] = true
	}
	return assets, nil
}

func publishForgejoRelease(dir, tag, title string, attachments []string, out io.Writer, run forgeCommand) error {
	// Use the same release name as creation/attachment. fj's title lookup
	// reports an explicit absence after listing releases; auth/network errors
	// must never be mistaken for a missing release.
	view, err := run("--style", "minimal", "release", "view", title)
	exists := err == nil
	if err != nil && !strings.Contains(string(view), "could not find release "+title) {
		return forgeError("list release assets", view, err)
	}
	assets := make(map[string]bool)
	if exists {
		assets, err = releaseAssets(view)
		if err != nil {
			return err
		}
	}
	if assets["SHA256SUMS"] {
		tmp, err := os.MkdirTemp("", "harnez-release-checksums-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		path := filepath.Join(tmp, "SHA256SUMS")
		data, err := run("release", "asset", "download", title, "SHA256SUMS", "--output", path)
		if err != nil {
			return forgeError("download published SHA256SUMS", data, err)
		}
		published, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := verifyPublishedSet(dir, attachments, published); err != nil {
			return err
		}
	} else if len(assets) > 0 {
		return fmt.Errorf("release has assets but no published SHA256SUMS; cannot safely mix sets; resolve manually (no remote assets deleted)")
	}
	if !exists {
		data, err := run("release", "create", title, "--tag", tag, "-b", title)
		if err != nil {
			return forgeError("create release", data, err)
		}
	}
	for _, path := range attachments {
		name := filepath.Base(path)
		if assets[name] {
			fmt.Fprintf(out, "  [publish]   %s exists, skipped\n", name)
			continue
		}
		data, err := run("release", "asset", "create", title, path)
		if err != nil {
			return forgeError("upload "+name, data, err)
		}
		assets[name] = true
	}
	return nil
}

func verifyPublishedSet(dir string, attachments []string, published []byte) error {
	sums, err := parseChecksums(published)
	if err != nil {
		return fmt.Errorf("published SHA256SUMS: %w", err)
	}
	localData, err := os.ReadFile(filepath.Join(dir, "dist", "SHA256SUMS"))
	if err != nil {
		return err
	}
	local, err := parseChecksums(localData)
	if err != nil {
		return err
	}
	if len(local) != len(sums) {
		return fmt.Errorf("checksum mismatch: local and published SHA256SUMS describe different sets; resolve manually")
	}
	for name, sum := range sums {
		if local[name] != sum {
			return fmt.Errorf("checksum mismatch for %s against published SHA256SUMS; resolve manually", name)
		}
		if err := verifyChecksum(filepath.Join(dir, "dist", name), sum); err != nil {
			return err
		}
	}
	for _, path := range attachments {
		name := filepath.Base(path)
		if name == "SHA256SUMS" || strings.HasSuffix(name, ".minisig") {
			continue
		}
		if _, ok := sums[name]; !ok {
			return fmt.Errorf("%s missing from published SHA256SUMS; cannot mix sets", name)
		}
	}
	return nil
}
