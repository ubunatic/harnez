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

// ClaudeTranscriptFilesMulti discovers top-level session transcripts across
// multiple Claude config directories in list order. A session ID (basename
// without .jsonl) seen in an earlier directory wins; missing or unreadable
// directories are skipped. Subagent transcripts in nested directories are excluded.
func ClaudeTranscriptFilesMulti(dirs []string) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string
	for _, dir := range dirs {
		files, err := ClaudeTranscriptFiles(dir)
		if err != nil {
			continue
		}
		for _, p := range files {
			id := strings.TrimSuffix(filepath.Base(p), ".jsonl")
			if seen[id] {
				continue
			}
			seen[id] = true
			paths = append(paths, p)
		}
	}
	return paths, nil
}
