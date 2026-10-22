package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type releaseAsset struct {
	Name string
	Path string
}

type releaseArtifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func readArtifactManifest(dir string) ([]releaseArtifact, bool, error) {
	manifestPath := filepath.Join(dir, "dist", "artifacts.json")
	data, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var artifacts []releaseArtifact
	if err := json.Unmarshal(data, &artifacts); err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", manifestPath, err)
	}
	return artifacts, true, nil
}

func artifactPath(dir string, artifact releaseArtifact) (string, error) {
	if artifact.Path == "" {
		return "", fmt.Errorf("artifact %s has an empty path", artifact.Name)
	}
	path := filepath.Clean(filepath.FromSlash(artifact.Path))
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact %s path %q escapes project directory", artifact.Name, artifact.Path)
	}
	return path, nil
}

func releaseAssetsFromDist(dir, projectName, tagName string) ([]releaseAsset, error) {
	distDir := filepath.Join(dir, "dist")
	artifacts, hasManifest, err := readArtifactManifest(dir)
	if err != nil {
		return nil, err
	}
	var candidates []releaseAsset
	if hasManifest {
		for _, artifact := range artifacts {
			if filepath.Base(artifact.Name) != artifact.Name || artifact.Name == "." || artifact.Name == ".." {
				return nil, fmt.Errorf("invalid artifact name %q in dist/artifacts.json", artifact.Name)
			}
			if !releaseArtifactMatches(artifact.Name, projectName, tagName, distDir) {
				continue
			}
			path, err := artifactPath(dir, artifact)
			if err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, releaseAsset{Name: artifact.Name, Path: rel})
		}
	} else {
		entries, err := os.ReadDir(distDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read dist directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !releaseArtifactMatches(entry.Name(), projectName, tagName, distDir) {
				continue
			}
			candidates = append(candidates, releaseAsset{Name: entry.Name(), Path: filepath.Join("dist", entry.Name())})
		}
	}
	seen := make(map[string]bool, len(candidates))
	assets := make([]releaseAsset, 0, len(candidates)+2)
	for _, asset := range candidates {
		if !seen[asset.Name] {
			seen[asset.Name] = true
			assets = append(assets, asset)
		}
	}
	for _, name := range []string{"SHA256SUMS", "SHA256SUMS.minisig"} {
		path := filepath.Join(distDir, name)
		if !fileExists(path) || seen[name] {
			continue
		}
		if name == "SHA256SUMS" && !checksumMatchesVersion(path, "", tagName) ||
			name == "SHA256SUMS.minisig" && !checksumMatchesVersion(filepath.Join(distDir, "SHA256SUMS"), "", tagName) {
			continue
		}
		assets = append(assets, releaseAsset{Name: name, Path: filepath.Join("dist", name)})
		seen[name] = true
	}
	return assets, nil
}

func releaseArtifactMatches(name, projectName, tagName, distDir string) bool {
	if !artifactMatchesVersion(name, tagName) {
		return false
	}
	if name == "SHA256SUMS" && !checksumMatchesVersion(filepath.Join(distDir, name), "", tagName) {
		return false
	}
	if name == "SHA256SUMS.minisig" && !checksumMatchesVersion(filepath.Join(distDir, "SHA256SUMS"), "", tagName) {
		return false
	}
	return strings.HasSuffix(name, ".tar.gz") ||
		strings.HasSuffix(name, ".zip") ||
		strings.HasSuffix(name, ".minisig") ||
		name == "SHA256SUMS" ||
		strings.HasPrefix(name, projectName+"-")
}

func parseChecksums(data []byte) (map[string]string, error) {
	sums := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid SHA256SUMS entry %q", line)
		}
		name := strings.TrimPrefix(fields[1], "*")
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != sha256.Size || filepath.Base(name) != name || name == "." || name == ".." {
			return nil, fmt.Errorf("invalid SHA256SUMS entry %q", line)
		}
		if _, exists := sums[name]; exists {
			return nil, fmt.Errorf("duplicate SHA256SUMS entry %s", name)
		}
		sums[name] = strings.ToLower(fields[0])
	}
	return sums, nil
}

func verifyChecksum(path, want string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: local %s, SHA256SUMS %s; preserve the published set and resolve manually", filepath.Base(path), got, want)
	}
	return nil
}

func checksumArtifactPath(dir, name string) (string, error) {
	artifacts, exists, err := readArtifactManifest(dir)
	if err != nil {
		return "", err
	}
	if exists {
		for _, artifact := range artifacts {
			if artifact.Name != name {
				continue
			}
			return artifactPath(dir, artifact)
		}
	}
	return filepath.Join(dir, "dist", name), nil
}

// completeDist treats the checksum manifest as the expected artifact inventory.
// Missing files require a rebuild; corrupt files require explicit recovery.
func completeDist(dir, version string) (bool, error) {
	dist := filepath.Join(dir, "dist")
	for _, name := range []string{"SHA256SUMS", "SHA256SUMS.minisig"} {
		info, err := os.Stat(filepath.Join(dist, name))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return false, nil
		}
	}
	path := filepath.Join(dist, "SHA256SUMS")
	if !checksumMatchesVersion(path, version, "") {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	sums, err := parseChecksums(data)
	if err != nil {
		return false, err
	}
	for name, sum := range sums {
		artifactPath, err := checksumArtifactPath(dir, name)
		if err != nil {
			return false, err
		}
		if err := verifyChecksum(artifactPath, sum); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
	}
	return len(sums) > 0, nil
}
