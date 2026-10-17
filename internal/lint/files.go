package lint

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// TrackedFiles lists git-tracked files below each path in paths (relative to
// dir), or all tracked files when paths is empty. Non-regular files (symlinks,
// submodules) are skipped so every result can be read and linted.
func TrackedFiles(dir string, paths []string) ([]string, error) {
	args := append([]string{"-C", dir, "ls-files", "-z", "--"}, paths...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for _, name := range strings.Split(string(bytes.TrimRight(out, "\x00")), "\x00") {
		if name == "" {
			continue
		}
		info, err := os.Lstat(joinDir(dir, name))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}

func joinDir(dir, name string) string {
	if dir == "" || dir == "." {
		return name
	}
	return dir + string(os.PathSeparator) + name
}

// SeverityRank orders severities so a --check level can include everything at
// or above it. Unknown severities rank lowest.
func SeverityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 2
	case SeverityWarning:
		return 1
	}
	return 0
}

// ParseSeverity parses a --check level.
func ParseSeverity(s string) (Severity, error) {
	switch Severity(strings.ToLower(strings.TrimSpace(s))) {
	case "", SeverityError:
		return SeverityError, nil
	case SeverityWarning:
		return SeverityWarning, nil
	}
	return "", fmt.Errorf("unknown check level %q (expected error, warning)", s)
}

// RuleCount is the number of findings for one rule ID.
type RuleCount struct {
	RuleID   string
	Severity Severity
	Count    int
}

// SummarizeByRule counts findings per rule ID, most frequent first.
func SummarizeByRule(findings []Finding) []RuleCount {
	idx := map[string]int{}
	var out []RuleCount
	for _, f := range findings {
		i, ok := idx[f.RuleID]
		if !ok {
			i = len(out)
			idx[f.RuleID] = i
			out = append(out, RuleCount{RuleID: f.RuleID, Severity: f.Severity})
		}
		out[i].Count++
	}
	sortRuleCounts(out)
	return out
}

func sortRuleCounts(c []RuleCount) {
	sort.Slice(c, func(i, j int) bool {
		if c[i].Count != c[j].Count {
			return c[i].Count > c[j].Count
		}
		return c[i].RuleID < c[j].RuleID
	})
}

// FileCount is the number of findings in one file.
type FileCount struct {
	File  string
	Count int
}

// TopFiles counts findings per file and returns the n files with the most,
// ties broken by path.
func TopFiles(findings []Finding, n int) []FileCount {
	idx := map[string]int{}
	var out []FileCount
	for _, f := range findings {
		i, ok := idx[f.File]
		if !ok {
			i = len(out)
			idx[f.File] = i
			out = append(out, FileCount{File: f.File})
		}
		out[i].Count++
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].File < out[j].File
	})
	return out[:min(n, len(out))]
}
