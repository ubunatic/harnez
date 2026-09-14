// Command macos-podman manages containerized macOS KVM instances via Podman.
//
// Usage:
//
//	go run ./scripts/macos-podman [run|stop|status|boot-status|screenshot|clean] [flags]
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
)

type config struct {
	name        string
	version     string
	httpPort    int
	vncPort     int
	sshPort     int
	cpuCores    int
	ramSize     string
	diskSize    string
	storageDir  string
	sharedDir   string
	noShared    bool
	autoRm      bool
	stopTimeout int
	noCrop      bool
	short       bool
	remoteHost  string
}

var cfg = config{
	name:        "macos-kvm",
	version:     "11",
	httpPort:    8006,
	vncPort:     5900,
	sshPort:     2222,
	cpuCores:    1,
	ramSize:     "4G",
	diskSize:    "64G",
	storageDir:  "./macos-storage",
	sharedDir:   ".",
	autoRm:      false,
	stopTimeout: 10,
	noCrop:      false,
	short:       false,
	remoteHost:  "",
}

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "macos-podman [command]",
		Short: "Manage containerized macOS KVM instances via Podman",
		Long: `Manage containerized macOS KVM instances via Podman.

Default action (without subcommands) starts the macOS container.

Endpoints:
  Web UI (noVNC): http://localhost:8006
  VNC:            localhost:5900
  SSH:            ssh -p 2222 localhost`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return doRun(cmd.Context(), cfg)
		},
	}

	// Persistent flags
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfg.name, "name", cfg.name, "Container name")
	pf.StringVar(&cfg.version, "version", cfg.version, "macOS version (11=BigSur, 12=Monterey, 13=Ventura, 14=Sonoma, 15=Sequoia)")
	pf.IntVar(&cfg.httpPort, "port", cfg.httpPort, "Web noVNC HTTP port")
	pf.IntVar(&cfg.vncPort, "vnc-port", cfg.vncPort, "VNC port")
	pf.IntVar(&cfg.sshPort, "ssh-port", cfg.sshPort, "SSH port")
	pf.IntVar(&cfg.cpuCores, "cpu", cfg.cpuCores, "CPU cores (default 1 for AMD Ryzen compatibility)")
	pf.StringVar(&cfg.ramSize, "ram", cfg.ramSize, "RAM size")
	pf.StringVar(&cfg.diskSize, "disk", cfg.diskSize, "Disk size")
	pf.StringVar(&cfg.storageDir, "storage", cfg.storageDir, "Storage path on host")
	pf.StringVar(&cfg.sharedDir, "shared", cfg.sharedDir, "Host directory to mount at /shared")
	pf.BoolVar(&cfg.noShared, "no-shared", false, "Disable mounting host directory")
	pf.BoolVar(&cfg.autoRm, "rm", false, "Remove container on exit (ephemeral mode)")
	pf.IntVar(&cfg.stopTimeout, "timeout", cfg.stopTimeout, "Stop timeout in seconds before force kill")
	pf.BoolVar(&cfg.noCrop, "no-crop", false, "Disable auto-cropping of black borders from screenshots")
	pf.BoolVarP(&cfg.short, "short", "s", false, "Compact single-line output summary")
	pf.StringVarP(&cfg.remoteHost, "host", "H", "", "Remote SSH host for podman execution (e.g. x600)")

	// Subcommands
	rootCmd.AddCommand(
		newRunCommand(),
		newStopCommand(),
		newStatusCommand(),
		newBootStatusCommand(),
		newInspectCommand(),
		newScreenshotCommand(),
		newTypeCommand(),
		newSendKeyCommand(),
		newInstallCommand(),
		newSnapshotCommand(),
		newSyncCommand(),
		newCleanCommand(),
	)

	return rootCmd
}

func newRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start macOS container",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doRun(cmd.Context(), cfg)
		},
	}
}

func newStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop running macOS container (bounded timeout)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doStop(cmd.Context(), cfg)
		},
	}
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show basic status, endpoints, and quick HTTP check",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doStatus(cmd.Context(), cfg)
		},
	}
}

func newBootStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "boot-status",
		Aliases: []string{"boot"},
		Short:   "Comprehensive boot diagnosis, guest stages, logs & screenshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doBootStatus(cmd.Context(), cfg)
		},
	}
}

func newInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "inspect",
		Aliases: []string{"registers", "cpu", "mem", "debug"},
		Short:   "Deep inspection of guest CPU registers, instruction pointer (RIP), stack & memory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doInspect(cmd.Context(), cfg)
		},
	}
}

func newScreenshotCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "screenshot [output_path.png]",
		Short: "Capture current guest screen to PNG",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outPath := "/tmp/macos_screen.png"
			if len(args) > 0 {
				outPath = args[0]
			}
			return doScreenshot(cmd.Context(), cfg, outPath)
		},
	}
}

func newCleanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Stop and remove container and storage directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doClean(cmd.Context(), cfg)
		},
	}
}

func newTypeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "type <text>",
		Short: "Send keystrokes directly to macOS guest via QEMU monitor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doSendText(cmd.Context(), cfg, args[0])
		},
	}
}

func newSendKeyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "send-key <key>",
		Short: "Send a special key (ret, spc, tab, ctrl-c, etc.) to macOS guest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doSendKey(cmd.Context(), cfg, args[0])
		},
	}
}

func newInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install-os",
		Short: "Automate macOS disk formatting and trigger OS installation in Recovery",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doAutomatedInstall(cmd.Context(), cfg)
		},
	}
}

func newSnapshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "snapshot",
		Aliases: []string{"snap"},
		Short:   "Manage persistent storage snapshots (save, list, restore, rm)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "save [name]",
		Short: "Create a named snapshot of current macOS storage",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return doSnapshotSave(cmd.Context(), cfg, name)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all existing snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doSnapshotList(cmd.Context(), cfg)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "restore <name>",
		Short: "Restore macOS storage from a named snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doSnapshotRestore(cmd.Context(), cfg, args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"delete"},
		Short:   "Delete a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doSnapshotDelete(cmd.Context(), cfg, args[0])
		},
	})

	return cmd
}

func runHostShell(ctx context.Context, remoteHost, script string) (string, error) {
	var cmd *exec.Cmd
	if remoteHost != "" {
		cmd = exec.CommandContext(ctx, "ssh", remoteHost, script)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", script)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func resolveSnapshotPath(snapDir, version, name string) string {
	if strings.HasPrefix(name, version+"-") {
		return fmt.Sprintf("%s/%s", snapDir, name)
	}
	return fmt.Sprintf("%s/%s-%s", snapDir, version, name)
}

func doSnapshotSave(ctx context.Context, cfg config, name string) error {
	if name == "" {
		name = time.Now().Format("20060102-150405")
	}
	_, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if running {
		fmt.Printf("Notice: container %q is running on %s. Taking snapshot of current disk state.\n", cfg.name, hostName(cfg.remoteHost))
	}
	snapDir := fmt.Sprintf("%s/.snapshots", cfg.storageDir)
	snapTarget := resolveSnapshotPath(snapDir, cfg.version, name)
	srcDir := fmt.Sprintf("%s/%s", cfg.storageDir, cfg.version)

	fmt.Printf("Saving snapshot %q on %s (source: %s)...\n", name, hostName(cfg.remoteHost), srcDir)
	script := fmt.Sprintf("mkdir -p %s && rm -rf %s && cp -r --sparse=always %s %s", shellEscape(snapDir), shellEscape(snapTarget), shellEscape(srcDir), shellEscape(snapTarget))
	out, err := runHostShell(ctx, cfg.remoteHost, script)
	if err != nil {
		return fmt.Errorf("snapshot save failed: %s: %w", strings.TrimSpace(out), err)
	}

	sizeOut, _ := runHostShell(ctx, cfg.remoteHost, fmt.Sprintf("du -sh %s 2>/dev/null", shellEscape(snapTarget)))
	size := strings.Fields(strings.TrimSpace(sizeOut))
	sizeStr := "unknown"
	if len(size) > 0 {
		sizeStr = size[0]
	}
	fmt.Printf("Snapshot %q saved successfully.\n  Path: %s\n  Size: %s\n", name, snapTarget, sizeStr)
	return nil
}

func doSnapshotList(ctx context.Context, cfg config) error {
	snapDir := fmt.Sprintf("%s/.snapshots", cfg.storageDir)
	script := fmt.Sprintf("if [ -d %s ]; then du -sh %s/* 2>/dev/null; fi", shellEscape(snapDir), shellEscape(snapDir))
	out, err := runHostShell(ctx, cfg.remoteHost, script)
	if err != nil {
		return fmt.Errorf("list snapshots failed: %s: %w", strings.TrimSpace(out), err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		fmt.Printf("No snapshots found in %s on %s.\n", snapDir, hostName(cfg.remoteHost))
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	fmt.Printf("Snapshots in %s on %s:\n", snapDir, hostName(cfg.remoteHost))
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			snapName := filepath.Base(parts[1])
			fmt.Printf("  - %-32s (%s)\n", snapName, parts[0])
		}
	}
	return nil
}

func doSnapshotRestore(ctx context.Context, cfg config, name string) error {
	_, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if running {
		return fmt.Errorf("container %q is running on %s. Stop container before restoring: 'go run ./scripts/macos-podman stop'", cfg.name, hostName(cfg.remoteHost))
	}
	snapDir := fmt.Sprintf("%s/.snapshots", cfg.storageDir)
	snapTarget := resolveSnapshotPath(snapDir, cfg.version, name)
	destDir := fmt.Sprintf("%s/%s", cfg.storageDir, cfg.version)

	fmt.Printf("Restoring snapshot %q on %s to %s...\n", name, hostName(cfg.remoteHost), destDir)
	script := fmt.Sprintf("if [ ! -d %s ]; then if [ -d %s/%s ]; then snap=%s/%s; else echo 'Snapshot not found'; exit 1; fi; else snap=%s; fi; rm -rf %s && cp -r --sparse=always \"$snap\" %s",
		shellEscape(snapTarget),
		shellEscape(snapDir), shellEscape(name),
		shellEscape(snapDir), shellEscape(name),
		shellEscape(snapTarget),
		shellEscape(destDir),
		shellEscape(destDir),
	)
	out, err := runHostShell(ctx, cfg.remoteHost, script)
	if err != nil {
		return fmt.Errorf("snapshot restore failed: %s: %w", strings.TrimSpace(out), err)
	}
	fmt.Printf("Snapshot %q restored successfully to %s.\n", name, destDir)
	return nil
}

func doSnapshotDelete(ctx context.Context, cfg config, name string) error {
	snapDir := fmt.Sprintf("%s/.snapshots", cfg.storageDir)
	snapTarget := resolveSnapshotPath(snapDir, cfg.version, name)
	script := fmt.Sprintf("rm -rf %s %s/%s", shellEscape(snapTarget), shellEscape(snapDir), shellEscape(name))
	out, err := runHostShell(ctx, cfg.remoteHost, script)
	if err != nil {
		return fmt.Errorf("snapshot delete failed: %s: %w", strings.TrimSpace(out), err)
	}
	fmt.Printf("Snapshot %q deleted from %s.\n", name, hostName(cfg.remoteHost))
	return nil
}

func newSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync [remote_host]",
		Short: "Sync local persistent macOS storage to remote host using sparse rsync",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			destHost := cfg.remoteHost
			if len(args) > 0 {
				destHost = args[0]
			}
			if destHost == "" {
				return fmt.Errorf("specify remote host via argument or --host/-H flag (e.g. macos-podman sync x600)")
			}
			return doSyncStorage(cmd.Context(), cfg, destHost)
		},
	}
}

