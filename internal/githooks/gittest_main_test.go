package githooks

import (
	"os"
	"testing"

	"ubunatic.com/harnez/internal/gittest"
)

// TestMain hides the host's git config from these tests (issue 1120) and
// removes the harnez binary the hook tests build (issue 1206).
func TestMain(m *testing.M) {
	cleanup := gittest.MustIsolate()
	code := m.Run()
	cleanup()
	removeHarnezBuild()
	os.Exit(code)
}
