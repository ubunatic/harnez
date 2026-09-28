package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"ubunatic.com/harnez"
)

func newManCmd(rootCmd *cobra.Command) *cobra.Command {
	var install bool
	var targetDir string
	cmd := &cobra.Command{
		Use:   "man",
		Short: "Generate, print, or install roff man pages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if install || targetDir != "" {
				return installManPages(rootCmd, targetDir, cmd.OutOrStdout())
			}
			return doc.GenMan(rootCmd, manHeader(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&install, "install", false, "install man pages into the standard man directory")
	cmd.Flags().StringVar(&targetDir, "dir", "", "custom target directory for man pages (implies --install)")
	return cmd
}

func manHeader() *doc.GenManHeader {
	return &doc.GenManHeader{
		Title:   "HARNEZ",
		Section: "1",
		Source:  "harnez " + harnez.Version,
		Manual:  "Harnez Manual",
	}
}

func defaultManDir() string {
	if os.Geteuid() == 0 {
		return "/usr/local/share/man/man1"
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "man", "man1")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "share", "man", "man1")
	}
	return "/usr/local/share/man/man1"
}

func installManPages(rootCmd *cobra.Command, targetDir string, out io.Writer) error {
	if targetDir == "" {
		targetDir = defaultManDir()
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating man directory %s: %w", targetDir, err)
	}
	if err := doc.GenManTree(rootCmd, manHeader(), targetDir); err != nil {
		return fmt.Errorf("generating man pages in %s: %w", targetDir, err)
	}
	fmt.Fprintf(out, "Installed man pages to %s\n", targetDir)
	return nil
}
