package claude

import (
	"fmt"
	"os"
	"path/filepath"
)

// skillAdoptionStatus permits migration only for a plain SKILL.md whose
// declared frontmatter name proves it matches the configured skill.
func skillAdoptionStatus(dir, skillName string) (bool, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, "cannot inspect directory contents: " + err.Error()
	}
	if len(entries) != 1 || entries[0].Name() != "SKILL.md" || entries[0].IsDir() {
		return false, "directory contains files beyond SKILL.md"
	}
	info, err := entries[0].Info()
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

func reportUnmanagedSkill(path, reason string) {
	marker := filepath.Join(path, ManagedSkillMarker)
	fmt.Printf("  kept %s: %s; take over after review with: touch %q\n", path, reason, marker)
}
