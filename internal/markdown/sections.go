package markdown

import (
	"fmt"
	"strings"
)

// ManagedSection names a stable marker ID and its desired Markdown content.
type ManagedSection struct {
	Name, Content string
}

// ReconcileSections updates sections in place, prunes retired Harnez blocks,
// and appends new ones. Every byte outside complete marker pairs is retained.
// Existing section names remain their IDs, including Tool Feedback Protocol.
func ReconcileSections(existing string, sections []ManagedSection) (string, error) {
	original := existing
	desired := make(map[string]string, len(sections))
	for _, section := range sections {
		if section.Name == "" || strings.ContainsAny(section.Name, "\r\n<>") {
			return "", fmt.Errorf("invalid managed section ID %q", section.Name)
		}
		if _, duplicate := desired[section.Name]; duplicate {
			return "", fmt.Errorf("duplicate managed section ID %q", section.Name)
		}
		if strings.Contains(section.Content, "<!-- harnez:") {
			return "", fmt.Errorf("nested managed markers in section %q", section.Name)
		}
		desired[section.Name] = MDMarkers.Begin(section.Name) + "\n" + strings.TrimRight(section.Content, "\n") + "\n" + MDMarkers.End(section.Name) + "\n"
	}
	var result strings.Builder
	seen := make(map[string]bool)
	const prefix = "<!-- harnez:begin "
	for {
		start := strings.Index(existing, prefix)
		if start < 0 {
			result.WriteString(existing)
			break
		}
		result.WriteString(existing[:start])
		existing = existing[start:]
		nameEnd := strings.Index(existing, " -->")
		if nameEnd < 0 {
			result.WriteString(existing)
			break
		}
		name := existing[len(prefix):nameEnd]
		endMarker := MDMarkers.End(name)
		end := strings.Index(existing[nameEnd+4:], endMarker)
		if end < 0 || strings.Contains(existing[nameEnd+4:][:end], prefix) {
			// Preserve malformed/nested blocks rather than risk deleting user text.
			result.WriteString(existing)
			break
		}
		end += nameEnd + 4 + len(endMarker)
		if end < len(existing) && existing[end] == '\n' {
			end++
		}
		if block, keep := desired[name]; keep && !seen[name] {
			result.WriteString(block)
			seen[name] = true
		}
		existing = existing[end:]
	}
	for _, section := range sections {
		// A malformed block may have stopped reconciliation before this ID.
		// Preserve it and avoid appending another copy on every apply.
		if !seen[section.Name] && !strings.Contains(original, MDMarkers.Begin(section.Name)) {
			result.WriteString(desired[section.Name])
		}
	}
	return result.String(), nil
}
