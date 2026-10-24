package usage

import (
	"os"
	"path/filepath"
	"strings"
)

// ClaudeTranscriptFiles discovers top-level session transcripts under Claude's
// project slugs. Subagent transcripts in nested directories are excluded.
func ClaudeTranscriptFiles(claudeDir string) ([]string, error) {
	projectsDir := filepath.Join(claudeDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectDir := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue // Match usage attribution's tolerance of inaccessible projects.
		}
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
				paths = append(paths, filepath.Join(projectDir, file.Name()))
			}
		}
	}
	return paths, nil
}
