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

	if envPath := HarnezEnvPath(); envPath != "" {
		changed, err := fsutil.WriteIfChanged(envPath, []byte(HarnezEnvContent))
		if err != nil {
			return fmt.Errorf("harnez env %s: %w", envPath, err)
		}
		if changed {
			r.changes++
			fmt.Printf("  wrote %s\n", envPath)
		} else {
			r.addStat("env script", fsutil.ContractHome(envPath))
		}
	}

	if shellOpt {
		for _, rcPath := range ShellRCPaths() {
			changed, existed, err := markdown.ApplyMK(rcPath, "env", HarnezShellRCSnippet)
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
	return nil
}
