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

// removeDecommissionedArtifacts removes the commands and skills named in
// cfg.Decommissioned. It runs regardless of the selection (issue 1061 D2) and
// records each removal in r for the end-of-run summary (issue 1061 review item 4).
func removeDecommissionedArtifacts(r *applyReport, target string, cfg *Config) error {
	for _, name := range cfg.Decommissioned.Commands {
		if err := decommissionedName(name); err != nil {
			return err
		}
		for _, root := range commandTargets(target, cfg) {
			artifact := filepath.Join(root, name+".md")
			if err := os.Remove(artifact); err == nil {
				r.changes++
				fmt.Printf("  removed %s\n", artifact)
				r.addRemoval(artifact, "decommissioned")
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("remove decommissioned command %s: %w", artifact, err)
			}
		}
	}
	for _, name := range cfg.Decommissioned.Skills {
		if err := decommissionedName(name); err != nil {
			return err
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
				return fmt.Errorf("inspect decommissioned skill %s: %w", artifact, err)
			}
			if err := os.RemoveAll(artifact); err != nil {
				return fmt.Errorf("remove decommissioned skill %s: %w", artifact, err)
			}
			r.changes++
			fmt.Printf("  removed %s\n", artifact)
			r.addRemoval(artifact, "decommissioned")
		}
	}
	return nil
}

// removeDroppedSkills removes only previously managed skills absent from config.
// It runs regardless of the selection (issue 1061 D2) and records each removal
// in r for the end-of-run summary (issue 1061 review item 4).
func removeDroppedSkills(r *applyReport, cfg *Config) error {
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
			return fmt.Errorf("skills dir %s: %w", skillsRoot, err)
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
				return fmt.Errorf("remove dropped skill %s: %w", skillDir, err)
			}
			fmt.Printf("  removed %s\n", skillDir)
			r.changes++
			r.addRemoval(skillDir, "not in config")
		}
	}
	return nil
}
