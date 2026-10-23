package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestUsageBinaryReturnsWithinDeadline(t *testing.T) {
	skipShort(t, "builds and runs the harnez binary")
	bin := buildUsageTestBinary(t)
	tests := []struct {
		name       string
		serverMode string
		wantStatus string
	}{
		{name: "no server", wantStatus: "server starting"},
		{name: "starting", serverMode: "starting", wantStatus: "server starting"},
		{name: "accept never answers", serverMode: "hung", wantStatus: "server not answering"},
		{name: "healthy", serverMode: "healthy"},
		{name: "slow response after probe timeout", serverMode: "slow", wantStatus: "server not answering"},
		{name: "stale socket", serverMode: "stale", wantStatus: "server starting"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, runtimeDir := usageTestDirs(t)
			env := usageTestEnv(home, runtimeDir)
			seedUsageTestStore(t, home)
			var release chan struct{}
			var accepted <-chan struct{}
			if tt.serverMode == "starting" || tt.serverMode == "hung" || tt.serverMode == "healthy" || tt.serverMode == "slow" {
				var listener net.Listener
				listener, accepted, release = startUsageTestServer(t, runtimeDir, tt.serverMode)
				defer listener.Close()
			}
			if tt.serverMode == "stale" {
				listener, err := net.Listen("unix", usageTestSocketPath(runtimeDir))
				if err != nil {
					t.Fatal(err)
				}
				listener.(*net.UnixListener).SetUnlinkOnClose(false)
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if tt.serverMode == "no server" || tt.serverMode == "stale" {
				t.Cleanup(func() { stopUsageTestServer(t, bin, env) })
			}

			const processLimit = 9 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), processLimit)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "usage", "--minimal")
			cmd.Env = env
			var stdout, stderr strings.Builder
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			start := time.Now()
			runErr := cmd.Run()
			elapsed := time.Since(start)
			t.Logf("usage subprocess elapsed: %s", elapsed)
			if runErr != nil {
				t.Fatalf("usage did not return successfully in %s (elapsed %s): %v\nstdout:\n%s\nstderr:\n%s", processLimit, elapsed, runErr, stdout.String(), stderr.String())
			}
			if elapsed >= 10*time.Second {
				t.Fatalf("usage took %s, want under 10s", elapsed)
			}
			if tt.wantStatus != "" && !strings.Contains(stderr.String(), tt.wantStatus) {
				t.Fatalf("stderr lacks status %q:\n%s", tt.wantStatus, stderr.String())
			}
			if tt.wantStatus != "" && strings.Contains(stdout.String(), tt.wantStatus) {
				t.Fatalf("status leaked to parseable stdout:\n%s", stdout.String())
			}
			if accepted != nil {
				select {
				case <-accepted:
				default:
					t.Fatal("usage did not connect to the fake server")
				}
			}
			if release != nil {
				close(release)
			}
		})
	}
}

func TestUsageBinaryStoreReadDeadlinePrintsError(t *testing.T) {
	skipShort(t, "builds and runs the harnez binary")
	const usageDeadline = "250ms"
	bin := buildUsageTestBinaryWithDeadline(t, usageDeadline)
	home, runtimeDir := usageTestDirs(t)
	storePath := filepath.Join(home, "data", "harnez", "telemetry.sqlite")
	if err := os.MkdirAll(filepath.Dir(storePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(storePath, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*raceFactor)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "usage", "--minimal", "--offline")
	cmd.Env = usageTestEnv(home, runtimeDir)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	t.Logf("slow-store binary elapsed: %s", elapsed)
	if err == nil {
		t.Fatal("usage unexpectedly succeeded with a blocked store open")
	}
	if ctx.Err() != nil {
		t.Fatalf("usage process exceeded its test limit (%s): %v", elapsed, err)
	}
	if elapsed >= time.Second*raceFactor {
		t.Fatalf("usage returned after %s, want under %s", elapsed, time.Second*raceFactor)
	}
	if !strings.Contains(stderr.String(), "context deadline exceeded") {
		t.Fatalf("stderr lacks store deadline error:\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should not contain a partial dashboard after store failure:\n%s", stdout.String())
	}
}

func TestUsageWatchShowsHungServerStatusPTY(t *testing.T) {
	skipShort(t, "runs harnez usage --watch in a PTY")
	bin := buildUsageTestBinary(t)
	home, runtimeDir := usageTestDirs(t)
	env := usageTestEnv(home, runtimeDir)
	seedUsageTestStore(t, home)
	listener, accepted, release := startUsageTestServer(t, runtimeDir, "hung")
	defer listener.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "usage", "--watch")
	cmd.Env = env
	master, err := pty.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	chunks := make(chan string, 32)
	go func() {
		defer close(chunks)
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				chunks <- string(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	var transcript strings.Builder
	for !strings.Contains(transcript.String(), "server not answering since ") {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				t.Fatalf("PTY closed before status row appeared:\n%s", transcript.String())
			}
			transcript.WriteString(chunk)
		case <-ctx.Done():
			t.Fatalf("watch did not show hung-server status:\n%s", transcript.String())
		}
	}
	select {
	case <-accepted:
	default:
		t.Fatal("watch did not connect to the hung server")
	}
	if _, err := master.Write([]byte{'q'}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watch exited with error: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("watch did not exit after q")
	}
}

