package server

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// RunnerBrokerHandle manages a child process's inherited broker FD and any
// host-side relay lifetime attached to that launch.
type RunnerBrokerHandle struct {
	runnerFile *os.File
	cancel     context.CancelFunc
	done       <-chan error
}

// ReleaseChildFD closes the parent process's copy of the child FD after cmd.Start.
func (h *RunnerBrokerHandle) ReleaseChildFD() {
	if h == nil || h.runnerFile == nil {
		return
	}
	_ = h.runnerFile.Close()
	h.runnerFile = nil
}

// Close releases any remaining parent FD copy and stops the host relay cleanly.
func (h *RunnerBrokerHandle) Close() error {
	if h == nil {
		return nil
	}
	h.ReleaseChildFD()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	if h.done == nil {
		return nil
	}
	err := <-h.done
	h.done = nil
	return err
}

// PrepareCmdRunnerBroker binds a launch-scoped runner broker FD to cmd.
// On the host (TAIA_SESSION_DIR unset), it starts a host RelayRunnerBroker tied
// to ctx for the child's lifetime. Inside a runner (TAIA_SESSION_DIR set), it
// forwards a validated inherited FD when available and otherwise strips stale FD
// environment entries so the child fails closed without contacting Config.Socket.
func PrepareCmdRunnerBroker(ctx context.Context, cmd *exec.Cmd) (*RunnerBrokerHandle, error) {
	if cmd == nil {
		return nil, nil
	}
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	baseEnv := cmd.Env
	if baseEnv == nil {
		baseEnv = os.Environ()
	}
	if os.Getenv("TAIA_SESSION_DIR") != "" {
		return prepareForwardedRunnerBroker(cfg, cmd, baseEnv)
	}
	hostSocket, err := cfg.Socket()
	if err != nil {
		cmd.Env = setEnvKey(baseEnv, cfg.RunnerChannel.FDEnv, "", false)
		return nil, err
	}
	hostEnd, runnerEnd, err := SocketPair()
	if err != nil {
		cmd.Env = setEnvKey(baseEnv, cfg.RunnerChannel.FDEnv, "", false)
		return nil, err
	}
	relayCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- RelayRunnerBroker(relayCtx, hostEnd, hostSocket, cfg) }()

	fdNum := 3 + len(cmd.ExtraFiles)
	cmd.ExtraFiles = append(cmd.ExtraFiles, runnerEnd)
	cmd.Env = setEnvKey(baseEnv, cfg.RunnerChannel.FDEnv, strconv.Itoa(fdNum), true)
	return &RunnerBrokerHandle{runnerFile: runnerEnd, cancel: cancel, done: done}, nil
}

func prepareForwardedRunnerBroker(cfg Config, cmd *exec.Cmd, baseEnv []string) (*RunnerBrokerHandle, error) {
	fd, err := parseAndValidateRunnerFD(cfg)
	if err != nil {
		// If the env var named an existing AF_UNIX socket of any type, ensure it
		// is close-on-exec before stripping the env var so children cannot probe it.
		if raw := strings.TrimSpace(os.Getenv(cfg.RunnerChannel.FDEnv)); raw != "" {
			if rawFD, convErr := strconv.Atoi(raw); convErr == nil && rawFD >= 3 {
				if domain, optErr := syscall.GetsockoptInt(rawFD, syscall.SOL_SOCKET, syscall.SO_DOMAIN); optErr == nil && domain == syscall.AF_UNIX {
					syscall.CloseOnExec(rawFD)
				}
			}
		}
		cmd.Env = setEnvKey(baseEnv, cfg.RunnerChannel.FDEnv, "", false)
		return nil, nil
	}
	dupFile, err := dupFDAsFile(fd)
	if err != nil {
		cmd.Env = setEnvKey(baseEnv, cfg.RunnerChannel.FDEnv, "", false)
		return nil, err
	}
	fdNum := 3 + len(cmd.ExtraFiles)
	cmd.ExtraFiles = append(cmd.ExtraFiles, dupFile)
	cmd.Env = setEnvKey(baseEnv, cfg.RunnerChannel.FDEnv, strconv.Itoa(fdNum), true)
	return &RunnerBrokerHandle{runnerFile: dupFile}, nil
}

func dupFDAsFile(fd int) (*os.File, error) {
	dupFD, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(dupFD), "harnez-broker-fwd"), nil
}

func setEnvKey(env []string, key, value string, set bool) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	if set {
		out = append(out, prefix+value)
	}
	return out
}
