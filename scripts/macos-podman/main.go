// Command macos-podman manages containerized macOS KVM instances via Podman.
//
// Usage:
//
//	go run ./scripts/macos-podman [run|stop|status|clean] [flags]
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
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
		fmt.Fprintf(os.Stderr, `Usage: go run ./scripts/macos-podman [run|stop|status|boot-status|screenshot|clean] [flags]

Actions:
  run          Start macOS container (default)
  stop         Stop running macOS container (bounded timeout)
  status       Show basic status, endpoints, and quick HTTP check
  boot-status  Comprehensive boot diagnosis, guest stages, logs & screenshot
  screenshot   Capture current guest screen to PNG
  clean        Stop and remove container and storage directory

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
		case "run", "stop", "status", "boot-status", "boot", "screenshot", "clean":
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
	case "boot-status", "boot":
		err = doBootStatus(ctx, cfg)
	case "screenshot":
		outputPath := "/tmp/macos_screen.png"
		if fs.NArg() > 0 {
			outputPath = fs.Arg(0)
		}
		err = doScreenshot(ctx, cfg, outputPath)
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
		configureNoVNC(ctx, cfg.name)
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
	configureNoVNC(ctx, cfg.name)
	printEndpoints(cfg)
	return nil
}

func configureNoVNC(ctx context.Context, name string) {
	patchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(patchCtx, "podman", "exec", name, "sed", "-i", "s/UI.initSetting('show_dot', false);/UI.initSetting('show_dot', true);/g", "/usr/share/novnc/app/ui.js")
	_ = cmd.Run()
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

func doBootStatus(ctx context.Context, cfg config) error {
	exists, running := containerRunning(ctx, cfg.name)
	fmt.Printf("Container: %s (version: macOS %s)\n", cfg.name, cfg.version)
	fmt.Printf("  State:    exists=%v running=%v\n", exists, running)
	if !running {
		fmt.Println("  Status:   Guest is not running. Run 'go run ./scripts/macos-podman run' to start.")
		return nil
	}

	printEndpoints(cfg)

	fmt.Println("\n--- Service Probes ---")
	// 1. Web UI probe
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d", cfg.httpPort))
	if err == nil {
		resp.Body.Close()
		fmt.Printf("  Web (noVNC): HTTP %s (Active)\n", resp.Status)
	} else {
		fmt.Printf("  Web (noVNC): Down (%v)\n", err)
	}

	// 2. VNC probe
	if banner, err := probeTCPBanner(cfg.vncPort, 1*time.Second); err == nil {
		fmt.Printf("  VNC Server:  Active (%s)\n", strings.TrimSpace(banner))
	} else {
		fmt.Printf("  VNC Server:  Unreachable (%v)\n", err)
	}

	// 3. SSH probe
	if banner, err := probeTCPBanner(cfg.sshPort, 1*time.Second); err == nil {
		fmt.Printf("  SSH Server:  Active (%s)\n", strings.TrimSpace(banner))
	} else {
		fmt.Println("  SSH Server:  Waiting for macOS guest daemon to start...")
	}

	fmt.Println("\n--- QEMU / Hypervisor Status ---")
	if qmpStatus, err := queryQEMUMonitor(ctx, cfg.name, "info status"); err == nil {
		for _, line := range strings.Split(qmpStatus, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "VM status:") {
				fmt.Printf("  VM State:    %s\n", line)
			}
		}
	}
	if qmpCPUs, err := queryQEMUMonitor(ctx, cfg.name, "info cpus"); err == nil {
		for _, line := range strings.Split(qmpCPUs, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "* CPU") || strings.HasPrefix(line, "CPU") {
				fmt.Printf("  QEMU CPUs:   %s\n", line)
			}
		}
	}

	fmt.Println("\n--- Bootloader & Kernel Diagnostic Inspection ---")
	stage := assessBootStage(ctx, cfg.name)
	fmt.Printf("  Current Boot Stage: %s\n", stage)

	fmt.Println("\n--- Guest Screen Capture ---")
	screenshotPath := "/tmp/macos_screen.png"
	w, h, err := captureGuestScreen(ctx, cfg.name, screenshotPath)
	if err == nil {
		fmt.Printf("  Screenshot:  Captured %dx%d -> %s\n", w, h, screenshotPath)
	} else {
		fmt.Printf("  Screenshot:  Capture failed: %v\n", err)
	}

	return nil
}

func doScreenshot(ctx context.Context, cfg config, outPath string) error {
	exists, running := containerRunning(ctx, cfg.name)
	if !exists || !running {
		return fmt.Errorf("container %q is not running", cfg.name)
	}
	w, h, err := captureGuestScreen(ctx, cfg.name, outPath)
	if err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}
	fmt.Printf("Saved screenshot (%dx%d) to %s\n", w, h, outPath)
	return nil
}

func assessBootStage(ctx context.Context, name string) string {
	logCmd := exec.CommandContext(ctx, "podman", "logs", "--tail", "100", name)
	out, err := logCmd.Output()
	if err != nil {
		return "Unknown (logs unavailable)"
	}
	logs := string(out)

	if strings.Contains(logs, "Kernel panic") || strings.Contains(logs, "panic(cpu") {
		return "CRITICAL: Kernel Panic detected in serial logs"
	}
	if strings.Contains(logs, "HANDOFF TO XNU") {
		return "Stage 3/4: Kernel Active (XNU loaded, Apple logo displayed / Recovery initializing)"
	}
	if strings.Contains(logs, "OpenCore") || strings.Contains(logs, "BdsDxe") {
		return "Stage 2/4: OpenCore EFI Bootloader initializing"
	}
	if strings.Contains(logs, "Booting macOS using QEMU") {
		return "Stage 1/4: QEMU hypervisor initialized"
	}
	return "Stage 1/4: Container starting"
}

func probeTCPBanner(port int, timeout time.Duration) (string, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), timeout)
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

func queryQEMUMonitor(ctx context.Context, name, command string) (string, error) {
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

	cmd := exec.CommandContext(ctx, "podman", "exec", name, "python3", "-c", pyScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func captureGuestScreen(ctx context.Context, name string, outPath string) (int, int, error) {
	dumpScript := `
import socket
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.settimeout(2.0)
s.connect('/dev/shm/monitor.sock')
s.sendall(b'screendump /dev/shm/screen.ppm\n')
s.close()
`
	cmd := exec.CommandContext(ctx, "podman", "exec", name, "python3", "-c", dumpScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		return 0, 0, fmt.Errorf("screendump command failed: %s: %w", string(out), err)
	}

	catCmd := exec.CommandContext(ctx, "podman", "exec", name, "cat", "/dev/shm/screen.ppm")
	ppmData, err := catCmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("read screen.ppm failed: %w", err)
	}

	return convertPPMToPNG(ppmData, outPath)
}

func convertPPMToPNG(ppmData []byte, outPath string) (int, int, error) {
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

	outFile, err := os.Create(outPath)
	if err != nil {
		return 0, 0, fmt.Errorf("create output file %s: %w", outPath, err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, img); err != nil {
		return 0, 0, fmt.Errorf("encode png: %w", err)
	}

	return width, height, nil
}

