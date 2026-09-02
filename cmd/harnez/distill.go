package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/distill"
	"ubunatic.com/harnez/internal/telemetry"
)

func newDistillCmd() *cobra.Command {
	var mode string
	var maxLines int
	var maxBytes int
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
				MaxBytes: maxBytes,
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
	cmd.Flags().IntVar(&maxBytes, "max-bytes", 200_000, "hard byte cap on output; truncates head+tail with a note when exceeded (0 disables)")
	cmd.Flags().BoolVar(&noDedup, "no-dedup", false, "skip collapsing repeated lines")

	cmd.AddCommand(newDistillHookCmd())
	return cmd
}

// distillAutopipeEnv opts a user into the PreToolUse Bash auto-pipe hook.
// Off by default: the hook is safe to install globally via `harnez apply`
// and stays a no-op until a user explicitly sets this.
const distillAutopipeEnv = "HARNEZ_DISTILL_AUTOPIPE"

type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName string            `json:"hookEventName"`
	UpdatedInput  map[string]string `json:"updatedInput"`
}

func newDistillHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook",
		Short: "PreToolUse hook: auto-pipe noisy Bash commands through distill",
		Long: `hook implements a Claude Code PreToolUse hook for the Bash matcher.

It is a no-op unless ` + distillAutopipeEnv + ` is set to "1" or "true": harnez apply
can install this hook globally without it changing anything until a user
opts in. When enabled, it rewrites known-noisy commands (go test/build/vet,
cargo test/build, pytest, npm test, make test/build/check, git status/diff/log)
to pipe their combined output through 'harnez distill', preserving the
original command's exit code via 'set -o pipefail'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDistillHook(os.Stdin, os.Stdout)
		},
	}
}

func runDistillHook(in io.Reader, out io.Writer) error {
	enabled := os.Getenv(distillAutopipeEnv)
	if enabled != "1" && !strings.EqualFold(enabled, "true") {
		return nil
	}

	var payload hookInput
	if err := json.NewDecoder(in).Decode(&payload); err != nil {
		return fmt.Errorf("decode hook payload: %w", err)
	}
	if payload.ToolName != "Bash" {
		return nil
	}

	rewritten, ok := distill.RewriteBashCommand(payload.ToolInput.Command)
	if !ok {
		return nil
	}

	return json.NewEncoder(out).Encode(hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName: "PreToolUse",
			UpdatedInput:  map[string]string{"command": rewritten},
		},
	})
}

func runDistillWrapper(args []string, opts distill.Options) error {
	if opts.Mode == distill.ModeAuto {
		opts.Mode = distill.DetectModeFromArgs(args)
	}

	start := time.Now()
	c := exec.Command(args[0], args[1:]...)
	c.Stdin = os.Stdin
	out, runErr := c.CombinedOutput()
	duration := time.Since(start)

	distilled, rawBytes, distilledBytes := distill.DistillWithMetrics(string(out), opts)
	fmt.Println(distilled)

	exitCode := exitCodeFromError(runErr)

	var distBytesPtr *int64
	if opts.Mode != distill.ModeRaw {
		distBytesPtr = &distilledBytes
	}

	score, note := telemetry.ScoreShell(string(out), exitCode)

	recordExecTelemetry(execOptions{Tool: "distill"}, execCall{
		Tool:           "distill",
		ExitCode:       exitCode,
		DurationMs:     duration.Milliseconds(),
		RawBytes:       rawBytes,
		DistilledBytes: distBytesPtr,
		Score:          &score,
		Note:           note,
	})

	var exitErr *exec.ExitError
	if runErr != nil {
		if errors.As(runErr, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("run %v: %w", args, runErr)
	}
	return nil
}
