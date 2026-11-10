package claude

import (
	"fmt"
	"io/fs"
	"os"

	"ubunatic.com/harnez/internal/markdown"
)

// initDocPreflight resolves each destination's variant and validates all selected
// docs before init edits local sections or copies any doc.
func initDocPreflight(cfg *Config, dir string, docs []string, variant string) (map[string]string, error) {
	// Resolve an omitted variant independently for each destination. Explicit
	// "lite" or "full" values always override the existing copy.
	docVariants := make(map[string]string, len(docs))
	for _, name := range docs {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok || lang.Local == "" {
			continue
		}
		resolved := variant
		if resolved == "" {
			data, err := os.ReadFile(localPath(dir, lang.Local))
			if err == nil {
				resolved = markdown.ParseVariantMarker(string(data))
			}
			if resolved == "lite" {
				if lang.LiteSource == "" {
					fmt.Printf("  ⚠️  %s has lite marker but no lite source; using default source\n", name)
					resolved = ""
				} else if _, err := fs.Stat(cfg.FS, lang.LiteSource); err != nil {
					fmt.Printf("  ⚠️  %s has lite marker but no lite source; using default source\n", name)
					resolved = ""
				}
			}
		}
		docVariants[name] = resolved
	}
	// Check every selected doc before copying any of them. A markerless
	// local edit must never be lost halfway through a multi-doc init.
	for _, name := range docs {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok || lang.Local == "" {
			continue
		}
		data, err := fs.ReadFile(cfg.FS, lang.SourceFor(docVariants[name]))
		if err != nil {
			return nil, fmt.Errorf("language %s: read source: %w", name, err)
		}
		if _, _, err := prepareManagedDoc(localPath(dir, lang.Local), data); err != nil {
			return nil, fmt.Errorf("language %s: copy: %w", name, err)
		}
	}
	return docVariants, nil
}