func doSyncStorage(ctx context.Context, cfg config, remoteHost string) error {
	fmt.Printf("Syncing %s to %s:%s (sparse mode)...\n", cfg.storageDir, remoteHost, cfg.storageDir)
	mkdirCmd := exec.CommandContext(ctx, "ssh", remoteHost, fmt.Sprintf("mkdir -p %s", cfg.storageDir))
	_ = mkdirCmd.Run()
	cmd := exec.CommandContext(ctx, "rsync", "-avz", "--sparse", "--progress", cfg.storageDir+"/", fmt.Sprintf("%s:%s/", remoteHost, cfg.storageDir))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func doSendKey(ctx context.Context, cfg config, key string) error {
	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if !exists || !running {
		return fmt.Errorf("container %q is not running", cfg.name)
	}
	_, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, fmt.Sprintf("sendkey %s", key))
	return err
}

func doSendText(ctx context.Context, cfg config, text string) error {
	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if !exists || !running {
		return fmt.Errorf("container %q is not running", cfg.name)
	}

	keymap := map[rune]string{
		' ':  "spc",
		'\n': "ret",
		'-':  "minus",
		'_':  "shift-minus",
		'/':  "slash",
		'.':  "dot",
		':':  "shift-semicolon",
		';':  "semicolon",
		'=':  "equal",
		'"':  "shift-apostrophe",
		'\'': "apostrophe",
		'\\': "backslash",
		'+':  "shift-equal",
		'$':  "shift-4",
		'%':  "shift-5",
		'&':  "shift-7",
		'*':  "shift-8",
		'(':  "shift-9",
		')':  "shift-0",
		'!':  "shift-1",
		'@':  "shift-2",
		'#':  "shift-3",
	}

	for _, r := range text {
		var k string
		if val, ok := keymap[r]; ok {
			k = val
		} else if unicode.IsUpper(r) {
			k = fmt.Sprintf("shift-%c", unicode.ToLower(r))
		} else {
			k = string(r)
		}
		if _, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, fmt.Sprintf("sendkey %s", k)); err != nil {
			return fmt.Errorf("sendkey %s: %w", k, err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	return nil
}

func doAutomatedInstall(ctx context.Context, cfg config) error {
	stage := assessBootStage(ctx, cfg.remoteHost, cfg.name)
	if !strings.Contains(stage, "Stage 4/4") && !strings.Contains(stage, "Language Chooser") && !strings.Contains(stage, "Recovery") {
		return fmt.Errorf("guest is not in Stage 4/4 Recovery mode (current: %s)", stage)
	}

	fmt.Println("Triggering automated disk initialization and installation...")
	fmt.Println("1. Sending 'ret' to confirm Language Chooser (if pending)...")
	_ = doSendKey(ctx, cfg, "ret")
	time.Sleep(1 * time.Second)

	fmt.Println("2. Formatting target virtual disk (Macintosh HD, APFS, GPT)...")
	if err := doSendText(ctx, cfg, "diskutil eraseDisk APFS \"Macintosh HD\" GPT /dev/disk0\n"); err != nil {
		return fmt.Errorf("send diskutil command: %w", err)
	}

	fmt.Println("\nDisk erase command sent to Recovery Terminal.")
	fmt.Println("Monitor progress via:")
	fmt.Println("  go run ./scripts/macos-podman screenshot")
	fmt.Println("  go run ./scripts/macos-podman boot-status")
	return nil
}

func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '=' || r == ',') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func podmanCmd(ctx context.Context, remoteHost string, args ...string) *exec.Cmd {
	if remoteHost != "" {
		escaped := make([]string, len(args))
		for i, a := range args {
			escaped[i] = shellEscape(a)
		}
		sshArg := "podman " + strings.Join(escaped, " ")
		return exec.CommandContext(ctx, "ssh", remoteHost, sshArg)
	}
	return exec.CommandContext(ctx, "podman", args...)
}

func checkPrerequisites(remoteHost string) error {
	if remoteHost != "" {
		cmd := exec.Command("ssh", remoteHost, "podman --version")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("remote host %q: podman is not reachable via SSH (%w)", remoteHost, err)
		}
		kvmCmd := exec.Command("ssh", remoteHost, "test -w /dev/kvm")
		if err := kvmCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: /dev/kvm is not writeable on %s — add user to kvm group: 'sudo usermod -aG kvm $USER'\n", remoteHost)
		}
		return nil
	}

	if _, err := exec.LookPath("podman"); err != nil {
		return fmt.Errorf("podman is not installed or not in PATH")
	}
	if runtime.GOOS == "linux" {
		kvmCmd := exec.Command("test", "-w", "/dev/kvm")
		if err := kvmCmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: /dev/kvm is not writeable — add user to kvm group: 'sudo usermod -aG kvm $USER'")
		}
	}
	return nil
}

