package server

import (
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/telemetry"
)

func TestInsertAgyCompactionPair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.sqlite")
	if err := InsertAgyCompactionPair(path, "agy-session"); err != nil {
		t.Fatal(err)
	}
	db, err := telemetry.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	pair, err := db.LatestCompactionPair("agy-session")
	if err != nil || pair == nil || pair.Interrupted || pair.Source != "stream_fallback" || pair.Trigger != "manual" {
		t.Fatalf("pair = %+v, error = %v", pair, err)
	}
}
