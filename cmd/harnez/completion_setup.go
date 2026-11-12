package main

import (
	"bytes"
	"fmt"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/claude"
)

// completionRenderer renders cobra's completion script for root, the same
// script `harnez completion <shell>` prints. `harnez apply` installs it.
func completionRenderer(root *cobra.Command) claude.CompletionRenderer {
	return func(shell string) ([]byte, error) {
		var buf bytes.Buffer
		var err error
		switch shell {
		case "bash":
			err = root.GenBashCompletionV2(&buf, true)
		case "zsh":
			err = root.GenZshCompletion(&buf)
		case "fish":
			err = root.GenFishCompletion(&buf, true)
		default:
			return nil, fmt.Errorf("no completion script for shell %q", shell)
		}
		return buf.Bytes(), err
	}
}