func isAMDCPU(remoteHost string) bool {
	var data []byte
	var err error
	if remoteHost != "" {
		cmd := exec.Command("ssh", remoteHost, "cat /proc/cpuinfo")
		data, err = cmd.Output()
	} else {
		data, err = os.ReadFile("/proc/cpuinfo")
	}
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "AuthenticAMD")
}

func containerRunning(ctx context.Context, remoteHost, name string) (exists bool, running bool) {
	cmd := podmanCmd(ctx, remoteHost, "inspect", "-f", "{{.State.Running}}", name)
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	return true, strings.TrimSpace(string(out)) == "true"
}

func hostName(remoteHost string) string {
	if remoteHost != "" {
		return remoteHost
	}
	return "localhost"
}

func doRun(ctx context.Context, cfg config) error {
	if err := checkPrerequisites(cfg.remoteHost); err != nil {
		return err
	}

	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if exists && running {
		fmt.Printf("Container %q is already running on %s.\n", cfg.name, hostName(cfg.remoteHost))
		printEndpoints(cfg)
		return nil
	}

	if exists && !running {
		fmt.Printf("Starting existing stopped container %q on %s...\n", cfg.name, hostName(cfg.remoteHost))
		startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		cmd := podmanCmd(startCtx, cfg.remoteHost, "start", cfg.name)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to start container: %s: %w", string(out), err)
		}
		configureNoVNC(ctx, cfg.remoteHost, cfg.name)
		printEndpoints(cfg)
		return nil
	}

	if cfg.remoteHost == "" {
		if err := os.MkdirAll(cfg.storageDir, 0o755); err != nil {
			return fmt.Errorf("create storage dir: %w", err)
		}
	}

	runArgs := []string{
		"run", "-d",
		"--name", cfg.name,
		"-p", fmt.Sprintf("%d:8006", cfg.httpPort),
		"-p", fmt.Sprintf("%d:5900", cfg.vncPort),
		"-p", fmt.Sprintf("%d:22", cfg.sshPort),
		"-e", fmt.Sprintf("VERSION=%s", cfg.version),
		"-e", fmt.Sprintf("CPU_CORES=%d", cfg.cpuCores),
		"-e", fmt.Sprintf("RAM_SIZE=%s", cfg.ramSize),
		"-e", fmt.Sprintf("DISK_SIZE=%s", cfg.diskSize),
		"-v", fmt.Sprintf("%s:/storage:Z", cfg.storageDir),
		"--stop-timeout", fmt.Sprintf("%d", cfg.stopTimeout),
	}

	if isAMDCPU(cfg.remoteHost) {
		fmt.Printf("AMD CPU detected on %s: applying Intel CPUID spoofing and disabling 64-bit PCI hole...\n", hostName(cfg.remoteHost))
		runArgs = append(runArgs, "-e", "CPU_MODEL=Haswell-noTSX,stepping=3", "-e", "ARGS=-global q35-pcihost.pci-hole64-size=0")
	}

	if !cfg.noShared && cfg.sharedDir != "" && cfg.remoteHost == "" {
		if abs, err := filepath.Abs(cfg.sharedDir); err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				runArgs = append(runArgs, "-v", fmt.Sprintf("%s:/shared:Z", abs))
			}
		}
	}

	runArgs = append(runArgs, "--group-add", "keep-groups", "--device", "/dev/kvm", "--device", "/dev/net/tun", "--cap-add", "NET_ADMIN")
	if cfg.autoRm {
		runArgs = append(runArgs, "--rm")
	}

	runArgs = append(runArgs, "docker.io/dockurr/macos:latest")

	fmt.Printf("Launching macOS %s in Podman on %s (container: %s)...\n", cfg.version, hostName(cfg.remoteHost), cfg.name)
	launchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	cmd := podmanCmd(launchCtx, cfg.remoteHost, runArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman run failed: %s: %w", string(out), err)
	}

	fmt.Printf("Container %q started successfully on %s.\n", cfg.name, hostName(cfg.remoteHost))
	configureNoVNC(ctx, cfg.remoteHost, cfg.name)
	printEndpoints(cfg)
	return nil
}

