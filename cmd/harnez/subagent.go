package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/readcard"
	"ubunatic.com/harnez/internal/subagent"
)

func newSubagentCmd() *cobra.Command {
	var mode, dir, task, provider, card, lite string
	cmd := &cobra.Command{
		Use:   "subagent [flags] [-- launcher args...]",
		Short: "Stage provider-aware subagent context and optionally run a launcher",
		Long: `Stage a prompt and optional STYLE_GUIDE.png in --dir. Without a launcher,
print JSON metadata. With -- launcher args..., send the prompt on stdin and set
HARNEZ_STYLE_GUIDE to the staged image path. The launcher must support stdin prompts;
it or its agent must open/attach the PNG. This command does not itself attach images.
--doc-mode=auto uses vision for Claude/Codex and concise text for Gemini/local/unknown.
Explicit full, lite, or vision modes override provider selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := subagent.ParseDocMode(mode)
			if err != nil {
				return err
			}
			if strings.TrimSpace(task) == "" {
				return fmt.Errorf("--task is required")
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(absDir, 0755); err != nil {
				return err
			}
			var rules string
			if lite != "" {
				data, err := os.ReadFile(lite)
				if err != nil {
					return err
				}
				rules = string(data)
			}
			target := readcard.DetectProvider(os.Getenv)
			if cmd.Flags().Changed("provider") {
				target = readcard.ParseProvider(provider)
			}
			staged, err := subagent.StageSubagentContext(absDir, subagent.StageOptions{DocMode: parsed, Provider: target, Task: task, CardPath: card, LiteRules: rules})
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(staged)
			}
			child := exec.CommandContext(cmd.Context(), args[0], args[1:]...)
			child.Dir = absDir
			child.Stdin = strings.NewReader(staged.Prompt)
			child.Stdout, child.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
			child.Env = append(os.Environ(), "HARNEZ_STYLE_GUIDE="+staged.StagedCardPath)
			if err := child.Run(); err != nil {
				var exit *exec.ExitError
				if errors.As(err, &exit) {
					return &exitCodeError{Code: exit.ExitCode()}
				}
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mode, "doc-mode", "auto", "documentation mode: auto, vision, lite, full")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "staging and launcher working directory")
	cmd.Flags().StringVar(&task, "task", "", "task prompt")
	cmd.Flags().StringVar(&provider, "provider", "", "target profile: claude, codex, gemini, local")
	cmd.Flags().StringVar(&card, "card", "", "explicit style guide PNG")
	cmd.Flags().StringVar(&lite, "lite-rules", "", "concise rules file for lite mode")
	return cmd
}
