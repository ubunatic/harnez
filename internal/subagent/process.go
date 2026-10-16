package subagent

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
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

// IsolateProcess starts cmd in its own process group, so KillGroup(pid)
// reaches the whole tree.
func IsolateProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func providerOutput(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	return providerLines(ctx, cmd, nil)
}

// providerLines runs cmd like providerOutput and also passes each stdout line
// (without its newline) to fn as it arrives. The returned bytes are the full
// stdout either way; a nil fn reads it in one go.
func providerLines(ctx context.Context, cmd *exec.Cmd, fn func([]byte)) ([]byte, error) {
	IsolateProcess(cmd)
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
	var b []byte
	var readErr error
	if fn == nil {
		b, readErr = io.ReadAll(rd)
	} else {
		b, readErr = readLines(rd, fn)
	}
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitErr.Stderr = stderr.Bytes()
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return b, &ProviderExitError{Err: err, Stderr: msg}
			}
		}
		return b, err
	}
	return b, readErr
}

// runProviderCLI runs the provider CLI name in dir and passes each stdout
// line to fn as it arrives; a nil fn is the batch path. A command override
// replaces the process and its output lines are replayed to fn. A non-zero
// exit carries the trimmed stderr in the error.
func runProviderCLI(ctx context.Context, name, dir string, override func(context.Context, string, ...string) ([]byte, error), fn func([]byte), args []string) ([]byte, error) {
	if override != nil {
		b, err := override(ctx, name, args...)
		if fn != nil {
			_, _ = readLines(bytes.NewReader(b), fn)
		}
		return b, err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return providerLines(ctx, cmd, fn)
}

// ProviderExitError is a provider CLI's non-zero exit with its trimmed
// stderr, kept apart from model output so IsStaleSession matches only what
// the CLI itself reported.
type ProviderExitError struct {
	Err    error
	Stderr string
}

func (e *ProviderExitError) Error() string { return e.Err.Error() + ": " + lastLines(e.Stderr, 3) }
func (e *ProviderExitError) Unwrap() error { return e.Err }

// readLines collects all of rd and calls fn for every line, including a
// final line without a newline.
func readLines(rd io.Reader, fn func([]byte)) ([]byte, error) {
	var all []byte
	br := bufio.NewReaderSize(rd, 64*1024)
	for {
		line, err := br.ReadBytes('\n')
		all = append(all, line...)
		if trimmed := bytes.TrimRight(line, "\r\n"); len(trimmed) > 0 {
			fn(trimmed)
		}
		if err == io.EOF {
			return all, nil
		}
		if err != nil {
			return all, err
		}
	}
}
