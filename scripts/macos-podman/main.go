// Command macos-podman manages containerized macOS KVM instances via Podman.
//
// Usage:
//
//	go run ./scripts/macos-podman [run|stop|status|clean] [flags]
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type config struct {
	action      string
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
}

func main() {
	cfg := config{
		name:        "macos-kvm",
		version:     "13",
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
	}

	fs := flag.NewFlagSet("macos-podman", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: go run ./scripts/macos-podman [run|stop|status|clean] [flags]

Actions:
  run       Start macOS container (default)
  stop      Stop running macOS container (bounded timeout)
  status    Show status, ports, and readiness check
  clean     Stop and remove container and storage directory

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Web UI (noVNC): http://localhost:%d
VNC:            localhost:%d
SSH:            ssh -p %d localhost
`, cfg.httpPort, cfg.vncPort, cfg.sshPort)
	}

	fs.StringVar(&cfg.name, "name", cfg.name, "Container name")
	fs.StringVar(&cfg.version, "version", cfg.version, "macOS version (11=BigSur, 12=Monterey, 13=Ventura, 14=Sonoma, 15=Sequoia)")
	fs.IntVar(&cfg.httpPort, "port", cfg.httpPort, "Web noVNC HTTP port")
	fs.IntVar(&cfg.vncPort, "vnc-port", cfg.vncPort, "VNC port")
	fs.IntVar(&cfg.sshPort, "ssh-port", cfg.sshPort, "SSH port")
	fs.IntVar(&cfg.cpuCores, "cpu", cfg.cpuCores, "CPU cores (default 1 for AMD Ryzen compatibility)")
	fs.StringVar(&cfg.ramSize, "ram", cfg.ramSize, "RAM size")
	fs.StringVar(&cfg.diskSize, "disk", cfg.diskSize, "Disk size")
	fs.StringVar(&cfg.storageDir, "storage", cfg.storageDir, "Storage path on host")
	fs.StringVar(&cfg.sharedDir, "shared", cfg.sharedDir, "Host directory to mount at /shared")
	fs.BoolVar(&cfg.noShared, "no-shared", false, "Disable mounting host directory")
	fs.BoolVar(&cfg.autoRm, "rm", false, "Remove container on exit (ephemeral mode)")
	fs.IntVar(&cfg.stopTimeout, "timeout", cfg.stopTimeout, "Stop timeout in seconds before force kill")

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "run", "stop", "status", "clean":
			cfg.action = args[0]
			args = args[1:]
		default:
			cfg.action = "run"
		}
	} else {
		cfg.action = "run"
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	ctx := context.Background()
	var err error
	switch cfg.action {
	case "run":
		err = doRun(ctx, cfg)
	case "stop":
		err = doStop(ctx, cfg)
	case "status":
		err = doStatus(ctx, cfg)
	case "clean":
		err = doClean(ctx, cfg)
	default:
		err = fmt.Errorf("unknown action: %s", cfg.action)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func checkPrerequisites() error {
	if _, err := exec.LookPath("podman"); err != nil {
		return fmt.Errorf("podman is not installed or not in PATH")
	}
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "WARNING: /dev/kvm not found — macOS will run unaccelerated")
		}
	}
	return nil
}

func isAMDCPU() bool {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "AuthenticAMD")
}

func containerRunning(ctx context.Context, name string) (exists bool, running bool) {
	cmd := exec.CommandContext(ctx, "podman", "inspect", "-f", "{{.State.Running}}", name)
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	return true, strings.TrimSpace(string(out)) == "true"
}

func doRun(ctx context.Context, cfg config) error {
	if err := checkPrerequisites(); err != nil {
		return err
	}

	exists, running := containerRunning(ctx, cfg.name)
	if exists && running {
		fmt.Printf("Container %q is already running.\n", cfg.name)
		printEndpoints(cfg)
		return nil
	}

	if exists && !running {
		fmt.Printf("Starting existing stopped container %q...\n", cfg.name)
		startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(startCtx, "podman", "start", cfg.name)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to start container: %s: %w", string(out), err)
		}
		printEndpoints(cfg)
		return nil
	}

	if err := os.MkdirAll(cfg.storageDir, 0o755); err != nil {
		return fmt.Errorf("create storage dir: %w", err)
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

	if isAMDCPU() {
		fmt.Println("AMD CPU detected: applying Intel CPUID spoofing for macOS kernel stability...")
		runArgs = append(runArgs, "-e", "ARGS=-cpu Haswell-noTSX,vendor=GenuineIntel", "-e", "QEMU_CPU=Haswell-noTSX")
	}

	if !cfg.noShared && cfg.sharedDir != "" {
		if abs, err := filepath.Abs(cfg.sharedDir); err == nil {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				runArgs = append(runArgs, "-v", fmt.Sprintf("%s:/shared:Z", abs))
			}
		}
	}

	if _, err := os.Stat("/dev/kvm"); err == nil {
		runArgs = append(runArgs, "--device", "/dev/kvm")
	}
	if _, err := os.Stat("/dev/net/tun"); err == nil {
		runArgs = append(runArgs, "--device", "/dev/net/tun", "--cap-add", "NET_ADMIN")
	}
	if cfg.autoRm {
		runArgs = append(runArgs, "--rm")
	}

	runArgs = append(runArgs, "docker.io/dockurr/macos:latest")

	fmt.Printf("Launching macOS %s in Podman (container: %s)...\n", cfg.version, cfg.name)
	launchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(launchCtx, "podman", runArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman run failed: %s: %w", string(out), err)
	}

	fmt.Printf("Container %q started successfully.\n", cfg.name)
	printEndpoints(cfg)
	return nil
}

func doStop(ctx context.Context, cfg config) error {
	exists, running := containerRunning(ctx, cfg.name)
	if !exists {
		fmt.Printf("Container %q does not exist.\n", cfg.name)
		return nil
	}
	if !running {
		fmt.Printf("Container %q is already stopped.\n", cfg.name)
		return nil
	}

	fmt.Printf("Stopping container %q (timeout: %ds)...\n", cfg.name, cfg.stopTimeout)
	stopCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.stopTimeout+5)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(stopCtx, "podman", "stop", cfg.name)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Graceful stop failed (%v). Force killing container %q...\n", err, cfg.name)
		_ = exec.CommandContext(ctx, "podman", "kill", cfg.name).Run()
	} else {
		fmt.Printf("Container %q stopped: %s\n", cfg.name, strings.TrimSpace(string(out)))
	}
	return nil
}

func doStatus(ctx context.Context, cfg config) error {
	exists, running := containerRunning(ctx, cfg.name)
	fmt.Printf("Container: %s\n", cfg.name)
	fmt.Printf("  Exists:  %v\n", exists)
	fmt.Printf("  Running: %v\n", running)

	if !running {
		return nil
	}

	printEndpoints(cfg)

	// Quick HTTP readiness probe
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d", cfg.httpPort))
	if err == nil {
		defer resp.Body.Close()
		fmt.Printf("  Web UI:  HTTP %s (Ready)\n", resp.Status)
	} else {
		fmt.Printf("  Web UI:  Connecting... (%v)\n", err)
	}

	// Tail recent logs
	logCmd := exec.CommandContext(ctx, "podman", "logs", "--tail", "5", cfg.name)
	if out, err := logCmd.Output(); err == nil && len(bytes.TrimSpace(out)) > 0 {
		fmt.Println("\nRecent Logs:")
		fmt.Println(string(out))
	}
	return nil
}

func doClean(ctx context.Context, cfg config) error {
	fmt.Printf("Cleaning up container %q...\n", cfg.name)
	_ = exec.CommandContext(ctx, "podman", "rm", "-f", cfg.name).Run()

	if _, err := os.Stat(cfg.storageDir); err == nil {
		fmt.Printf("Removing storage directory %s...\n", cfg.storageDir)
		if err := os.RemoveAll(cfg.storageDir); err != nil {
			return fmt.Errorf("remove storage dir: %w", err)
		}
	}
	fmt.Println("Cleanup complete.")
	return nil
}

func printEndpoints(cfg config) {
	fmt.Printf("  Web UI:  http://localhost:%d\n", cfg.httpPort)
	fmt.Printf("  VNC:     localhost:%d\n", cfg.vncPort)
	fmt.Printf("  SSH:     ssh -p %d localhost\n", cfg.sshPort)
	if !cfg.noShared && cfg.sharedDir != "" {
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
