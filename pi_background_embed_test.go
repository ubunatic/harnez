package harnez

import (
	"bytes"
	"os"
	"testing"
)

// The Pi background-task sources are embedded so a later installer can mirror them into the global Pi
// package. The embedded copies must match the repository files, or the installer would ship stale code.
func TestPiBackgroundTaskSourcesEmbeddedMatchRepository(t *testing.T) {
	for _, path := range []string{
		".pi/extensions/background-tasks.ts",
		".pi/lib/background-task-runner.ts",
		"spec/pi-background-tasks.yaml",
	} {
		embedded, err := DefaultFS.ReadFile(path)
		if err != nil {
			t.Fatalf("embedded %s: %v", path, err)
		}
		onDisk, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(embedded, onDisk) {
			t.Errorf("embedded %s differs from the repository file", path)
		}
	}
}
