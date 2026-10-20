package claude

import (
	"errors"
	"fmt"
	"os"

	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/markdown"
)

// manageGlobalSections shares apply/diff rendering and the existing Markdown
// marker IDs. Global instruction files are user-owned: never create them.
func manageGlobalSections(cfg *Config, check bool) (bool, error) {
	sections := make([]markdown.ManagedSection, 0, len(cfg.GlobalSections))
	retired := append([]string(nil), cfg.RetiredGlobalSections...)
	for _, section := range cfg.GlobalSections {
		if section.RateFeedback && RateFeedbackDisabled(cfg, nil) {
			retired = append(retired, section.Name)
			continue
		}
		sections = append(sections, markdown.ManagedSection{Name: section.Name, Content: section.Content, ExistingOnly: section.ExistingOnly})
	}
	// Validate even when no target file exists.
	if _, err := markdown.ReconcileSections("", sections, retired...); err != nil {
		return false, err
	}
	changed := false
	for _, target := range cfg.GlobalSectionTargets {
		path := fsutil.ExpandHome(target)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return changed, err
		}
		desired, err := markdown.ReconcileSections(string(data), sections, retired...)
		if err != nil {
			return changed, fmt.Errorf("global instructions %s: %w", path, err)
		}
		if desired == string(data) {
			continue
		}
		changed = true
		if check {
			if _, err := markdown.DiffContent(path, string(data), desired); err != nil {
				return changed, err
			}
			continue
		}
		if err := os.WriteFile(path, []byte(desired), 0600); err != nil {
			return changed, err
		}
		fmt.Printf("  updated global sections %s\n", path)
	}
	return changed, nil
}
