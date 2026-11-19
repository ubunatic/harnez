package release

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// InferBumpFromTags inspects recent stable semver git tags in dir that carry
// tagPrefix and returns the inferred bump ("minor" or "patch") along with a
// human-readable reason. An empty tagPrefix accepts plain semver tags, with
// an optional "v". Major bumps are never inferred automatically.
func InferBumpFromTags(dir, tagPrefix string) (string, string) {
	cmd := exec.Command("git", "tag", "--list")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "patch", "unable to inspect git tags"
	}

	rawTags := strings.Split(strings.TrimSpace(string(out)), "\n")
	return inferBumpFromTagList(rawTags, tagPrefix)
}

// parseTagVersion parses raw as a release tag. With a non-empty tagPrefix the
// tag must start with it and the prefix is stripped before ParseSemver.
func parseTagVersion(raw, tagPrefix string) (Semver, bool) {
	if tagPrefix != "" {
		if !strings.HasPrefix(raw, tagPrefix) {
			return Semver{}, false
		}
		raw = strings.TrimPrefix(raw, tagPrefix)
	}
	v, err := ParseSemver(raw)
	if err != nil {
		return Semver{}, false
	}
	return v, true
}

func inferBumpFromTagList(rawTags []string, tagPrefix string) (string, string) {
	seen := make(map[[3]int]bool)
	var stable []Semver
	for _, raw := range rawTags {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		v, ok := parseTagVersion(raw, tagPrefix)
		if !ok || v.Prerelease != "" {
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

	// tagName renders a version as the tag the repository actually uses.
	tagName := func(v Semver) string {
		if tagPrefix == "" {
			return v.TagName()
		}
		return FormatTag(tagPrefix, v.String())
	}

	if len(stable) == 0 {
		return "patch", "no stable tags found"
	}
	if len(stable) < 2 {
		return "patch", fmt.Sprintf("fewer than 2 stable tags (%s)", tagName(stable[0]))
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
		names[len(stable)-1-i] = tagName(v)
	}
	reason := "from recent tags: " + strings.Join(names, " -> ")

	if minorCount > patchCount || (minorCount > 0 && minorCount == patchCount && newestKind == "minor") {
		return "minor", reason
	}
	return "patch", reason
}
