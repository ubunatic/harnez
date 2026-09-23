package find

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadIndex(t *testing.T) {
	tmpDir := t.TempDir()

	chunks := []Chunk{
		{
			FilePath:   "auth/user.go",
			Package:    "auth",
			Kind:       KindType,
			Identifier: "User",
			LineStart:  10,
			LineEnd:    15,
			Summary:    "type User struct",
			Imports:    []string{"net/http"},
		},
	}

	if err := SaveIndex(tmpDir, chunks); err != nil {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	payload, err := LoadIndex(tmpDir)
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	if payload.Version != CurrentIndexVersion {
		t.Errorf("expected version %d, got %d", CurrentIndexVersion, payload.Version)
	}

	if len(payload.Chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(payload.Chunks))
	}

	if payload.Chunks[0].Identifier != "User" {
		t.Errorf("expected Identifier User, got %q", payload.Chunks[0].Identifier)
	}
}

func TestLoadIndexCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	harnezDir := filepath.Join(tmpDir, ".harnez")
	os.MkdirAll(harnezDir, 0o755)

	indexPath := filepath.Join(harnezDir, "index.json")
	os.WriteFile(indexPath, []byte("{invalid json"), 0o644)

	_, err := LoadIndex(tmpDir)
	if err == nil {
		t.Errorf("expected error on corrupted index file, got nil")
	}
}
