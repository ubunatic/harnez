package exec

import (
	"os"
	"testing"

	"ubunatic.com/harnez/internal/gittest"
)

// TestMain hides the host's git config from these tests (issue 1120).
func TestMain(m *testing.M) {
	cleanup := gittest.MustIsolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