func buildUsageTestBinaryWithDeadline(t *testing.T, deadline string) string {
	t.Helper()
	serverSpec, err := filepath.Abs("../../spec/server.yaml")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(serverSpec)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "usage_deadline: 10s", "usage_deadline: "+deadline, 1)
	if updated == string(data) {
		t.Fatal("server spec usage_deadline was not replaced")
	}
	root := t.TempDir()
	override := filepath.Join(root, "server.yaml")
	if err := os.WriteFile(override, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(root, "overlay.json")
	overlay, err := json.Marshal(struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: map[string]string{serverSpec: override}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "harnez")
	cmd := exec.Command("go", "build", "-overlay", overlayPath, "-o", bin, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build deadline-test binary: %v\n%s", err, output)
	}
	return bin
}

func buildUsageTestBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "harnez")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build harnez binary: %v\n%s", err, output)
	}
	return bin
}

func startUsageTestServer(t *testing.T, runtimeDir, mode string) (net.Listener, <-chan struct{}, chan struct{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(usageTestSocketPath(runtimeDir)), 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", usageTestSocketPath(runtimeDir))
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan struct{})
	release := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		if mode == "hung" || mode == "slow" {
			<-release
			if mode == "slow" {
				writeUsageTestResponse(conn, 200, `{"health":"ok","server":"harnez","version":"test","build_identity":"test"}`)
			}
			return
		}
		if _, err := httpReadTestRequest(conn); err != nil {
			return
		}
		if mode == "starting" {
			writeUsageTestResponse(conn, 503, `{"health":"starting"}`)
			return
		}
		writeUsageTestResponse(conn, 200, `{"health":"ok","server":"harnez","version":"test","build_identity":"test"}`)
	}()
	return listener, accepted, release
}

func httpReadTestRequest(conn net.Conn) (string, error) {
	request, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return "", err
	}
	defer request.Body.Close()
	return request.Method + " " + request.URL.Path, nil
}

func writeUsageTestResponse(conn net.Conn, status int, body string) {
	statusText := "OK"
	if status == 503 {
		statusText = "Service Unavailable"
	}
	fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", status, statusText, len(body), body)
}

func stopUsageTestServer(t *testing.T, bin string, env []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "server", "stop")
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("isolated server cleanup: %v: %s", err, output)
	}
	terminateUsageTestServerProcess(t, bin, env)
}

func terminateUsageTestServerProcess(t *testing.T, bin string, env []string) {
	t.Helper()
	runtimeValue := ""
	for _, item := range env {
		if strings.HasPrefix(item, "XDG_RUNTIME_DIR=") {
			runtimeValue = strings.TrimPrefix(item, "XDG_RUNTIME_DIR=")
			break
		}
	}
	entries, err := os.ReadDir("/proc")
	if err != nil || runtimeValue == "" {
		return
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		procDir := filepath.Join("/proc", entry.Name())
		commandLine, err := os.ReadFile(filepath.Join(procDir, "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(string(commandLine), "\x00")
		if len(args) < 3 || args[0] != bin || args[1] != "server" {
			continue
		}
		environment, err := os.ReadFile(filepath.Join(procDir, "environ"))
		if err != nil || !strings.Contains(string(environment), "XDG_RUNTIME_DIR="+runtimeValue+"\x00") {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			t.Logf("terminate isolated server PID %d: %v", pid, err)
		}
	}
}

func usageTestDirs(t *testing.T) (string, string) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	runtimeDir, err := os.MkdirTemp("", "h873-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(runtimeDir) })
	for _, dir := range []string{home, runtimeDir, filepath.Join(runtimeDir, "harnez")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return home, runtimeDir
}

func seedUsageTestStore(t *testing.T, home string) {
	t.Helper()
	store, err := usagestore.Open(filepath.Join(home, "data", "harnez", "telemetry.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error {
		return store.Exec(ctx, query)
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteLatestUsageSummary(ctx, usagestore.UsageSummaryRecord{
		Hostname:   "usage-binary-test",
		ObservedAt: time.Now().UTC(),
		Payload:    []byte(`{"agents":[]}`),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func usageTestSocketPath(runtimeDir string) string {
	return filepath.Join(runtimeDir, "harnez", "server.sock")
}

func usageTestEnv(home, runtimeDir string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_RUNTIME_DIR=" + runtimeDir,
		"XDG_DATA_HOME=" + filepath.Join(home, "data"),
		"XDG_STATE_HOME=" + filepath.Join(home, "state"),
		"XDG_CONFIG_HOME=" + filepath.Join(home, "config"),
		"HARNEZ_DISABLE_RATE_FEEDBACK=1",
	}
}
