package release

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		if err := verifyChecksum(filepath.Join(dist, name), sum); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
	}
	return len(sums) > 0, nil
}
