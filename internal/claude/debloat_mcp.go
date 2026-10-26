package claude

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// mcpServerDenials preserves objects matched by command or URL as well as names.
func mcpServerDenials(settings map[string]any) ([]any, error) {
	raw, present := settings["deniedMcpServers"]
	if !present {
		return nil, nil
	}
	entries, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("deniedMcpServers must be an array; refusing to overwrite existing value")
	}
	return entries, nil
}

func hasMCPServerDenial(entries []any, name string) bool {
	return slices.ContainsFunc(entries, func(entry any) bool {
		obj, ok := entry.(map[string]any)
		return ok && obj["serverName"] == name
	})
}

func applyMCPServerDenials(settings map[string]any, rec *debloatRecord, names []string) error {
	if len(names) == 0 {
		return nil
	}
	entries, err := mcpServerDenials(settings)
	if err != nil {
		return err
	}
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("preset_denied_mcp_servers contains an empty server name")
		}
	}
	if rec.MCPDenyPresent == nil {
		_, present := settings["deniedMcpServers"]
		rec.MCPDenyPresent = &present
	}
	for _, name := range names {
		if hasMCPServerDenial(entries, name) {
			continue
		}
		entries = append(entries, map[string]any{"serverName": name})
		if !slices.Contains(rec.MCPDenyAdded, name) {
			rec.MCPDenyAdded = append(rec.MCPDenyAdded, name)
		}
	}
	sort.Strings(rec.MCPDenyAdded)
	settings["deniedMcpServers"] = entries
	return nil
}

func revertMCPServerDenials(settings map[string]any, rec debloatRecord) error {
	if rec.MCPDenyPresent == nil {
		return nil // Backward compatibility with older sidecars.
	}
	entries, err := mcpServerDenials(settings)
	if err != nil {
		return err
	}
	kept := make([]any, 0, len(entries))
	for _, entry := range entries {
		obj, ok := entry.(map[string]any)
		name, _ := obj["serverName"].(string)
		if ok && len(obj) == 1 && slices.Contains(rec.MCPDenyAdded, name) {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == 0 && !*rec.MCPDenyPresent {
		delete(settings, "deniedMcpServers")
	} else {
		settings["deniedMcpServers"] = kept
	}
	return nil
}

func statusMCPServerDenials(settings map[string]any, rec debloatRecord, names []string) {
	entries, err := mcpServerDenials(settings)
	fmt.Println("deniedMcpServers:")
	if err != nil {
		fmt.Printf("  %v\n", err)
		return
	}
	for _, name := range names {
		state := "off"
		if hasMCPServerDenial(entries, name) {
			state = "on (pre-existing)"
			if slices.Contains(rec.MCPDenyAdded, name) {
				state = "on (harnez-managed)"
			}
		}
		fmt.Printf("  %-26s %s\n", name, state)
	}
}