func configureNoVNC(ctx context.Context, remoteHost, name string) {
	patchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := podmanCmd(patchCtx, remoteHost, "exec", name, "sed", "-i", "s/UI.initSetting('show_dot', false);/UI.initSetting('show_dot', true);/g", "/usr/share/novnc/app/ui.js")
	_ = cmd.Run()
}

func doStop(ctx context.Context, cfg config) error {
	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if !exists {
		fmt.Printf("Container %q does not exist on %s.\n", cfg.name, hostName(cfg.remoteHost))
		return nil
	}
	if !running {
		fmt.Printf("Container %q is already stopped on %s.\n", cfg.name, hostName(cfg.remoteHost))
		return nil
	}

	fmt.Printf("Stopping container %q on %s (timeout: %ds)...\n", cfg.name, hostName(cfg.remoteHost), cfg.stopTimeout)
	stopCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.stopTimeout+5)*time.Second)
	defer cancel()

	cmd := podmanCmd(stopCtx, cfg.remoteHost, "stop", cfg.name)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Graceful stop failed (%v). Force killing container %q on %s...\n", err, cfg.name, hostName(cfg.remoteHost))
		_ = podmanCmd(ctx, cfg.remoteHost, "kill", cfg.name).Run()
	} else {
		fmt.Printf("Container %q stopped: %s\n", cfg.name, strings.TrimSpace(string(out)))
	}
	return nil
}

func doStatus(ctx context.Context, cfg config) error {
	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if cfg.short {
		if !running {
			if exists {
				fmt.Println("Status: STOPPED (container exists)")
			} else {
				fmt.Println("Status: NOT_FOUND")
			}
			return nil
		}
		webStatus := "down"
		client := http.Client{Timeout: 500 * time.Millisecond}
		if resp, err := client.Get(fmt.Sprintf("http://localhost:%d", cfg.httpPort)); err == nil {
			resp.Body.Close()
			webStatus = resp.Status
		}
		fmt.Printf("[RUNNING] Container: %s | Web: %s | Ports: web=%d vnc=%d ssh=%d\n",
			cfg.name, webStatus, cfg.httpPort, cfg.vncPort, cfg.sshPort)
		return nil
	}

	fmt.Printf("Container: %s\n", cfg.name)
	fmt.Printf("  Exists:  %v\n", exists)
	fmt.Printf("  Running: %v\n", running)

	if !running {
		return nil
	}

	printEndpoints(cfg)

	// Quick HTTP readiness probe
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d", hostName(cfg.remoteHost), cfg.httpPort))
	if err == nil {
		defer resp.Body.Close()
		fmt.Printf("  Web UI:  HTTP %s (Ready)\n", resp.Status)
	} else {
		fmt.Printf("  Web UI:  Connecting... (%v)\n", err)
	}

	// Tail recent logs
	logCmd := podmanCmd(ctx, cfg.remoteHost, "logs", "--tail", "5", cfg.name)
	if out, err := logCmd.Output(); err == nil && len(bytes.TrimSpace(out)) > 0 {
		fmt.Println("\nRecent Logs:")
		fmt.Println(string(out))
	}
	return nil
}

func doClean(ctx context.Context, cfg config) error {
	fmt.Printf("Cleaning up container %q on %s...\n", cfg.name, hostName(cfg.remoteHost))
	_ = podmanCmd(ctx, cfg.remoteHost, "rm", "-f", cfg.name).Run()

	if cfg.remoteHost != "" {
		fmt.Printf("Removing storage directory %s on %s...\n", cfg.storageDir, cfg.remoteHost)
		_ = exec.CommandContext(ctx, "ssh", cfg.remoteHost, fmt.Sprintf("rm -rf %s", cfg.storageDir)).Run()
	} else if _, err := os.Stat(cfg.storageDir); err == nil {
		fmt.Printf("Removing storage directory %s...\n", cfg.storageDir)
		if err := os.RemoveAll(cfg.storageDir); err != nil {
			return fmt.Errorf("remove storage dir: %w", err)
		}
	}
	fmt.Println("Cleanup complete.")
	return nil
}

func printEndpoints(cfg config) {
	h := hostName(cfg.remoteHost)
	fmt.Printf("  Web UI:  http://%s:%d\n", h, cfg.httpPort)
	fmt.Printf("  VNC:     %s:%d\n", h, cfg.vncPort)
	fmt.Printf("  SSH:     ssh -p %d %s\n", cfg.sshPort, h)
	if !cfg.noShared && cfg.sharedDir != "" && cfg.remoteHost == "" {
		if abs, err := filepath.Abs(cfg.sharedDir); err == nil {
			fmt.Printf("  Shared:  %s -> /shared\n", abs)
		}
	}
	if cfg.autoRm {
		fmt.Println("  Mode:    Ephemeral (--rm)")
	} else {
		fmt.Printf("  Storage: %s (persistent)\n", cfg.storageDir)
	}
}

