package subagent

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/procs"
)

type processObserverKey struct{}

// WithProcessObserver records the provider before reading its output.
func WithProcessObserver(ctx context.Context, observer func(int, uint64) error) context.Context {
	return context.WithValue(ctx, processObserverKey{}, observer)
}

func observeProcess(ctx context.Context, cmd *exec.Cmd) error {
	observer, _ := ctx.Value(processObserverKey{}).(func(int, uint64) error)
	if observer == nil {
		return nil
	}
	all, err := (procs.LinuxProcReader{}).List()
	if err != nil {
		return err
	}
	var start uint64
	for _, p := range all {
		if p.PID == cmd.Process.Pid {
			start = p.Starttime
		}
	}
	return observer(cmd.Process.Pid, start)
}

func isolateProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func providerOutput(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	isolateProcess(cmd)
	if cmd.Cancel != nil {
		// Cancel the whole provider tree, not just its leader: a surviving
		// grandchild keeps stdout open and blocks the read below.
		cmd.Cancel = func() error { return KillGroup(cmd.Process.Pid, syscall.SIGKILL) }
		cmd.WaitDelay = 5 * time.Second
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// Observe immediately after Start, before consuming provider output.
	rd, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if err := observeProcess(ctx, cmd); err != nil {
		_ = KillGroup(cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		return nil, err
	}
	b, readErr := io.ReadAll(rd)
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
		}
		return b, err
	}
	return b, readErr
}
