package claude

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// diffSkills checks dropped, disabled and installed skill content without mutation.
func diffSkills(cfg *Config, selection Set) (bool, error) {
	anyChanged := false
	disableRateFeedback := RateFeedbackDisabled(cfg, nil)
	configuredSkills := make(map[string]struct{}, len(cfg.Skills))
	for _, skill := range cfg.Skills {
		configuredSkills[skill.Name] = struct{}{}
	}
	for _, skillsRoot := range skillTargets(cfg) {
		entries, err := os.ReadDir(skillsRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("skills dir %s: %w", skillsRoot, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(skillsRoot, entry.Name())
			if _, keep := configuredSkills[entry.Name()]; !keep && isManagedSkill(skillDir) && !isExternalSkill(skillDir) {
				anyChanged = true
			}
		}
	}

	if len(cfg.Skills) > 0 {
		for _, skill := range cfg.Skills {
			if skillDisabled(skill, selection, disableRateFeedback) {
				for _, skillsRoot := range skillTargets(cfg) {
					skillDir := filepath.Join(skillsRoot, skill.Name)
					if isExternalSkill(skillDir) || !isManagedSkill(skillDir) {
						continue
					}
					path := filepath.Join(skillDir, "SKILL.md")
					if _, err := os.Stat(path); err == nil {
						anyChanged = true
					} else if !os.IsNotExist(err) {
						return false, fmt.Errorf("skill %s [%s]: %w", skill.Name, path, err)
					}
				}
				continue
			}
			if !selection.HasComponent("skills") {
				continue // skip semantics: not managed, so never drifted
			}
			content, err := genSkillContent(skill, cfg.FS)
			if err != nil {
				return false, err
			}
			resources, err := genSkillResources(skill, cfg.FS)
			if err != nil {
				return false, err
			}
			for _, skillsRoot := range skillTargets(cfg) {
				skillDir := filepath.Join(skillsRoot, skill.Name)
				if isExternalSkill(skillDir) {
					continue
				}
				if _, err := os.Lstat(skillDir); err == nil && !isManagedSkill(skillDir) {
					adopt, reason := skillAdoptionStatus(skillDir, skill.Name)
					if !adopt {
						reportUnmanagedSkill(skillDir, reason)
						anyChanged = true
						continue
					}
					fmt.Printf("  would adopt %s\n", skillDir)
					anyChanged = true
				} else if err != nil && !errors.Is(err, os.ErrNotExist) {
					return false, fmt.Errorf("skill %s dir: %w", skill.Name, err)
				}
				path, err := safeSkillPath(skillDir, "SKILL.md")
				if err != nil {
					return false, err
				}
				existing, readErr := os.ReadFile(path)
				if readErr != nil {
					if os.IsNotExist(readErr) {
						anyChanged = true
						continue
					}
					return false, fmt.Errorf("skill %s [%s]: %w", skill.Name, path, readErr)
				}
				if string(existing) != content {
					anyChanged = true
				}
				for _, resource := range resources {
					resourcePath, err := safeSkillPath(skillDir, resource.target)
					if err != nil {
						return false, err
					}
					existing, readErr := os.ReadFile(resourcePath)
					if readErr != nil {
						if os.IsNotExist(readErr) {
							anyChanged = true
							continue
						}
						return false, fmt.Errorf("skill %s resource [%s]: %w", skill.Name, resourcePath, readErr)
					}
					if !bytes.Equal(existing, resource.data) {
						anyChanged = true
					}
				}
			}
		}
	}
	return anyChanged, nil
}