func doBootStatus(ctx context.Context, cfg config) error {
	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if !running {
		if cfg.short {
			if exists {
				fmt.Println("Status: STOPPED (container exists)")
			} else {
				fmt.Println("Status: NOT_FOUND")
			}
			return nil
		}
		fmt.Printf("Container: %s on %s (version: macOS %s)\n", cfg.name, hostName(cfg.remoteHost), cfg.version)
		fmt.Printf("  State:    exists=%v running=%v\n", exists, running)
		fmt.Println("  Status:   Guest is not running. Run 'go run ./scripts/macos-podman run' to start.")
		return nil
	}

	stage := assessBootStage(ctx, cfg.remoteHost, cfg.name)
	h := hostName(cfg.remoteHost)

	// 1. Web UI probe
	client := http.Client{Timeout: 1 * time.Second}
	webStatus := "Down"
	if resp, err := client.Get(fmt.Sprintf("http://%s:%d", h, cfg.httpPort)); err == nil {
		resp.Body.Close()
		webStatus = fmt.Sprintf("HTTP %s (Active)", resp.Status)
	}

	// 2. VNC probe
	vncStatus := "Unreachable"
	if banner, err := probeTCPBanner(h, cfg.vncPort, 1*time.Second); err == nil {
		vncStatus = fmt.Sprintf("Active (%s)", strings.TrimSpace(banner))
	}

	// 3. SSH probe
	sshStatus := "Waiting for guest daemon"
	if banner, err := probeTCPBanner(h, cfg.sshPort, 1*time.Second); err == nil {
		if strings.Contains(banner, "SSH") {
			sshStatus = fmt.Sprintf("Ready (%s)", strings.TrimSpace(banner))
		} else {
			sshStatus = "Port open (starting up)"
		}
	}

	// 4. Screenshot
	screenshotPath := "/tmp/macos_screen.png"
	w, hpx, err := captureGuestScreen(ctx, cfg.remoteHost, cfg.name, screenshotPath, !cfg.noCrop)
	screenInfo := "Capture failed"
	if err == nil {
		screenInfo = fmt.Sprintf("%dx%d", w, hpx)
	}

	if cfg.short {
		fmt.Printf("[RUNNING on %s] %s | Web: %s | VNC: %s | SSH: %s | Screen: %s -> %s\n",
			h, stage, webStatus, vncStatus, sshStatus, screenInfo, screenshotPath)
		return nil
	}

	fmt.Printf("Container: %s on %s (version: macOS %s)\n", cfg.name, h, cfg.version)
	fmt.Printf("  State:    exists=%v running=%v\n", exists, running)
	printEndpoints(cfg)

	fmt.Println("\n--- Service Probes ---")
	fmt.Printf("  Web (noVNC): %s\n", webStatus)
	fmt.Printf("  VNC Server:  %s\n", vncStatus)
	fmt.Printf("  SSH Server:  %s\n", sshStatus)

	fmt.Println("\n--- QEMU / Hypervisor Status ---")
	if qmpStatus, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, "info status"); err == nil {
		for _, line := range strings.Split(qmpStatus, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "VM status:") {
				fmt.Printf("  VM State:    %s\n", line)
			}
		}
	}
	if qmpCPUs, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, "info cpus"); err == nil {
		for _, line := range strings.Split(qmpCPUs, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "* CPU") || strings.HasPrefix(line, "CPU") {
				fmt.Printf("  QEMU CPUs:   %s\n", line)
			}
		}
	}

	fmt.Println("\n--- Bootloader & Kernel Diagnostic Inspection ---")
	fmt.Printf("  Current Boot Stage: %s\n", stage)

	fmt.Println("\n--- Guest Screen Capture ---")
	if err == nil {
		fmt.Printf("  Screenshot:  Captured %s (auto-cropped) -> %s\n", screenInfo, screenshotPath)
	} else {
		fmt.Printf("  Screenshot:  Capture failed: %v\n", err)
	}

	return nil
}

func doScreenshot(ctx context.Context, cfg config, outPath string) error {
	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if !exists || !running {
		return fmt.Errorf("container %q is not running on %s", cfg.name, hostName(cfg.remoteHost))
	}
	w, h, err := captureGuestScreen(ctx, cfg.remoteHost, cfg.name, outPath, !cfg.noCrop)
	if err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}
	fmt.Printf("Saved screenshot (%dx%d) to %s\n", w, h, outPath)
	return nil
}

