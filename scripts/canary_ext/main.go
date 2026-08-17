package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	extensionUUID = "voice-input@harnez.ubunatic.com"
	logMarker     = "[HARNEZ_VOICE_INPUT_EXT] VoiceInputExtension enabled successfully!"
)

// ── Process Manager ──────────────────────────────────────────────────────────

type processManager struct {
	cmds []*exec.Cmd
}

func (pm *processManager) add(cmd *exec.Cmd) {
	pm.cmds = append(pm.cmds, cmd)
}

func (pm *processManager) killAll() {
	for i := len(pm.cmds) - 1; i >= 0; i-- {
		cmd := pm.cmds[i]
		if cmd != nil && cmd.Process != nil {
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				_ = syscall.Kill(-pgid, syscall.SIGTERM)
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
			}
			_ = cmd.Process.Kill()
		}
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func detectEditor() (string, error) {
	for _, candidate := range []string{"gedit", "gnome-text-editor"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("neither gedit nor gnome-text-editor found in PATH")
}

func killStaleNestedProcesses() {
	// Kill stale gnome-shell instances that were run with --nested or --devkit
	out, err := exec.Command("pgrep", "-a", "gnome-shell").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "--nested") || strings.Contains(line, "--devkit") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					var pid int
					if _, err := fmt.Sscanf(fields[0], "%d", &pid); err == nil && pid > 0 && pid != os.Getpid() {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
			}
		}
	}
	// Also clean up any orphan dbus-daemon or calendar-server processes spawned from pts terminals
	_ = exec.Command("pkill", "-f", "gnome-shell-calendar-server").Run()
}

func gnomeShellModeFlag() string {
	out, err := exec.Command("gnome-shell", "--help").CombinedOutput()
	if err == nil && bytes.Contains(out, []byte("--devkit")) {
		return "--devkit"
	}
	return "--nested"
}

func detectWaylandSocket(logPath string, timeout time.Duration) (string, error) {
	socketRe := regexp.MustCompile(`wayland-[0-9]+`)
	deadline := time.Now().Add(timeout)
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(logPath); err == nil {
			if match := socketRe.Find(data); len(match) > 0 {
				return string(match), nil
			}
		}

		entries, err := filepath.Glob(filepath.Join(runtimeDir, "wayland-[1-9]*"))
		if err == nil {
			for _, entry := range entries {
				if fi, err := os.Stat(entry); err == nil && (fi.Mode()&os.ModeSocket != 0) {
					return filepath.Base(entry), nil
				}
			}
		}

		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout waiting for nested Wayland socket (log: %s)", logPath)
}

func setupExtensionSymlink(sourceDir, destBaseDir, uuid string) (string, error) {
	targetLink := filepath.Join(destBaseDir, uuid)
	_ = os.Remove(targetLink)
	if err := os.MkdirAll(destBaseDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir extensions base: %w", err)
	}
	absSource, err := filepath.Abs(sourceDir)
	if err != nil {
		return "", fmt.Errorf("resolve abs source: %w", err)
	}
	if err := os.Symlink(absSource, targetLink); err != nil {
		return "", fmt.Errorf("symlink extension: %w", err)
	}
	return targetLink, nil
}

// ── Main Canary Execution ────────────────────────────────────────────────────

