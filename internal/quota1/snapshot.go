package quota1

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func gitOutput(root string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
}

func eligibleSnapshotPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if parts[0] == "issues" || strings.EqualFold(filepath.Ext(path), ".md") || isExcludedFile(parts[len(parts)-1]) {
		return false
	}
	for _, part := range parts[:len(parts)-1] {
		if isExcludedDir(part) {
			return false
		}
	}
	return true
}

// snapshotCandidates lists paths whose working-tree content may differ from head.
func snapshotCandidates(root, head string) (map[string]bool, error) {
	paths := make(map[string]bool)
	for _, args := range [][]string{
		{"diff", "--no-ext-diff", "--no-renames", "--name-only", "-z", head, "--"},
		{"ls-files", "--others", "--exclude-standard", "-z"},
	} {
		out, err := gitOutput(root, args...)
		if err != nil {
			return nil, fmt.Errorf("list snapshot changes: %w", err)
		}
		for _, path := range strings.Split(string(out), "\x00") {
			if path != "" && eligibleSnapshotPath(path) {
				paths[path] = true
			}
		}
	}
	return paths, nil
}

func contentHash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func workingHash(root, path string) (string, error) {
	name := filepath.Join(root, path)
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(name)
		if err != nil {
			return "", err
		}
		return contentHash([]byte(target)), nil
	}
	if !info.Mode().IsRegular() {
		return "", nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return contentHash(data), nil
}

func snapshotRun(root string, started time.Time) (RunRecord, error) {
	record := RunRecord{Started: started}
	head, err := gitOutput(root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return record, nil // Non-git directories and repositories without a commit use mtimes.
	}
	record.TestedHead = strings.TrimSpace(string(head))
	paths, err := snapshotCandidates(root, record.TestedHead)
	if err != nil {
		return record, err
	}
	record.TestedDirty = make(map[string]string, len(paths))
	for path := range paths {
		hash, err := workingHash(root, path)
		if err != nil {
			return record, fmt.Errorf("hash %s: %w", path, err)
		}
		record.TestedDirty[path] = hash
	}
	return record, nil
}

func changesSinceRecord(root, stateFile string, record RunRecord) ([]string, error) {
	if record.TestedHead == "" {
		return findModifiedSourceFiles(root, stateFile, record.Started)
	}
	paths, err := snapshotCandidates(root, record.TestedHead)
	if err != nil {
		return nil, err
	}
	for path := range record.TestedDirty {
		paths[path] = true
	}
	var changed []string
	for path := range paths {
		current, err := workingHash(root, path)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", path, err)
		}
		tested, dirty := record.TestedDirty[path]
		if !dirty {
			kind, err := gitOutput(root, "cat-file", "-t", record.TestedHead+":"+path)
			if err == nil && strings.TrimSpace(string(kind)) != "blob" {
				continue
			}
			data, err := gitOutput(root, "show", record.TestedHead+":"+path)
			if err == nil {
				tested = contentHash(data)
			}
		}
		if current != tested {
			changed = append(changed, filepath.Join(root, filepath.FromSlash(path)))
		}
	}
	sort.Strings(changed)
	return changed, nil
}
