package bench

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const markerFile = "enabled"

// ErrNotSetUp is returned by Ready when `harnez bench --setup` has not run.
var ErrNotSetUp = errors.New("bench is optional and not set up; run `harnez bench --setup` first")

// DBPath is the bench database location inside dir.
func DBPath(dir string) string { return filepath.Join(dir, "bench.sqlite") }

// Setup creates the bench directory, database and enabled marker, and
// reports which agent CLIs were found on PATH. It is idempotent.
func Setup(dir string, lookPath func(string) (string, error)) (found map[string]string, err error) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	s, err := OpenStore(DBPath(dir))
	if err != nil {
		return nil, err
	}
	if err := s.Close(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, markerFile), []byte("bench enabled\n"), 0o644); err != nil {
		return nil, fmt.Errorf("bench: write marker: %w", err)
	}
	found = map[string]string{}
	for _, a := range []string{AgentClaude, AgentCodex, AgentAgy} {
		if p, err := lookPath(a); err == nil {
			found[a] = p
		}
	}
	return found, nil
}

// Ready returns ErrNotSetUp unless Setup has run for dir.
func Ready(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, markerFile)); err != nil {
		return ErrNotSetUp
	}
	return nil
}
