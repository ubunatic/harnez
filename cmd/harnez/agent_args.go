package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/subagent"
)

// assemblePrompt joins prompt files, positional words, and the verbatim tail.
func assemblePrompt(files []string, words []string, afterDash []string, stdin io.Reader) (string, error) {
	parts := make([]string, 0, len(files)+2)
	for _, name := range files {
		var data []byte
		var err error
		if name == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			return "", fmt.Errorf("read prompt file %q: %w", name, err)
		}
		parts = append(parts, string(data))
	}
	if len(words) > 0 {
		parts = append(parts, strings.Join(words, " "))
	}
	if len(afterDash) > 0 {
		parts = append(parts, strings.Join(afterDash, " "))
	}
	prompt := strings.Join(parts, "\n\n")
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("no prompt given")
	}
	return prompt, nil
}

func oldStyleModelWord(word string) bool {
	for _, provider := range []string{"codex", "claude", "agy", "local"} {
		if strings.HasPrefix(word, provider+":") && len(word) > len(provider)+1 {
			return true
		}
	}
	return false
}

func promptStorage(files, words, tail []string, prompt string) string {
	if len(files) == 0 {
		return prompt
	}
	parts := make([]string, len(files))
	for i, name := range files {
		if name == "-" {
			parts[i] = "- (stdin)"
			continue
		}
		if info, err := os.Stat(name); err == nil {
			parts[i] = fmt.Sprintf("%s (%d bytes)", name, info.Size())
		} else {
			parts[i] = name
		}
	}
	if len(words) > 0 {
		parts = append(parts, strings.Join(words, " "))
	}
	if len(tail) > 0 {
		parts = append(parts, strings.Join(tail, " "))
	}
	return strings.Join(parts, "\n\n")
}

func resolveSession(store *subagent.FileSessionStore, name, dir string) (*subagent.Session, error) {
	session, err := store.Find(name)
	if err != nil {
		return nil, err
	}
	if dir == "" || dir == "." {
		return session, nil
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve session directory: %w", err)
	}
	have, err := filepath.Abs(session.WorkingDir)
	if err != nil || want != have {
		return nil, fmt.Errorf("session %q is outside directory %q", session.Name, dir)
	}
	return session, nil
}

func promptArgs(cmdArgs []string, dash int) ([]string, []string) {
	if dash < 0 || dash > len(cmdArgs) {
		return cmdArgs, nil
	}
	return cmdArgs[:dash], cmdArgs[dash:]
}

// noArgs rejects positional arguments and says what replaced them.
func noArgs(hint string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return fmt.Errorf("%s: unexpected argument %q; %s", cmd.Name(), args[0], hint)
	}
}

// silenceUsage keeps cobra's usage dump out of runtime and argument errors:
// agent hosts read this output, and every error message names the fix.
func silenceUsage(cmd *cobra.Command) {
	cmd.SilenceUsage = true
	for _, c := range cmd.Commands() {
		silenceUsage(c)
	}
}
