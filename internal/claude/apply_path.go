package claude

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/markdown"
)

// Hooks run through /bin/sh, which reads no rc file, so the harnez install
// directory must be on the PATH that the login shell hands to Claude Code.
// apply therefore puts it on PATH in every installed shell's startup file:
// ~/.zshenv (zsh reads it for every invocation), ~/.bashrc (when it exists)
// and a fish conf.d file that apply owns outright.

// pathSection names the managed section in POSIX startup files.
const pathSection = "path"

// fishPathFile is the fish conf.d file apply owns; it holds only the PATH line.
const fishPathFile = "harnez-path.fish"

// lookPath finds an installed shell. Tests replace it to fake shell installs.
var lookPath = exec.LookPath

// systemBinDirs are on every login shell's PATH already, so they need no entry.
var systemBinDirs = map[string]bool{
	"/bin": true, "/sbin": true, "/usr/bin": true, "/usr/sbin": true,
	"/usr/local/bin": true, "/usr/local/sbin": true,
}

// pathTarget is one startup file that carries the PATH entry.
type pathTarget struct {
	path   string
	fish   bool
	wanted bool // the shell is installed or its file already exists
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

// pathRef spells dir with $HOME when it lies under home, so the written file
// stays valid for any user who inherits it.
func pathRef(dir, home string) string {
	if home != "" && (dir == home || strings.HasPrefix(dir, home+string(filepath.Separator))) {
		return "$HOME" + dir[len(home):]
	}
	return dir
}

// posixPathBody is the managed bash and zsh body: prepend dir to PATH once.
func posixPathBody(dir, home string) string {
	ref := pathRef(dir, home)
	return "case \":$PATH:\" in\n" +
		"    *\":" + ref + ":\"*) ;;\n" +
		"    *) export PATH=\"" + ref + ":$PATH\" ;;\n" +
		"esac\n"
}

// fishPathContent is the whole fish conf.d file for dir.
func fishPathContent(dir, home string) string {
	ref := pathRef(dir, home)
	return "# Managed by harnez apply: puts the harnez install directory on PATH.\n" +
		"if not contains -- \"" + ref + "\" $PATH\n" +
		"    set -gx PATH \"" + ref + "\" $PATH\n" +
		"end\n"
}

// pathTargets lists every startup file that may carry the entry. The wanted
// flag says whether apply should write it on this machine.
func pathTargets(home string) []pathTarget {
	zshenv := filepath.Join(home, ".zshenv")
	_, zshErr := lookPath("zsh")
	fishDir := filepath.Join(home, ".config", "fish")
	_, fishErr := lookPath("fish")
	bashrc := filepath.Join(home, ".bashrc")
	return []pathTarget{
		{path: zshenv, wanted: zshErr == nil || fileExists(zshenv)},
		{path: filepath.Join(fishDir, "conf.d", fishPathFile), fish: true, wanted: fishErr == nil || fileExists(fishDir)},
		{path: bashrc, wanted: fileExists(bashrc)},
	}
}

// applyPathIntegration writes dir into each wanted startup file, reports what
// changed, and warns when the running shell does not yet have dir on PATH.
func applyPathIntegration(r *applyReport, home, dir string) error {
	if dir == "" || home == "" {
		return nil
	}
	var unchanged []string
	for _, t := range pathTargets(home) {
		if !t.wanted {
			continue
		}
		if t.fish {
			changed, err := fsutil.WriteIfChanged(t.path, []byte(fishPathContent(dir, home)))
			if err != nil {
				return fmt.Errorf("fish PATH %s: %w", t.path, err)
			}
			if changed {
				r.changes++
				fmt.Printf("  wrote %s\n", t.path)
			} else {
				unchanged = append(unchanged, fsutil.ContractHome(t.path))
			}
			continue
		}
		changed, existed, err := markdown.ApplyMK(t.path, pathSection, posixPathBody(dir, home))
		if err != nil {
			return fmt.Errorf("PATH section %s: %w", t.path, err)
		}
		if changed {
			r.changes++
			if existed {
				fmt.Printf("  updated %s [%s]\n", t.path, pathSection)
			} else {
				fmt.Printf("  wrote %s [%s]\n", t.path, pathSection)
			}
		} else {
			unchanged = append(unchanged, fsutil.ContractHome(t.path))
		}
	}
	if len(unchanged) > 0 {
		r.addStat("PATH", strings.Join(unchanged, ", "))
	}
	if !dirOnPath(dir, os.Getenv("PATH")) {
		fmt.Printf("  note: %s is not on this shell's PATH; open a new terminal so hooks find harnez\n", fsutil.ContractHome(dir))
	}
	return nil
}

// dirOnPath reports whether dir is one of the entries of a PATH value.
func dirOnPath(dir, pathValue string) bool {
	for _, entry := range filepath.SplitList(pathValue) {
		if filepath.Clean(entry) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

// diffPathIntegration reports whether any wanted startup file lacks the current entry.
func diffPathIntegration(home, dir string) bool {
	if dir == "" || home == "" {
		return false
	}
	for _, t := range pathTargets(home) {
		if !t.wanted {
			continue
		}
		if pathStateOf(t, home, dir) != "ok" {
			return true
		}
	}
	return false
}

// pathStateOf returns "ok", "missing" or "drifted" for one startup file.
func pathStateOf(t pathTarget, home, dir string) string {
	if t.fish {
		data, err := os.ReadFile(t.path)
		if err != nil {
			return "missing"
		}
		if string(data) == fishPathContent(dir, home) {
			return "ok"
		}
		return "drifted"
	}
	if !markdown.ContainsSectionMK(t.path, pathSection) {
		return "missing"
	}
	if changed, err := markdown.DiffMK(t.path, pathSection, posixPathBody(dir, home)); err != nil || changed {
		return "drifted"
	}
	return "ok"
}

// cleanPathIntegration removes the PATH entry from every startup file apply may
// have written, whether or not the shell is still installed.
func cleanPathIntegration(home string) {
	if home == "" {
		return
	}
	for _, t := range pathTargets(home) {
		if t.fish {
			if err := os.Remove(t.path); err == nil {
				fmt.Printf("  removed %s\n", t.path)
			}
			continue
		}
		if removed, _, err := markdown.CleanMK(t.path, pathSection); err == nil && removed {
			fmt.Printf("  cleaned %s [%s]\n", t.path, pathSection)
		}
	}
}
