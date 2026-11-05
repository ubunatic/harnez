package prwait

import (
	"os"
	"testing"

	"ubunatic.com/harnez/internal/gittest"
)

// TestMain hides the host's git config, e.g. url.insteadOf rewrites of
// remote URLs (issue 1120).
func TestMain(m *testing.M) {
	cleanup := gittest.MustIsolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
