package release

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// InferBumpFromTags inspects recent stable semver git tags in dir and returns
// the inferred bump ("minor" or "patch") along with a human-readable reason.
// Major bumps are never inferred automatically.
func InferBumpFromTags(dir string) (string, string) {
	cmd := exec.Command("git", "tag", "--list")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "patch", "unable to inspect git tags"
	}

	rawTags := strings.Split(strings.TrimSpace(string(out)), "\n")
	return inferBumpFromTagList(rawTags)
}

func inferBumpFromTagList(rawTags []string) (string, string) {
	seen := make(map[[3]int]bool)
	var stable []Semver
	for _, raw := range rawTags {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		v, err := ParseSemver(raw)
		if err != nil || v.Prerelease != "" {
			continue
		}
		key := [3]int{v.Major, v.Minor, v.Patch}
		if seen[key] {
			continue
		}
		seen[key] = true
		stable = append(stable, v)
	}

	sort.Slice(stable, func(i, j int) bool {
		if stable[i].Major != stable[j].Major {
			return stable[i].Major > stable[j].Major
		}
		if stable[i].Minor != stable[j].Minor {
			return stable[i].Minor > stable[j].Minor
		}
		return stable[i].Patch > stable[j].Patch
	})

	if len(stable) > 4 {
		stable = stable[:4]
	}

	if len(stable) == 0 {
		return "patch", "no stable tags found"
	}
	if len(stable) < 2 {
		return "patch", fmt.Sprintf("fewer than 2 stable tags (%s)", stable[0].TagName())
	}

	var minorCount, patchCount int
	var newestKind string
	for i := 0; i < len(stable)-1; i++ {
		newer := stable[i]
		older := stable[i+1]
		if newer.Major != older.Major {
			continue
		}
		var kind string
		switch {
		case newer.Minor > older.Minor && newer.Patch == 0:
			minorCount++
			kind = "minor"
		case newer.Minor == older.Minor && newer.Patch > older.Patch:
			patchCount++
			kind = "patch"
		}
		if kind != "" && newestKind == "" {
			newestKind = kind
		}
	}

	// Format inspected tags oldest-to-newest for readability.
	names := make([]string, len(stable))
	for i, v := range stable {
		names[len(stable)-1-i] = v.TagName()
	}
	reason := "from recent tags: " + strings.Join(names, " -> ")

	if minorCount > patchCount || (minorCount > 0 && minorCount == patchCount && newestKind == "minor") {
		return "minor", reason
	}
	return "patch", reason
}
