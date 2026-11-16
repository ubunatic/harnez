package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/runstate"
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
	if sessDir := runstate.RunnerDir(); sessDir != "" {
		if targetDir == "" {
			return fmt.Errorf("man --install to shared man directory is host-only in runner mode; pass --dir inside %s", sessDir)
		}
		if err := validateManDirInRunnerSession(sessDir, targetDir); err != nil {
			return err
		}
	}
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

func validateManDirInRunnerSession(sessDir, targetDir string) error {
	envName := runstate.RunnerDirEnv()
	if !filepath.IsAbs(sessDir) || !filepath.IsAbs(targetDir) {
		return fmt.Errorf("man --install target %s and $%s must be absolute in runner mode", targetDir, envName)
	}
	realSess, err := filepath.EvalSymlinks(filepath.Clean(sessDir))
	if err != nil {
		return fmt.Errorf("resolve $%s: %w", envName, err)
	}
	realTarget, err := resolveExistingPrefix(filepath.Clean(targetDir))
	if err != nil {
		return fmt.Errorf("resolve man target %s: %w", targetDir, err)
	}
	if realTarget != realSess && !strings.HasPrefix(realTarget, realSess+string(filepath.Separator)) {
		return fmt.Errorf("man --install target %s escapes $%s in runner mode", targetDir, envName)
	}
	return nil
}

func resolveExistingPrefix(absPath string) (string, error) {
	cur := absPath
	var tail []string
	for {
		if _, err := os.Lstat(cur); err == nil {
			resolved, err := filepath.EvalSymlinks(cur)
			if err != nil {
				return "", err
			}
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no existing root for %s", absPath)
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}
