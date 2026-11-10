package claude

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// skillAdoptionStatus permits migration only for a SKILL.md whose declared
// frontmatter name proves it matches the configured skill. Besides SKILL.md
// the directory may hold only the skill's own resources, byte-identical to
// what harnez ships; any other or edited file keeps the directory unmanaged.
func skillAdoptionStatus(dir, skillName string, resources []skillResourceContent) (bool, string) {
	if reason := unexpectedSkillFiles(dir, resources); reason != "" {
		return false, reason
	}
	info, err := os.Lstat(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return false, "cannot inspect SKILL.md: " + err.Error()
	}
	if !info.Mode().IsRegular() {
		return false, "SKILL.md is not a regular file"
	}
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return false, "cannot read SKILL.md: " + err.Error()
	}
	metadata, _, _, hasFrontmatter, err := parseSkillFrontmatter(string(data))
	if err != nil {
		return false, "SKILL.md frontmatter is invalid: " + err.Error()
	}
	name, ok := metadata["name"].(string)
	if !hasFrontmatter || !ok || name != skillName {
		return false, fmt.Sprintf("SKILL.md frontmatter name does not match configured skill %q", skillName)
	}
	return true, ""
}

// unexpectedSkillFiles returns a reason when dir holds anything besides
// SKILL.md and unmodified copies of the configured resources.
func unexpectedSkillFiles(dir string, resources []skillResourceContent) string {
	want := make(map[string][]byte, len(resources))
	for _, r := range resources {
		want[r.target] = r.data
	}
	reason := ""
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel == "." || d.IsDir() {
			return nil
		}
		if rel == "SKILL.md" {
			return nil
		}
		data, ok := want[rel]
		if !ok || !d.Type().IsRegular() {
			reason = "directory contains files beyond SKILL.md and the skill's resources"
			return fs.SkipAll
		}
		got, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, data) {
			reason = fmt.Sprintf("resource %s differs from the shipped version", rel)
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "cannot inspect directory contents: " + err.Error()
	}
	return reason
}

func reportUnmanagedSkill(path, reason string) {
	marker := filepath.Join(path, ManagedSkillMarker)
	fmt.Printf("  kept %s: %s; take over after review with: touch %q\n", path, reason, marker)
}
