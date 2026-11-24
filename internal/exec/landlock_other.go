//go:build !linux

package exec

import (
	"fmt"
	"os"
	"runtime"
)

func probeLandlockABI() (int, error) {
	return 0, fmt.Errorf("landlock requires Linux (found %s)", runtime.GOOS)
}

func applyLandlockRuleset(_ []string) error {
	return fmt.Errorf("landlock requires Linux (found %s)", runtime.GOOS)
}

// MaybeRunSandboxHelper is a no-op on non-Linux platforms unless explicitly
// invoked as the Landlock helper, in which case it fails closed with exit 126.
func MaybeRunSandboxHelper() {
	if len(os.Args) >= 4 && os.Args[1] == "__landlock-exec" && os.Args[2] == "--" &&
		os.Getenv("HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS") != "" {
		fmt.Fprintf(os.Stderr, "harnez: landlock requires Linux (found %s)\n", runtime.GOOS)
		os.Exit(126)
	}
}
