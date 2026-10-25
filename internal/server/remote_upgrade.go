package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/sshutil"
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

func StartRemoteBinaryUpgrade(ctx context.Context, host string, config RemoteUpgradeConfig, localVersion, goos, goarch, executable string, log io.Writer) {
	startRemoteBinaryUpgrade(ctx, host, config, localVersion, goos, goarch, executable, log, runRemoteCommand)
}

func startRemoteBinaryUpgrade(ctx context.Context, host string, config RemoteUpgradeConfig, localVersion, goos, goarch, executable string, log io.Writer, runner remoteCommandRunner) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		remoteBinaryUpgrader{config: config, run: runner, log: log}.upgrade(ctx, host, localVersion, goos, goarch, executable)
	}()
	return done
}

func runRemoteCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func commandFailure(output []byte, err error) string {
	message := strings.TrimSpace(string(output))
	if message != "" {
		message = strings.TrimSpace(strings.SplitN(message, "\n", 2)[0])
		if len(message) > 200 {
			message = message[:197] + "..."
		}
		return fmt.Sprintf("%s (%v)", message, err)
	}
	return err.Error()
}

func installFailureReason(output []byte, err error) string {
	message := strings.ToLower(string(output) + " " + err.Error())
	switch {
	case strings.Contains(message, "permission denied") || strings.Contains(message, "operation not permitted"):
		return "permission denied"
	case strings.Contains(message, "read-only file system") || strings.Contains(message, "filesystem is read-only"):
		return "read-only filesystem"
	case strings.Contains(message, "noexec") || strings.Contains(message, "cannot execute") && strings.Contains(message, "permission"):
		return "noexec mount or execution denied"
	default:
		return ""
	}
}

func installCommandFailure(path string, output []byte, err error) string {
	message := commandFailure(output, err)
	if reason := installFailureReason(output, err); reason != "" {
		return fmt.Sprintf("%s: %s (%s)", path, reason, message)
	}
	return message
}

func sshAuthenticationFailed(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "permission denied (publickey)") || strings.Contains(message, "no supported authentication methods available")
}

func (u remoteBinaryUpgrader) upgrade(ctx context.Context, host, localVersion, goos, goarch, executable string) {
	if !sshutil.ValidHost(host) || !sshutil.ValidInstallPath(u.config.InstallPath) {
		u.printf("remote upgrade: invalid destination or install path; skipping\n")
		return
	}
	local, ok := parseRemoteBuildVersion(localVersion)
	if !ok || !local.hasCount {
		u.printf("remote upgrade: local build version %q has no comparable commit count; skipping %s\n", localVersion, host)
		return
	}
	sshOptions := sshutil.Options(u.config.ConnectTimeout)
	sshArgs := append(append([]string(nil), sshOptions...), host)
	commandCtx, cancel := context.WithTimeout(ctx, u.config.CommandTimeout)
	platform, err := u.run(commandCtx, "ssh", append(sshArgs, "uname -sm")...)
	cancel()
	if err != nil {
		if os.Getenv("SSH_AUTH_SOCK") == "" && sshAuthenticationFailed(platform) {
			u.printf("remote upgrade: skipping %s: SSH authentication failed (no agent socket or usable key): %s\n", host, commandFailure(platform, err))
		} else {
			u.printf("remote upgrade: %s platform probe failed: %s\n", host, commandFailure(platform, err))
		}
		return
	}
	if !sameRemotePlatform(string(platform), goos, goarch) {
		u.printf("remote upgrade: %s platform %q differs from local %s/%s; skipping\n", host, strings.TrimSpace(string(platform)), goos, goarch)
		return
	}
	remoteVersionCtx, cancel := context.WithTimeout(ctx, u.config.CommandTimeout)
	versionOutput, err := u.run(remoteVersionCtx, "ssh", append(sshArgs, remoteShellPath(u.config.InstallPath)+" --version")...)
	cancel()
	if err != nil {
		u.printf("remote upgrade: %s version probe for %s failed: %s\n", host, u.config.InstallPath, installCommandFailure(u.config.InstallPath, versionOutput, err))
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
	info, err := os.Stat(executable)
	if err != nil {
		u.printf("remote upgrade: cannot stat local binary: %v\n", err)
		return
	}
	tempPath := fmt.Sprintf("%s.tmp-%d-%d", u.config.InstallPath, os.Getpid(), time.Now().UnixNano())
	transferCtx, cancel := context.WithTimeout(ctx, u.config.TransferTimeout)
	scpArgs := append([]string{"-q"}, sshOptions...)
	scpArgs = append(scpArgs, executable, host+":"+remoteSCPPath(tempPath))
	scpOutput, scpErr := u.run(transferCtx, "scp", scpArgs...)
	cancel()
	if scpErr != nil {
		u.printf("remote upgrade: copying binary to %s on %s failed: %s\n", u.config.InstallPath, host, installCommandFailure(u.config.InstallPath, scpOutput, scpErr))
		return
	}
	remoteDir := path.Dir(u.config.InstallPath)
	installCmd := fmt.Sprintf("mkdir -p %s && chmod %04o %s && mv -f %s %s", remoteShellPath(remoteDir), info.Mode().Perm(), remoteShellPath(tempPath), remoteShellPath(tempPath), remoteShellPath(u.config.InstallPath))
	installCtx, cancel := context.WithTimeout(ctx, u.config.CommandTimeout)
	installOutput, installErr := u.run(installCtx, "ssh", append(sshArgs, installCmd)...)
	cancel()
	if installErr != nil {
		u.printf("remote upgrade: installing copied binary at %s on %s failed: %s\n", u.config.InstallPath, host, installCommandFailure(u.config.InstallPath, installOutput, installErr))
		return
	}
	u.printf("remote upgrade: installed %s on %s\n", localVersion, host)
}

func remoteShellPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return `"$HOME"/` + "'" + strings.TrimPrefix(path, "~/") + "'"
	}
	return "'" + path + "'"
}

func remoteSCPPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return strings.TrimPrefix(path, "~/")
	}
	return path
}

func (u remoteBinaryUpgrader) printf(format string, args ...any) {
	if u.log != nil {
		fmt.Fprintf(u.log, format, args...)
	}
}
