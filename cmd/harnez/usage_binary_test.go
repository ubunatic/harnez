package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

var (
	usageTestBinaryDir  string
	usageTestBinaryPath string
	usageTestBinaryOnce sync.Once
	usageTestBinaryErr  error
)

func prepareUsageTestBinary() error {
	usageTestBinaryOnce.Do(func() {
		if supplied := os.Getenv("HARNEZ_TEST_BINARY"); supplied != "" {
			usageTestBinaryPath = supplied
			return
		}
		var err error
		usageTestBinaryDir, err = os.MkdirTemp("", "harnez-usage-test-bin-")
		if err != nil {
			usageTestBinaryErr = err
			return
		}
		usageTestBinaryPath = filepath.Join(usageTestBinaryDir, "harnez")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", usageTestBinaryPath, ".")
		output, err := cmd.CombinedOutput()
		if err != nil {
			_ = os.RemoveAll(usageTestBinaryDir)
			usageTestBinaryErr = fmt.Errorf("build harnez test binary: %w\n%s", err, output)
		}
	})
	return usageTestBinaryErr
}

func TestOneShotUsageDoesNotInvokeSSH(t *testing.T) {
	skipShort(t, "runs the harnez usage binary")
	root := t.TempDir()
	bin := buildUsageTestBinary(t)

	home, runtimeDir := usageTestDirs(t)
	seedUsageTestStore(t, home)
	sshDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// The stub records the PID of the process that ran it. The one-shot usage
	// process must never be that process; the auto-started collector may be.
	marker := filepath.Join(root, "ssh-parent-pids")
	sshStub := filepath.Join(sshDir, "ssh")
	if err := os.WriteFile(sshStub, []byte(fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$PPID\" >> %q\n", marker)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "config", "harnez"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "harnez", "local.yaml"), []byte("load:\n  watch_host: x600\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := usageTestEnv(home, runtimeDir)
	for i := range env {
		if strings.HasPrefix(env[i], "PATH=") {
			env[i] = "PATH=" + sshDir + string(os.PathListSeparator) + os.Getenv("PATH")
		}
	}

	// The first run auto-starts a detached collector that inherits env, so it
	// polls the fake host through the stub. Stop and wait for it on every exit.
	t.Cleanup(func() { stopUsageTestServer(t, bin, env) })

	for _, args := range [][]string{{"--minimal"}, {"--minimal", "--offline"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cmd := exec.CommandContext(ctx, bin, append([]string{"usage"}, args...)...)
		cmd.Env = env
		t.Cleanup(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})
		started := time.Now()
		output, err := cmd.CombinedOutput()
		elapsed := time.Since(started)
		cancel()
		t.Logf("usage %v elapsed: %s", args, elapsed)
		if err != nil {
			t.Fatalf("usage %v failed or exceeded 2s: %v\n%s", args, err, output)
		}
		if elapsed >= 2*time.Second {
			t.Fatalf("usage %v took %s, want under 2s", args, elapsed)
		}
		if elapsed >= time.Second {
			t.Fatalf("usage %v took %s, want under 1s", args, elapsed)
		}
		parents, err := os.ReadFile(marker)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if slices.Contains(strings.Fields(string(parents)), strconv.Itoa(cmd.Process.Pid)) {
			t.Fatalf("usage %v ran ssh itself (stub parent PID %d is the usage process)", args, cmd.Process.Pid)
		}
	}
}

func TestUsageBinaryReturnsWithinDeadline(t *testing.T) {
	skipShort(t, "builds and runs the harnez binary")
	bin := buildUsageTestBinary(t)
	home, runtimeDir := usageTestDirs(t)
	env := usageTestEnv(home, runtimeDir)
	seedUsageTestStore(t, home)
	listener, accepted, release := startUsageTestServer(t, runtimeDir, "hung")
	defer listener.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "usage", "--minimal")
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("usage watchdog subprocess did not exit successfully: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if ctx.Err() != nil {
		t.Fatalf("usage exceeded binary watchdog: %v", ctx.Err())
	}
	if !strings.Contains(stderr.String(), "server not answering") {
		t.Fatalf("stderr lacks hung-server status:\n%s", stderr.String())
	}
	if strings.Contains(stdout.String(), "server not answering") {
		t.Fatalf("status leaked to parseable stdout:\n%s", stdout.String())
	}
	select {
	case <-accepted:
	default:
		t.Fatal("usage did not connect to the hung fake server")
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

func buildUsageTestBinary(t *testing.T) string {
	t.Helper()
	if err := prepareUsageTestBinary(); err != nil {
		t.Fatalf("prepare shared usage test binary: %v", err)
	}
	if usageTestBinaryPath == "" {
		t.Fatal("shared usage test binary was not built")
	}
	return usageTestBinaryPath
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
	var pids []int
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
		pids = append(pids, pid)
	}
	waitForUsageTestProcessExit(t, pids)
}

// waitForUsageTestProcessExit blocks until every PID has exited or become a
// zombie, failing the test if one is still running after usageTestExitWait.
// The servers are detached (setsid, released), so they cannot be Wait()ed on.
func waitForUsageTestProcessExit(t *testing.T, pids []int) {
	t.Helper()
	deadline := time.NewTimer(usageTestExitWait)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for _, pid := range pids {
		for usageTestProcessRunning(pid) {
			select {
			case <-deadline.C:
				t.Fatalf("isolated server PID %d still running %s after SIGTERM", pid, usageTestExitWait)
			case <-ticker.C:
			}
		}
	}
}

const usageTestExitWait = 10 * time.Second

// usageTestProcessRunning reports whether pid exists and is not a zombie. The
// state is the field after the parenthesised command name in /proc/<pid>/stat.
func usageTestProcessRunning(pid int) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	text := string(stat)
	fields := strings.Fields(text[strings.LastIndex(text, ")")+1:])
	return len(fields) > 0 && fields[0] != "Z"
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

// usageTestRecoveryWait bounds the wait for the collector's first retry: the
// attached-viewer cadence starts at 10s, so recovery needs slightly more.
const usageTestRecoveryWait = 30 * time.Second

// TestCollectorRecoversRemoteLoadForOneShot is issue 879 M2b end to end: a
// running collector polls a fake host that is down, then up. A later one-shot
// usage shows the cached snapshot, and the one-shot process never runs ssh.
func TestCollectorRecoversRemoteLoadForOneShot(t *testing.T) {
	skipShort(t, "runs the harnez server and usage binaries and waits for the attached-viewer retry")
	bin := buildUsageTestBinary(t)
	home, runtimeDir := usageTestDirs(t)
	seedUsageTestStore(t, home)
	root := t.TempDir()
	sshDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "ssh-parent-pids")
	failed := filepath.Join(root, "ssh-failed")
	recovered := filepath.Join(root, "ssh-recovered")
	const snapshot = `{"timestamp":"2026-10-24T12:00:00Z","load":{"cpu":{"NumCPU":8,"CPUPercent":42,"CPUPercentOk":true,"Ok":true}}}`
	// The fake host refuses connections until the test creates recovered. The
	// failed marker is written only on a refused call, so the test can tell the
	// collector has seen the outage before it recovers the host.
	stub := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$PPID\" >> %q\nif test -e %q; then\n  printf '%%s' '%s'\n  exit 0\nfi\n: > %q\necho 'fake host down' >&2\nexit 255\n", marker, recovered, snapshot, failed)
	if err := os.WriteFile(filepath.Join(sshDir, "ssh"), []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "config", "harnez"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "harnez", "local.yaml"), []byte("load:\n  watch_host: x600\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := usageTestEnv(home, runtimeDir)
	for i := range env {
		if strings.HasPrefix(env[i], "PATH=") {
			env[i] = "PATH=" + sshDir + string(os.PathListSeparator) + os.Getenv("PATH")
		}
	}

	server := exec.Command(bin, "server")
	server.Env = env
	var serverLog strings.Builder
	server.Stdout, server.Stderr = &serverLog, &serverLog
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Wait() })
	t.Cleanup(func() { stopUsageTestServer(t, bin, env) })

	waitForUsageTestCondition(t, "the collector's first failed poll", func() bool {
		_, err := os.Stat(failed)
		return err == nil
	})
	if err := os.WriteFile(recovered, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	usage.StartClientHeartbeat(ctx)
	waitForUsageTestCondition(t, "the collector's retry to cache the snapshot", func() bool {
		return usage.ReadCachedRemoteLoadEntry("x600").Snapshot != nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "usage", "--minimal")
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("usage failed: %v\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "Remote Load") || !strings.Contains(text, "updated ") {
		t.Fatalf("one-shot usage lacks the recovered remote load box:\n%s", text)
	}
	if strings.Contains(text, "remote load unavailable") {
		t.Fatalf("one-shot usage still shows the unavailable placeholder:\n%s", text)
	}
	if !strings.Contains(text, "42%") {
		t.Fatalf("one-shot usage lacks the fake host's 42%% CPU reading:\n%s", text)
	}
	parents, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(strings.Fields(string(parents)), strconv.Itoa(cmd.Process.Pid)) {
		t.Fatalf("one-shot usage ran ssh itself (stub parent PID %d)", cmd.Process.Pid)
	}
}

// waitForUsageTestCondition polls ok until it holds or usageTestRecoveryWait
// passes. It waits on an external process's effects, so it cannot be a channel.
func waitForUsageTestCondition(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.NewTimer(usageTestRecoveryWait)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !ok() {
		select {
		case <-deadline.C:
			t.Fatalf("timed out after %s waiting for %s", usageTestRecoveryWait, what)
		case <-ticker.C:
		}
	}
}
