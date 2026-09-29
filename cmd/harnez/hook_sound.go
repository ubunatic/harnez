package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/sound"
)

var hookSoundExecutable = os.Executable

type hookSoundOptions struct {
	Sync       bool
	Executable func() (string, error)
	Play       func(ctx context.Context) error
}

func newHookSoundCmd() *cobra.Command {
	var syncMode bool
	cmd := &cobra.Command{
		Use:          "sound",
		Short:        "Play notification sound",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHookSound(cmd.Context(), hookSoundOptions{
				Sync: syncMode,
			})
		},
	}
	cmd.Flags().BoolVar(&syncMode, "sync", false, "play synchronously in the foreground")
	return cmd
}

func runHookSound(ctx context.Context, opts hookSoundOptions) error {
	if opts.Sync {
		if opts.Play != nil {
			return opts.Play(ctx)
		}
		return sound.PlayContext(ctx)
	}

	exeFn := opts.Executable
	if exeFn == nil {
		exeFn = hookSoundExecutable
	}
	exe, err := exeFn()
	if err != nil {
		return fmt.Errorf("lookup executable: %w", err)
	}

	detached := exec.Command(exe, "hook", "sound", "--sync")
	detached.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		defer devNull.Close()
		detached.Stdin = devNull
		detached.Stdout = devNull
		detached.Stderr = devNull
	} else {
		detached.Stdin = nil
		detached.Stdout = io.Discard
		detached.Stderr = io.Discard
	}

	if err := detached.Start(); err != nil {
		return fmt.Errorf("start detached sound player: %w", err)
	}
	return nil
}