func assessBootStage(ctx context.Context, remoteHost, name string) string {
	logCmd := podmanCmd(ctx, remoteHost, "logs", "--tail", "100", name)
	out, err := logCmd.Output()
	logs := ""
	if err == nil {
		logs = string(out)
	}

	if strings.Contains(logs, "Kernel panic") || strings.Contains(logs, "panic(cpu") || strings.Contains(logs, "machine_check.c") {
		return "CRITICAL: Kernel Panic detected in serial logs"
	}

	// Check live CPU registers for halt/spin loop
	if regOut, err := queryQEMUMonitor(ctx, remoteHost, name, "info registers"); err == nil {
		rip := extractRegValue(regOut, "RIP=")
		if rip != "" {
			if disOut, err := queryQEMUMonitor(ctx, remoteHost, name, "xp /4i 0x"+rip); err == nil {
				if strings.Contains(disOut, "jmp") && strings.Contains(disOut, rip) {
					cr2 := extractRegValue(regOut, "CR2=")
					return fmt.Sprintf("CRITICAL: Kernel Spin Halt at RIP=0x%s (Fault Address: 0x%s)", rip, cr2)
				}
			}
		}
	}

	if strings.Contains(logs, "Language Chooser") || strings.Contains(logs, "macOS Utilities") {
		return "Stage 4/4: macOS Recovery GUI Active (Interactive Shell ready: Utilities -> Terminal)"
	}
	if strings.Contains(logs, "WindowServer") || strings.Contains(logs, "com.apple.xpc.launchd") {
		return "Stage 3.5/4: macOS Userland Services (launchd active, WindowServer initializing)"
	}
	if strings.Contains(logs, "HANDOFF TO XNU") {
		return "Stage 3/4: Kernel Active (XNU loaded, Apple logo displayed)"
	}
	if strings.Contains(logs, "OpenCore") || strings.Contains(logs, "BdsDxe") {
		return "Stage 2/4: OpenCore EFI Bootloader initializing"
	}
	if strings.Contains(logs, "Booting macOS using QEMU") {
		return "Stage 1/4: QEMU hypervisor initialized"
	}
	return "Stage 1/4: Container starting"
}

func probeTCPBanner(host string, port int, timeout time.Duration) (string, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil && n == 0 {
		return "Connected (no initial banner)", nil
	}
	return string(buf[:n]), nil
}

func queryQEMUMonitor(ctx context.Context, remoteHost, name, command string) (string, error) {
	pyScript := fmt.Sprintf(`
import socket
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.settimeout(2.0)
s.connect('/dev/shm/monitor.sock')
s.sendall(b'%s\n')
out = b''
try:
    while True:
        c = s.recv(1024)
        if not c: break
        out += c
except:
    pass
s.close()
print(out.decode('utf-8', errors='ignore'))
`, command)

	cmd := podmanCmd(ctx, remoteHost, "exec", name, "python3", "-c", pyScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func captureGuestScreen(ctx context.Context, remoteHost, name string, outPath string, autoCrop bool) (int, int, error) {
	dumpScript := `
import socket
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.settimeout(2.0)
s.connect('/dev/shm/monitor.sock')
s.sendall(b'screendump /dev/shm/screen.ppm\n')
s.close()
`
	cmd := podmanCmd(ctx, remoteHost, "exec", name, "python3", "-c", dumpScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		return 0, 0, fmt.Errorf("screendump command failed: %s: %w", string(out), err)
	}

	catCmd := podmanCmd(ctx, remoteHost, "exec", name, "cat", "/dev/shm/screen.ppm")
	ppmData, err := catCmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("read screen.ppm failed: %w", err)
	}

	return convertPPMToPNG(ppmData, outPath, autoCrop)
}

func convertPPMToPNG(ppmData []byte, outPath string, autoCrop bool) (int, int, error) {
	reader := bufio.NewReader(bytes.NewReader(ppmData))

	// 1. Read magic number
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, 0, fmt.Errorf("read PPM magic: %w", err)
	}
	magic := strings.TrimSpace(line)
	if magic != "P6" {
		return 0, 0, fmt.Errorf("unsupported PPM format %q (expected P6)", magic)
	}

	// 2. Read dimensions (skipping comments)
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			return 0, 0, fmt.Errorf("read PPM dimensions: %w", err)
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") && len(line) > 0 {
			break
		}
	}

	dims := strings.Fields(line)
	if len(dims) < 2 {
		return 0, 0, fmt.Errorf("invalid PPM dimension line: %q", line)
	}
	width, err := strconv.Atoi(dims[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: %w", err)
	}
	height, err := strconv.Atoi(dims[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height: %w", err)
	}

	// 3. Read maxval line
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			return 0, 0, fmt.Errorf("read PPM maxval: %w", err)
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") && len(line) > 0 {
			break
		}
	}

	// 4. Decode binary RGB pixels
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	buf := make([]byte, 3)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if _, err := io.ReadFull(reader, buf); err != nil {
				return 0, 0, fmt.Errorf("read pixel data at (%d,%d): %w", x, y, err)
			}
			img.SetNRGBA(x, y, color.NRGBA{R: buf[0], G: buf[1], B: buf[2], A: 255})
		}
	}

	var finalImg image.Image = img
	finalWidth, finalHeight := width, height
	if autoCrop {
		finalImg, finalWidth, finalHeight = cropBlackBorders(img)
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return 0, 0, fmt.Errorf("create output file %s: %w", outPath, err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, finalImg); err != nil {
		return 0, 0, fmt.Errorf("encode png: %w", err)
	}

	return finalWidth, finalHeight, nil
}

