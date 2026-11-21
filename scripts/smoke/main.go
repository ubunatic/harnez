// Command smoke is the live part of the former scripts/smoke-test.sh (issue
// 1254). It checks what only the real machine can show; the hermetic
// apply/diff/repair cycle is TestApplyDiffRepairCycle and the bar alignment is
// TestAllUsageBoxSecondBarColumnAlignment.
//
// Checks, in order:
//   - BINARY apply against the real ~/.claude, then a repeat apply reports
//     "No changes." and leaves settings.json byte-identical.
//   - `claude plugin list` shows harnez-channel enabled, and jev-compaction
//     enabled when installed (files on disk are not proof, issue 747).
//     Skipped without claude.
//   - The harnez on PATH reports the same version as BINARY (issue 1245) and
//     `harnez usage --compact` exits 0 with output. Skipped without harnez.
//
// It never edits settings.json itself, unlike the old drift simulation.
//
// Usage:
//
//	go run ./scripts/smoke [BINARY]
//
// BINARY defaults to ./harnez.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// requiredPlugin must be enabled; the others must be enabled when installed
// (jev compaction is off by default in config.yaml).
const requiredPlugin = "harnez-channel@harnez-local"

var plugins = []string{requiredPlugin, "jev-compaction@harnez-local"}

func main() {
	bin := "./harnez"
	if len(os.Args) > 1 {
		bin = os.Args[1]
	}
	if err := run(bin); err != nil {
		fmt.Fprintf(os.Stderr, "  FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nAll smoke checks passed.")
}

func run(bin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settings := filepath.Join(home, ".claude", "settings.json")

	section("apply")
	if _, err := command(bin, "apply"); err != nil {
		return err
	}
	before, err := os.ReadFile(settings)
	if err != nil {
		return err
	}

	section("apply again: must be idempotent")
	out, err := command(bin, "apply")
	if err != nil {
		return err
	}
	if !strings.Contains(out, "No changes.") {
		return fmt.Errorf("second apply made unexpected changes")
	}
	after, err := os.ReadFile(settings)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("second apply changed %s", settings)
	}
	pass("idempotent, settings.json unchanged")

	section("plugins: Claude Code must load what apply ships")
	if _, err := exec.LookPath("claude"); err != nil {
		skip("claude not found on PATH")
	} else {
		list, err := exec.Command("claude", "plugin", "list").CombinedOutput()
		if err != nil {
			return fmt.Errorf("claude plugin list: %v\n%s", err, list)
		}
		for _, p := range plugins {
			switch pluginStatus(string(list), p) {
			case statusEnabled:
				pass(p + " enabled")
			case statusMissing:
				if p != requiredPlugin {
					skip(p + " not installed (off by default)")
					continue
				}
				return fmt.Errorf("%s missing from 'claude plugin list':\n%s", p, list)
			default:
				return fmt.Errorf("%s installed but not enabled in 'claude plugin list':\n%s", p, list)
			}
		}
	}

	section("installed harnez")
	installed, err := exec.LookPath("harnez")
	if err != nil {
		skip("harnez not found on PATH; run 'make install' first")
		return nil
	}
	want, err := version(bin)
	if err != nil {
		return err
	}
	got, err := version(installed)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("harnez on PATH (%s) is %q, %s is %q; run 'make install'", installed, got, bin, want)
	}
	pass("PATH harnez matches " + bin + ": " + got)
	usage, err := exec.Command(installed, "usage", "--compact").Output()
	if err != nil {
		return fmt.Errorf("harnez usage --compact: %v", err)
	}
	if len(bytes.TrimSpace(usage)) == 0 {
		return fmt.Errorf("harnez usage --compact produced no output")
	}
	pass("harnez usage --compact exited 0 with output")
	return nil
}

type status int

const (
	statusMissing status = iota
	statusDisabled
	statusEnabled
)

// pluginStatus reads the block for plugin, from its name line to the next
// blank line, and reports whether its Status line says enabled. "disabled"
// contains "enabled", so the word must match exactly.
func pluginStatus(list, plugin string) status {
	in := false
	for _, line := range strings.Split(list, "\n") {
		if !in {
			in = strings.Contains(line, plugin)
			continue
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		if _, value, ok := strings.Cut(line, "Status:"); ok {
			for _, word := range strings.Fields(value) {
				if word == "enabled" {
					return statusEnabled
				}
			}
			return statusDisabled
		}
	}
	if in {
		return statusDisabled
	}
	return statusMissing
}

func version(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("%s --version: %v", bin, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func command(bin string, args ...string) (string, error) {
	out, err := exec.Command(bin, args...).CombinedOutput()
	fmt.Print(string(out))
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %v", bin, strings.Join(args, " "), err)
	}
	return string(out), nil
}

func section(name string) { fmt.Printf("\n=== %s ===\n", name) }
func pass(msg string)     { fmt.Printf("  PASS: %s\n", msg) }
func skip(msg string)     { fmt.Printf("  SKIP: %s\n", msg) }
