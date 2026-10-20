package claude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/skillreg"
)

func decommissionedName(name string) error {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return fmt.Errorf("invalid decommissioned artifact name %q", name)
	}
	return nil
}

func removeDecommissionedArtifacts(target string, cfg *Config) (int, error) {
	changes := 0
	for _, name := range cfg.Decommissioned.Commands {
		if err := decommissionedName(name); err != nil {
			return changes, err
		}
		for _, root := range commandTargets(target, cfg) {
			artifact := filepath.Join(root, name+".md")
			if err := os.Remove(artifact); err == nil {
				changes++
				fmt.Printf("  removed %s\n", artifact)
			} else if !os.IsNotExist(err) {
				return changes, fmt.Errorf("remove decommissioned command %s: %w", artifact, err)
			}
		}
	}
	for _, name := range cfg.Decommissioned.Skills {
		if err := decommissionedName(name); err != nil {
			return changes, err
		}
		for _, root := range skillTargets(cfg) {
			artifact := filepath.Join(root, name)
			// Former bundles also carry the external marker, but apply owns them.
			if !skillreg.IsBundled(artifact) && externalSkillClash(artifact) {
				continue
			}
			if _, err := os.Stat(artifact); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return changes, fmt.Errorf("inspect decommissioned skill %s: %w", artifact, err)
			}
			if err := os.RemoveAll(artifact); err != nil {
				return changes, fmt.Errorf("remove decommissioned skill %s: %w", artifact, err)
			}
			changes++
			fmt.Printf("  removed %s\n", artifact)
		}
	}
	return changes, nil
}

// removeDroppedSkills removes only previously managed skills absent from config.
func removeDroppedSkills(cfg *Config) (int, error) {
	changes := 0
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
			return changes, fmt.Errorf("skills dir %s: %w", skillsRoot, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(skillsRoot, entry.Name())
			if _, keep := configuredSkills[entry.Name()]; keep || !isManagedSkill(skillDir) || isExternalSkill(skillDir) {
				continue
			}
			if err := os.RemoveAll(skillDir); err != nil {
				return changes, fmt.Errorf("remove dropped skill %s: %w", skillDir, err)
			}
			fmt.Printf("  removed %s\n", skillDir)
			changes++
		}
	}
	return changes, nil
}
