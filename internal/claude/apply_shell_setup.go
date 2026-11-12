package claude

import (
	"fmt"
	"os"
	"strings"

	"ubunatic.com/harnez/dev/shellsetup"
	"ubunatic.com/harnez/internal/fsutil"
)

// Shell setup for harnez: the PATH entry and the completion scripts are
// written by dev/shellsetup. This file supplies harnez's values: its name,
// its install directory, and the completion renderer that the command tree
// registers through SetCompletionRenderer.

// shellOwner names harnez's managed shell sections and its ~/.harnez directory.
const shellOwner = "harnez"

// CompletionRenderer renders the cobra completion script for one shell:
// "bash", "zsh" or "fish". The command tree installs it; without one, apply
// skips completion entirely.
type CompletionRenderer func(shell string) ([]byte, error)

var completionRenderer CompletionRenderer

// SetCompletionRenderer installs the renderer apply uses for completion scripts.
func SetCompletionRenderer(r CompletionRenderer) {
	completionRenderer = r
}

// shellSetup returns the installer for home: harnez's PATH entry, plus
// completion when a renderer is registered.
func shellSetup(home string) (*shellsetup.Installer, error) {
	cfg := shellsetup.Config{Home: home, Owner: shellOwner, PathDir: HarnezPathDir()}
	if render := completionRenderer; render != nil {
		cfg.Completions = []shellsetup.Completion{{
			Command: shellOwner,
			Script: func(shell shellsetup.Shell) ([]byte, error) {
				return render(string(shell))
			},
		}}
	}
	return shellsetup.New(cfg)
}

// applyShellSetup writes the PATH entry and completion, then reports what changed.
func applyShellSetup(r *applyReport, home string) error {
	inst, err := shellSetup(home)
	if err != nil {
		return fmt.Errorf("shell setup: %w", err)
	}
	res, err := inst.Apply()
	if err != nil {
		return err
	}
	for _, c := range res.Changes {
		r.changes++
		fmt.Printf("  %s\n", describeShellChange(c))
	}
	var pathStats, completionStats []string
	for _, c := range res.Unchanged {
		p := fsutil.ContractHome(c.Path)
		if c.Feature == shellsetup.FeaturePath {
			pathStats = append(pathStats, p)
		} else {
			completionStats = append(completionStats, p)
		}
	}
	if len(pathStats) > 0 {
		r.addStat("PATH", strings.Join(pathStats, ", "))
	}
	if len(completionStats) > 0 {
		r.addStat("completion", strings.Join(completionStats, ", "))
	}
	if res.PathMissing {
		fmt.Printf("  note: %s is not on this shell's PATH; open a new terminal so hooks find harnez\n", fsutil.ContractHome(HarnezPathDir()))
	}
	return nil
}

// shellSetupPending reports whether the PATH entry or completion is missing or stale.
func shellSetupPending(home string) bool {
	inst, err := shellSetup(home)
	return err == nil && inst.Pending()
}

// shellSetupChecks returns the status rows for the PATH entry and completion.
func shellSetupChecks(home string) []shellsetup.Check {
	inst, err := shellSetup(home)
	if err != nil {
		return nil
	}
	return inst.Checks()
}

// cleanShellSetup removes the PATH entry and completion, whether or not the
// shells are still installed, and reports what it removed.
func cleanShellSetup(home string) {
	inst, err := shellSetup(home)
	if err != nil {
		return
	}
	changes, err := inst.Clean()
	for _, c := range changes {
		fmt.Printf("  %s\n", describeShellChange(c))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  shell setup clean: %v\n", err)
	}
}

// describeShellChange renders one change as "wrote ~/.zshenv [path]".
func describeShellChange(c shellsetup.Change) string {
	line := c.Action + " " + fsutil.ContractHome(c.Path)
	if c.Section != "" {
		line += " [" + c.Section + "]"
	}
	return line
}
