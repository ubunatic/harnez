package usagecli

import (
	"fmt"

	"ubunatic.com/harnez/internal/usage"
)

// ResolveUsageHost selects the explicit host or its configured default.
func ResolveUsageHost(flagHost string, cfg *usage.LocalConfig) string {
	if flagHost != "" {
		return flagHost
	}
	if cfg == nil {
		return flagHost
	}
	return cfg.Usage.DefaultHost
}

// ValidateUsageFlags rejects incompatible view and output modes.
func ValidateUsageFlags(usageWatch, usageRaw, usageJSON, usageCompact, usageLoom, usageSplash bool) error {
	if usageWatch && usageJSON {
		return fmt.Errorf("--watch and --json cannot be combined")
	}
	if usageWatch && usageRaw {
		return fmt.Errorf("--watch and --raw cannot be combined")
	}
	if usageRaw && usageJSON {
		return fmt.Errorf("--raw and --json cannot be combined")
	}
	if usageCompact && usageRaw {
		return fmt.Errorf("--compact has no effect with --raw")
	}
	if usageLoom && usageRaw {
		return fmt.Errorf("--loom has no effect with --raw")
	}
	if usageLoom && usageJSON {
		return fmt.Errorf("--loom and --json cannot be combined")
	}
	if usageSplash && !usageWatch {
		return fmt.Errorf("--splash requires --watch")
	}
	return nil
}

// ValidateUsageModeFlags forbids explicit dashboard modes alongside other output modes.
func ValidateUsageModeFlags(explicitMode, usageRaw, usageJSON, usageLoom bool) error {
	if explicitMode && (usageRaw || usageJSON || usageLoom) {
		return fmt.Errorf("usage view mode flags cannot be combined with --raw, --json, or --loom")
	}
	return nil
}

// SelectUsageViewMode returns the selected explicit view, if any.
func SelectUsageViewMode(normal, compact, minimal bool) (string, bool, error) {
	selected := 0
	mode := ""
	if normal {
		selected++
		mode = "normal"
	}
	if compact {
		selected++
		mode = "compact"
	}
	if minimal {
		selected++
		mode = "minimal"
	}
	if selected > 1 {
		return "", true, fmt.Errorf("--normal, --compact, and --minimal are mutually exclusive")
	}
	return mode, selected == 1, nil
}
