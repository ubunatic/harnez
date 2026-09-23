package find

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const CurrentIndexVersion = 1
const IndexFileName = ".harnez/index.json"

// LoadIndex reads .harnez/index.json under repoRoot.
func LoadIndex(repoRoot string) (*IndexPayload, error) {
	indexPath := filepath.Join(repoRoot, IndexFileName)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}

	var payload IndexPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("corrupted index file: %w", err)
	}

	if payload.Version != CurrentIndexVersion {
		return nil, fmt.Errorf("unsupported index version %d (expected %d)", payload.Version, CurrentIndexVersion)
	}

	return &payload, nil
}

// SaveIndex writes payload atomically to .harnez/index.json under repoRoot.
func SaveIndex(repoRoot string, chunks []Chunk) error {
	dir := filepath.Join(repoRoot, ".harnez")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .harnez dir: %w", err)
	}

	payload := IndexPayload{
		Version:   CurrentIndexVersion,
		Generated: time.Now().UTC().Format(time.RFC3339),
		RepoRoot:  repoRoot,
		Chunks:    chunks,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "index-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp index: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp index: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp index: %w", err)
	}

	targetPath := filepath.Join(repoRoot, IndexFileName)
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename index file: %w", err)
	}

	return nil
}