func cropBlackBorders(img *image.NRGBA) (image.Image, int, int) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	minX, minY := width, height
	maxX, maxY := -1, -1

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.NRGBAAt(x, y)
			if c.R > 8 || c.G > 8 || c.B > 8 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	if maxX < minX || maxY < minY {
		return img, width, height
	}

	const pad = 16
	if minX -= pad; minX < 0 {
		minX = 0
	}
	if minY -= pad; minY < 0 {
		minY = 0
	}
	if maxX += pad + 1; maxX > width {
		maxX = width
	}
	if maxY += pad + 1; maxY > height {
		maxY = height
	}

	cropRect := image.Rect(minX, minY, maxX, maxY)
	cropped := img.SubImage(cropRect)
	return cropped, cropRect.Dx(), cropRect.Dy()
}

func doInspect(ctx context.Context, cfg config) error {
	exists, running := containerRunning(ctx, cfg.remoteHost, cfg.name)
	if !exists || !running {
		return fmt.Errorf("container %q is not running on %s", cfg.name, hostName(cfg.remoteHost))
	}

	fmt.Printf("=== Guest Deep Inspection: %s on %s ===\n", cfg.name, hostName(cfg.remoteHost))

	// 1. Query CPU Registers
	regOut, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, "info registers")
	if err != nil {
		return fmt.Errorf("query registers: %w", err)
	}

	rip := extractRegValue(regOut, "RIP=")
	rsp := extractRegValue(regOut, "RSP=")
	cr0 := extractRegValue(regOut, "CR0=")
	cr2 := extractRegValue(regOut, "CR2=")
	cr3 := extractRegValue(regOut, "CR3=")
	cpl := extractRegValue(regOut, "CPL=")

	fmt.Println("\n[CPU Execution & Memory State]")
	fmt.Printf("  RIP: %s\n", rip)
	fmt.Printf("  RSP: %s\n", rsp)
	fmt.Printf("  CR0: %s (Protected/Paging)\n", cr0)
	fmt.Printf("  CR2: %s (Page Fault Address)\n", cr2)
	fmt.Printf("  CR3: %s (Page Table Base)\n", cr3)
	fmt.Printf("  CPL: %s (%s)\n", cpl, cplDescription(cpl))

	// 2. Disassemble Instructions at RIP
	if rip != "" {
		fmt.Printf("\n[Instructions at RIP (%s)]\n", rip)
		if disOut, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, fmt.Sprintf("x /8i 0x%s", rip)); err == nil {
			printCleanMonitorOutput(disOut)
		}
	}

	// 3. Stack memory dump
	if rsp != "" {
		fmt.Printf("\n[Stack Memory at RSP (%s)]\n", rsp)
		if stackOut, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, fmt.Sprintf("x /8gx 0x%s", rsp)); err == nil {
			printCleanMonitorOutput(stackOut)
		}
	}

	// 4. Panic / Fault Analysis
	fmt.Println("\n[Diagnosis]")
	if rip != "" {
		if disOut, err := queryQEMUMonitor(ctx, cfg.remoteHost, cfg.name, fmt.Sprintf("xp /4i 0x%s", rip)); err == nil && strings.Contains(disOut, "jmp") && strings.Contains(disOut, rip) {
			fmt.Printf("  ALERT: Kernel Spin Halt / Panic Loop detected at RIP=0x%s!\n", rip)
			if cr2 == "0000000000000040" {
				fmt.Println("  Cause: Kernel NULL pointer dereference (+0x40 in IOPMrootDomain / Power Management).")
			}
			return nil
		}
	}

	if cpl == "3" || strings.HasPrefix(rip, "00000001") || strings.HasPrefix(rip, "00007") {
		fmt.Println("  Status: Active Userland execution (WindowServer / Recovery GUI / Apps active).")
	} else if strings.HasPrefix(rip, "ffffff80") || strings.HasPrefix(rip, "ffffffb") {
		fmt.Println("  Status: Active 64-bit macOS XNU Kernel execution (Drivers / IOKit / Syscalls).")
	} else {
		fmt.Println("  Status: Active EFI / Bootloader execution.")
	}

	return nil
}

func extractRegValue(output, key string) string {
	idx := strings.Index(output, key)
	if idx == -1 {
		return ""
	}
	sub := output[idx+len(key):]
	fields := strings.Fields(sub)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func cplDescription(cpl string) string {
	switch cpl {
	case "0":
		return "Ring 0 - Kernel Supervisor Mode"
	case "3":
		return "Ring 3 - Userland Mode"
	default:
		return "Privilege Level " + cpl
	}
}

func printCleanMonitorOutput(out string) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "QEMU") || strings.HasPrefix(line, "(qemu)") {
			continue
		}
		fmt.Printf("    %s\n", line)
	}
}

