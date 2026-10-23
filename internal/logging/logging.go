package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Debugf appends a timestamped line to ~/.harnez/debug.log when DEBUG is set.
// Logging is best effort and never affects the caller's result.
func Debugf(format string, args ...any) {
	enabled := os.Getenv("DEBUG")
	if enabled != "1" && !strings.EqualFold(enabled, "true") {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(home, ".harnez", "debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "[%s] "+format+"\n", append([]any{time.Now().UTC().Format(time.RFC3339Nano)}, args...)...)
}
