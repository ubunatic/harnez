package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendRotatingKillRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage-kills.jsonl")
	record := killFailureRecord{
		Time: time.Unix(1, 0).UTC(), Provider: "p", Command: "cmd", Signal: "SIGKILL", Cause: "unknown",
		Findings: map[string]string{"sender": "unknown"},
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	settings := usageKillLogSpec{MaxBytes: int64(len(encoded) + 1), Backups: 1}
	for range 3 {
		if err := appendRotatingKillRecord(path, settings, record); err != nil {
			t.Fatal(err)
		}
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(append(encoded, '\n')) || string(backup) != string(current) {
		t.Fatalf("rotated records current=%q backup=%q", current, backup)
	}
	if _, err := os.Stat(path + ".2"); !os.IsNotExist(err) {
		t.Fatalf("unexpected second backup: %v", err)
	}
}
