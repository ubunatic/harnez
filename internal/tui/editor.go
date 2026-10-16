package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// EditorSpec describes the external prompt editor (Ctrl+G).
type EditorSpec struct {
	// Fallbacks are tried in order when $VISUAL and $EDITOR are unset.
	Fallbacks []string `yaml:"fallbacks"`
	// TempPattern is the os.CreateTemp pattern of the prompt file.
	TempPattern string `yaml:"temp_pattern"`
}

// Editor opens prompt text in an external editor. The zero-valued hooks
// default to the process environment, exec.LookPath and a tty-attached run.
type Editor struct {
	Spec     EditorSpec
	Getenv   func(string) string
	LookPath func(string) (string, error)
	// Run runs argv with the terminal attached and returns its exit error.
	Run func(argv []string) error
	// TempDir is the directory of the prompt file; "" uses os.TempDir.
	TempDir string
}

// NewEditor returns an Editor for spec using the real environment.
func NewEditor(spec EditorSpec) *Editor {
	return &Editor{Spec: spec}
}

// Command resolves the editor argv: $VISUAL, then $EDITOR (both may carry
// arguments, e.g. "code -w"), then the first spec fallback found on PATH.
func (e *Editor) Command() ([]string, error) {
	getenv, lookPath := e.Getenv, e.LookPath
	if getenv == nil {
		getenv = os.Getenv
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	for _, v := range []string{"VISUAL", "EDITOR"} {
		if argv := strings.Fields(getenv(v)); len(argv) > 0 {
			return argv, nil
		}
	}
	for _, name := range e.Spec.Fallbacks {
		if p, err := lookPath(name); err == nil {
			return []string{p}, nil
		}
	}
	return nil, fmt.Errorf("no editor: set $VISUAL or $EDITOR (fallbacks %v not found)", e.Spec.Fallbacks)
}

// Edit writes text to a temp file, runs the editor on it and returns the
// saved content without trailing newlines. On a failed run it returns the
// error and no text. The temp file is always removed.
func (e *Editor) Edit(text string) (string, error) {
	argv, err := e.Command()
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(e.TempDir, e.Spec.TempPattern)
	if err != nil {
		return "", fmt.Errorf("editor temp file: %w", err)
	}
	path := f.Name()
	defer os.Remove(path)
	_, werr := f.WriteString(text)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return "", fmt.Errorf("editor temp file: %w", werr)
	}
	run := e.Run
	if run == nil {
		run = func(argv []string) error { return runOnTTY(argv, "", nil) }
	}
	if err := run(append(argv[:len(argv):len(argv)], path)); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("%s exited with status %d", argv[0], exit.ExitCode())
		}
		return "", fmt.Errorf("run %s: %w", argv[0], err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read editor file: %w", err)
	}
	return strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), nil
}

// runOnTTY runs argv in dir ("" is the current one) with env (nil is the
// process environment) on the controlling terminal, which loom also uses, so
// the child (editor, provider CLI) reads the keyboard even when stdin is a pipe.
func runOnTTY(argv []string, dir string, env []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer tty.Close()
		cmd.Stdin, cmd.Stdout = tty, tty
	}
	return cmd.Run()
}