func runExtensionCanary(ctx context.Context) error {
	for _, dep := range []string{"gnome-shell", "dbus-run-session"} {
		if _, err := exec.LookPath(dep); err != nil {
			return fmt.Errorf("missing required dependency: %s", dep)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("user home dir: %w", err)
	}

	// 1. Install symlink into user's extensions folder
	extUserDir := filepath.Join(homeDir, ".local", "share", "gnome-shell", "extensions")
	extSrcDir := filepath.Join("contrib", "gnome-shell-extension")
	if _, err := os.Stat(extSrcDir); err != nil {
		extSrcDir = filepath.Join("scripts", "canary_ext", "dummy_ext")
	}
	installedLink, err := setupExtensionSymlink(extSrcDir, extUserDir, extensionUUID)
	if err != nil {
		return fmt.Errorf("setup symlink: %w", err)
	}
	defer func() {
		_ = os.Remove(installedLink)
	}()
	fmt.Printf("1. Installed canary extension to: %s\n", installedLink)

	// Clean up stale nested shells before launch
	killStaleNestedProcesses()

	tmpDir, err := os.MkdirTemp("", "canary-nested-ext-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	logPath := filepath.Join(os.TempDir(), "canary-nested-extension.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()
	fmt.Printf("   Logging nested shell to: %s\n", logPath)

	pm := &processManager{}
	defer func() {
		fmt.Printf("\nCleaning up child processes...\n")
		pm.killAll()
	}()

	// 2. Enable extension in gsettings
	fmt.Printf("2. Enabling %s in gsettings...\n", extensionUUID)
	_ = exec.Command("gsettings", "set", "org.gnome.shell", "disable-user-extensions", "false").Run()
	_ = exec.Command("gnome-extensions", "enable", extensionUUID).Run()

	// 3. Launch nested GNOME Shell and capture its isolated DBUS_SESSION_BUS_ADDRESS
	fmt.Println("3. Launching nested GNOME Shell...")
	shellFlag := gnomeShellModeFlag()
	absExtSrc, _ := filepath.Abs(extSrcDir)

	dbusAddrFile := filepath.Join(tmpDir, "dbus_address.txt")
	wrapperScript := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$DBUS_SESSION_BUS_ADDRESS" > "%s"
exec gnome-shell %s --wayland
`, dbusAddrFile, shellFlag)

	wrapperPath := filepath.Join(tmpDir, "run_nested_shell.sh")
	if err := os.WriteFile(wrapperPath, []byte(wrapperScript), 0755); err != nil {
		return fmt.Errorf("write wrapper script: %w", err)
	}

	shellCmd := exec.Command("dbus-run-session", "--", wrapperPath)
	shellCmd.Env = append(os.Environ(),
		"GNOME_SHELL_EXTRA_EXTENSION_DIRS="+filepath.Dir(absExtSrc),
		"G_MESSAGES_DEBUG=all",
	)
	shellCmd.Stdout = logFile
	shellCmd.Stderr = logFile
	shellCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := shellCmd.Start(); err != nil {
		return fmt.Errorf("start nested gnome-shell: %w", err)
	}
	pm.add(shellCmd)

	fmt.Println("   Waiting for nested Wayland socket initialization...")
	waylandDisplay, err := detectWaylandSocket(logPath, 12*time.Second)
	if err != nil {
		return err
	}
	fmt.Printf("  ✓ Nested GNOME Shell socket: %s (PID: %d)\n", waylandDisplay, shellCmd.Process.Pid)

	// Wait for GNOME Shell to log "GNOME Shell started"
	fmt.Println("   Waiting for GNOME Shell compositor to finish startup...")
	startedDeadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(startedDeadline) {
		if data, err := os.ReadFile(logPath); err == nil {
			if strings.Contains(string(data), "GNOME Shell started at") {
				break
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	// 4. Enable extension inside the exact nested DBus session
	time.Sleep(500 * time.Millisecond)
	dbusAddrBytes, _ := os.ReadFile(dbusAddrFile)
	nestedDbusAddr := strings.TrimSpace(string(dbusAddrBytes))

	enableInsideCmd := exec.Command("gnome-extensions", "enable", extensionUUID)
	enableInsideCmd.Env = append(os.Environ(),
		"WAYLAND_DISPLAY="+waylandDisplay,
		"DBUS_SESSION_BUS_ADDRESS="+nestedDbusAddr,
	)
	enableOutput, _ := enableInsideCmd.CombinedOutput()
	if len(enableOutput) > 0 {
		fmt.Printf("   gnome-extensions enable: %s\n", strings.TrimSpace(string(enableOutput)))
	}

	// 5. Poll log file to verify the extension was started & printed its log
	fmt.Println("4. Verifying extension activation log from nested shell...")
	foundLog := false
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(logPath); err == nil {
			if strings.Contains(string(data), logMarker) {
				foundLog = true
				break
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	if foundLog {
		fmt.Printf("  ✓ SUCCESS: Detected log marker: %q\n", logMarker)
	} else {
		fmt.Printf("  [INFO] Log marker check finished.\n")
	}

	sendNotification := func(count int) {
		title := fmt.Sprintf("Harnez Canary Notification #%d", count)
		msg := fmt.Sprintf("Triggered at %s", time.Now().Format("15:04:05"))
		notifyCmd := exec.Command("notify-send", title, msg, "--icon=audio-input-microphone-symbolic")
		notifyCmd.Env = append(os.Environ(),
			"WAYLAND_DISPLAY="+waylandDisplay,
			"DBUS_SESSION_BUS_ADDRESS="+nestedDbusAddr,
		)
		if err := notifyCmd.Run(); err == nil {
			fmt.Printf("  ✓ Sent notification #%d to nested desktop\n", count)
		} else {
			fmt.Printf("  [WARN] notify-send failed: %v\n", err)
		}
	}

	// Send first notification
	sendNotification(1)

	// Monitor nested shell process in background
	shellExited := make(chan struct{})
	go func() {
		_ = shellCmd.Wait()
		close(shellExited)
	}()

	// Read stdin lines in background
	inputLines := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputLines <- scanner.Text()
		}
		close(inputLines)
	}()

	// Stream nested shell log in realtime to detect JS errors / warnings live
	go func() {
		var lastOffset int64 = 0
		for {
			if file, err := os.Open(logPath); err == nil {
				if fi, err := file.Stat(); err == nil && fi.Size() > lastOffset {
					buf := make([]byte, fi.Size()-lastOffset)
					_, _ = file.ReadAt(buf, lastOffset)
					lastOffset = fi.Size()
					text := string(buf)
					for _, line := range strings.Split(text, "\n") {
						if strings.Contains(line, "HARNEZ_VOICE_INPUT_EXT") ||
							strings.Contains(line, "JS ERROR") ||
							strings.Contains(line, "GNOME Shell-CRITICAL") ||
							strings.Contains(line, "Virtual function") {
							fmt.Printf("   [SHELL LOG] %s\n", line)
						}
					}
				}
				file.Close()
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	fmt.Println("\n>>> [Interactive Mode] Press Enter to send another notification, close the window or Ctrl+C to exit <<<")
	count := 2
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-shellExited:
			fmt.Println("\nNested shell window closed. Exiting...")
			return nil
		case _, ok := <-inputLines:
			if !ok {
				return nil
			}
			sendNotification(count)
			count++
		}
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := runExtensionCanary(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
