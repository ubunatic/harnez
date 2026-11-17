package claude

import (
	"fmt"
	"os"
	"strings"

	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/markdown"
)

// applyShellIntegration installs launchers, gear links and the environment script,
// then optionally injects that script into existing shell startup files.
func applyShellIntegration(r *applyReport, target string, cfg *Config, shellOpt bool) error {
	if home, err := os.UserHomeDir(); err == nil {
		shimPath, changed, err := EnsureBashShim(home)
		if err != nil {
			return err
		}
		if changed {
			r.changes++
			fmt.Printf("  wrote %s\n", shimPath)
		} else {
			r.addStat("bash shim", fsutil.ContractHome(shimPath))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		launcherPath, changed, err := EnsureHarnezAgyLauncher(home)
		if err != nil {
			return err
		}
		if changed {
			r.changes++
			fmt.Printf("  wrote %s\n", launcherPath)
		} else {
			r.addStat("harnez-agy launcher", fsutil.ContractHome(launcherPath))
		}
	}

	gearExe := gearExecutable()
	var gearStats []string
	for _, link := range GearSymlinkTargets(target, cfg) {
		lr, err := ensureSymlink(link, gearExe)
		if err != nil {
			return fmt.Errorf("gear symlink %s: %w", link, err)
		}
		if lr.changed {
			r.changes++
			fmt.Printf("  symlink %s → %s\n", link, gearExe)
		} else {
			gearStats = append(gearStats, fsutil.ContractHome(link))
		}
	}
	if len(gearStats) > 0 {
		r.addStat("⚙ symlink", strings.Join(gearStats, ", "))
	}

	targets := shellEnvTargets()
	for _, st := range targets {
		if st.envPath != "" {
			changed, err := fsutil.WriteIfChanged(st.envPath, []byte(st.content))
			if err != nil {
				return fmt.Errorf("harnez env %s: %w", st.envPath, err)
			}
			if changed {
				r.changes++
				fmt.Printf("  wrote %s\n", st.envPath)
			} else {
				r.addStat("env script", fsutil.ContractHome(st.envPath))
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		if err := applyShellSetup(r, home); err != nil {
			return err
		}
	}

	if shellOpt {
		for _, st := range targets {
			for _, rcPath := range st.rcPaths {
				changed, existed, err := markdown.ApplyMK(rcPath, "env", st.rcSnippet)
				if err != nil {
					return fmt.Errorf("shell rc %s: %w", rcPath, err)
				}
				if changed {
					r.changes++
					if existed {
						fmt.Printf("  updated %s [env]\n", rcPath)
					} else {
						fmt.Printf("  wrote %s [env]\n", rcPath)
					}
				} else {
					r.addStat("shell rc", fsutil.ContractHome(rcPath))
				}
			}
		}
	}
	return nil
}
