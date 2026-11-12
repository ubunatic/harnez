package claude

import (
	"os"
	"path/filepath"
)

// Hooks run through /bin/sh, which reads no rc file, so the harnez install
// directory must be on the PATH that the login shell hands to Claude Code.
// The PATH entry itself is written by dev/shellsetup; this file only says
// which directory harnez contributes.

// systemBinDirs are on every login shell's PATH already, so they need no entry.
var systemBinDirs = map[string]bool{
	"/bin": true, "/sbin": true, "/usr/bin": true, "/usr/sbin": true,
	"/usr/local/bin": true, "/usr/local/sbin": true,
}

// HarnezPathDir returns the resolved directory of the running harnez binary,
// normally ~/go/bin. It returns "" when the directory is already a system
// bin directory or the executable cannot be located.
func HarnezPathDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return pathEntryDir(filepath.Dir(exe))
}

func pathEntryDir(dir string) string {
	if dir == "" || systemBinDirs[dir] {
		return ""
	}
	return dir
}
