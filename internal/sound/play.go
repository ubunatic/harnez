package sound

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var (
	// ErrTimeout indicates a player process exceeded its allotted timeout and was killed.
	ErrTimeout = errors.New("sound player timed out")

	// ErrNoPlayer indicates all players in the fallback chain were missing or failed.
	ErrNoPlayer = errors.New("no sound player succeeded or available")

	// ErrUnsupportedOS indicates no sound configuration exists for the operating system.
	ErrUnsupportedOS = errors.New("unsupported operating system for sound playback")
)

// Options allows configuring playback behaviour, primarily for testing or explicit overrides.
type Options struct {
	Spec     *Spec
	OS       string
	File     string
	Timeout  time.Duration
	LookPath func(file string) (string, error)
	ExecCmd  func(ctx context.Context, name string, args ...string) *exec.Cmd
	Stderr   io.Writer
}

// Play plays the default notification sound using the embedded spec and runtime OS.
func Play() error {
	return PlayContext(context.Background())
}

// PlayContext plays the default notification sound respecting ctx cancellation.
func PlayContext(ctx context.Context) error {
	return PlayWithOptions(ctx, Options{})
}

// PlayWithOptions plays a sound using the provided options and fallback chain.
// If a player times out, playback stops immediately and does not try further players.
// If a player is missing (LookPath fails) or exits non-zero immediately, the next player is tried.
func PlayWithOptions(ctx context.Context, opts Options) error {
	if opts.Spec == nil {
		var err error
		opts.Spec, err = LoadSpec()
		if err != nil {
			return err
		}
	}

	osName := opts.OS
	if osName == "" {
		osName = runtime.GOOS
	}

	platform, err := opts.Spec.ForOS(osName)
	if err != nil {
		return err
	}

	soundFile := opts.File
	if soundFile == "" {
		soundFile = platform.SoundFile
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		var err error
		timeout, err = opts.Spec.TimeoutDuration()
		if err != nil {
			return err
		}
	}

	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	var lastErr error
	for _, player := range platform.Players {
		if len(player.Command) == 0 {
			continue
		}
		exe := player.Command[0]
		if _, err := lookPath(exe); err != nil {
			// Binary missing from PATH: fall through to next player
			continue
		}

		// Substitute {file} placeholder into arguments
		args := make([]string, len(player.Command))
		for i, token := range player.Command {
			args[i] = strings.ReplaceAll(token, "{file}", soundFile)
		}

		err := runPlayer(ctx, player.Name, args, timeout, opts)
		if err == nil {
			return nil
		}

		// If player timed out or parent context was canceled, stop immediately
		if errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}

		// Player exited non-zero immediately: continue to next player
		lastErr = err
	}

	if lastErr != nil {
		return fmt.Errorf("%w: %v", ErrNoPlayer, lastErr)
	}
	return fmt.Errorf("%w: %s", ErrNoPlayer, osName)
}

func runPlayer(ctx context.Context, name string, args []string, timeout time.Duration, opts Options) error {
	playerCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if opts.ExecCmd != nil {
		cmd = opts.ExecCmd(playerCtx, args[0], args[1:]...)
	} else {
		cmd = exec.CommandContext(playerCtx, args[0], args[1:]...)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	} else {
		cmd.SysProcAttr.Setpgid = true
	}

	cmd.Cancel = func() error {
		if cmd.Process != nil && cmd.Process.Pid > 1 {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = 200 * time.Millisecond

	cmd.Stdin = nil
	if cmd.Stdout == nil {
		cmd.Stdout = io.Discard
	}
	if cmd.Stderr == nil {
		if opts.Stderr != nil {
			cmd.Stderr = opts.Stderr
		} else {
			cmd.Stderr = io.Discard
		}
	}

	err := cmd.Run()
	if err == nil {
		return nil
	}

	if playerCtx.Err() != nil {
		if errors.Is(playerCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("player %q timed out after %s: %w", name, timeout, ErrTimeout)
		}
		return playerCtx.Err()
	}

	return fmt.Errorf("player %q exited with error: %w", name, err)
}
