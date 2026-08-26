package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/distill"
)

func newDistillCmd() *cobra.Command {
	var mode string
	var maxLines int
	var noDedup bool

	cmd := &cobra.Command{
		Use:   "distill [-- command args...]",
		Short: "Strip low-signal noise (passing tests, ANSI, repeats) from command output",
		Long: `distill filters routine command output down to the signal an agent needs.

Streaming mode reads stdin:
  go test ./... | harnez distill

Wrapper mode runs a command, distills its combined output, and exits with
the wrapped command's exit code:
  harnez distill -- go test ./...
  harnez distill -- git status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := distill.Options{
				Mode:     distill.Mode(mode),
				MaxLines: maxLines,
				NoDedup:  noDedup,
			}

			if len(args) > 0 {
				return runDistillWrapper(args, opts)
			}

			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			fmt.Println(distill.Distill(string(input), opts))
			return nil
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "auto", "filter mode: auto, gotest, git, raw")
	cmd.Flags().IntVar(&maxLines, "max-lines", 300, "truncate output beyond this many lines (0 disables)")
	cmd.Flags().BoolVar(&noDedup, "no-dedup", false, "skip collapsing repeated lines")

	return cmd
}

func runDistillWrapper(args []string, opts distill.Options) error {
	if opts.Mode == distill.ModeAuto {
		opts.Mode = distill.DetectModeFromArgs(args)
	}

	c := exec.Command(args[0], args[1:]...)
	c.Stdin = os.Stdin
	out, runErr := c.CombinedOutput()

	fmt.Println(distill.Distill(string(out), opts))

	var exitErr *exec.ExitError
	if runErr != nil {
		if errors.As(runErr, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("run %v: %w", args, runErr)
	}
	return nil
}
