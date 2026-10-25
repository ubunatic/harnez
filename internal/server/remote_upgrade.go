package server

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type remoteCommandRunner func(context.Context, string, ...string) ([]byte, error)

type remoteBuildVersion struct {
	release  [3]int
	count    int
	hasCount bool
}

var remoteVersionPattern = regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+)(?:\+([0-9]+))?`)

func parseRemoteBuildVersion(output string) (remoteBuildVersion, bool) {
	match := remoteVersionPattern.FindStringSubmatch(output)
	if match == nil {
		return remoteBuildVersion{}, false
	}
	parts := strings.Split(match[1], ".")
	var version remoteBuildVersion
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return remoteBuildVersion{}, false
		}
		version.release[i] = n
	}
	if match[2] != "" {
		count, err := strconv.Atoi(match[2])
		if err != nil {
			return remoteBuildVersion{}, false
		}
		version.count, version.hasCount = count, true
	}
	return version, true
}

func compareRemoteVersion(a, b remoteBuildVersion) int {
	for i := range a.release {
		if a.release[i] < b.release[i] {
			return -1
		}
		if a.release[i] > b.release[i] {
			return 1
		}
	}
	if a.hasCount && b.hasCount {
		if a.count < b.count {
			return -1
		}
		if a.count > b.count {
			return 1
		}
	}
	return 0
}

func sameRemotePlatform(remote string, goos, goarch string) bool {
	fields := strings.Fields(remote)
	if len(fields) != 2 || fields[0] != remoteOSName(goos) {
		return false
	}
	return fields[1] == remoteArchName(goos, goarch)
}

func remoteOSName(goos string) string {
	switch goos {
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	default:
		return goos
	}
}

func remoteArchName(goos, goarch string) string {
	if goarch == "amd64" {
		return "x86_64"
	}
	if goarch == "arm64" && goos == "linux" {
		return "aarch64"
	}
	return goarch
}

type remoteBinaryUpgrader struct {
	config RemoteUpgradeConfig
	run    remoteCommandRunner
	log    io.Writer
}

func newRemoteBinaryUpgrader(config RemoteUpgradeConfig, log io.Writer) remoteBinaryUpgrader {
	return remoteBinaryUpgrader{config: config, run: runRemoteCommand, log: log}
}

func runRemoteCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func sshConnectTimeout(timeout time.Duration) string {
	return strconv.Itoa(int(math.Ceil(timeout.Seconds())))
}

func (u remoteBinaryUpgrader) upgrade(ctx context.Context, host, localVersion, goos, goarch, executable string) {
	if strings.TrimSpace(host) == "" {
		return
	}
	local, ok := parseRemoteBuildVersion(localVersion)
	if !ok || !local.hasCount {
		u.printf("remote upgrade: local build version %q has no comparable commit count; skipping %s\n", localVersion, host)
		return
	}
	connectTimeout := sshConnectTimeout(u.config.ConnectTimeout)
	sshArgs := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=" + connectTimeout, host}
	commandCtx, cancel := context.WithTimeout(ctx, u.config.CommandTimeout)
	platform, err := u.run(commandCtx, "ssh", append(sshArgs, "uname -sm")...)
	cancel()
	if err != nil {
		u.printf("remote upgrade: %s platform probe failed: %v\n", host, err)
		return
	}
	if !sameRemotePlatform(string(platform), goos, goarch) {
		u.printf("remote upgrade: %s platform %q differs from local %s/%s; skipping\n", host, strings.TrimSpace(string(platform)), goos, goarch)
		return
	}
	remoteVersionCtx, cancel := context.WithTimeout(ctx, u.config.CommandTimeout)
	versionOutput, err := u.run(remoteVersionCtx, "ssh", append(sshArgs, u.config.InstallPath+" --version")...)
	cancel()
	if err != nil {
		u.printf("remote upgrade: %s version probe failed: %v\n", host, err)
		return
	}
	remote, ok := parseRemoteBuildVersion(string(versionOutput))
	if !ok {
		u.printf("remote upgrade: %s reported an unknown version %q; skipping\n", host, strings.TrimSpace(string(versionOutput)))
		return
	}
	cmp := compareRemoteVersion(remote, local)
	if cmp > 0 {
		u.printf("remote upgrade: %s runs newer version %s; keeping it\n", host, strings.TrimSpace(string(versionOutput)))
		return
	}
	// Legacy builds have no count. Upgrade them when their release is equal to
	// the local release as well, so the first count-aware build reaches the host.
	if remote.hasCount && cmp == 0 {
		u.printf("remote upgrade: %s already runs %s\n", host, strings.TrimSpace(string(versionOutput)))
		return
	}
	if remote.hasCount && cmp > 0 {
		u.printf("remote upgrade: %s runs newer version; keeping it\n", host)
		return
	}
	info, err := os.Stat(executable)
	if err != nil {
		u.printf("remote upgrade: cannot stat local binary: %v\n", err)
		return
	}
	tempPath := fmt.Sprintf("%s.tmp-%d-%d", u.config.InstallPath, os.Getpid(), time.Now().UnixNano())
	transferCtx, cancel := context.WithTimeout(ctx, u.config.TransferTimeout)
	_, err = u.run(transferCtx, "scp", "-q", "-o", "BatchMode=yes", "-o", "ConnectTimeout="+connectTimeout, executable, host+":"+tempPath)
	cancel()
	if err != nil {
		u.printf("remote upgrade: copying binary to %s failed: %v\n", host, err)
		return
	}
	remoteDir := path.Dir(u.config.InstallPath)
	installCmd := fmt.Sprintf("mkdir -p %s && chmod %04o %s && mv -f %s %s", remoteDir, info.Mode().Perm(), tempPath, tempPath, u.config.InstallPath)
	installCtx, cancel := context.WithTimeout(ctx, u.config.CommandTimeout)
	_, err = u.run(installCtx, "ssh", append(sshArgs, installCmd)...)
	cancel()
	if err != nil {
		u.printf("remote upgrade: installing copied binary on %s failed: %v\n", host, err)
		return
	}
	u.printf("remote upgrade: installed %s on %s\n", localVersion, host)
}

func (u remoteBinaryUpgrader) printf(format string, args ...any) {
	if u.log != nil {
		fmt.Fprintf(u.log, format, args...)
	}
}
