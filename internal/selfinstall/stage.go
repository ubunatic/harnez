package selfinstall

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// StageOptions configures one private staging run (`harnez install --stage`).
type StageOptions struct {
	// Dir is the new staging directory: absolute, clean, not yet existing,
	// with a real (no symlink) parent of mode 0700.
	Dir string
	// Source is the binary to stage; empty means the running executable.
	Source string
	// WriteMan writes the man pages into the given man1 directory.
	WriteMan func(manDir string) error
}

// Stage copies the harnez binary to Dir/bin/harnez and writes its man pages
// to Dir/share/man/man1, the layout PromoteStaged expects. It touches no
// shared root, so it is allowed in runner mode; on any failure it removes Dir.
// It replaces scripts/stage-install.sh (issue 1254).
func Stage(opts StageOptions, out io.Writer) (err error) {
	if err := validateStageDir(opts.Dir); err != nil {
		return err
	}
	source := opts.Source
	if source == "" {
		if source, err = os.Executable(); err != nil {
			return fmt.Errorf("stage: locate running binary: %w", err)
		}
	}
	if opts.WriteMan == nil {
		return errors.New("stage: no man page writer")
	}

	if err := mkdirMode(opts.Dir, 0o700); err != nil {
		return fmt.Errorf("stage: create %s: %w", opts.Dir, err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(opts.Dir)
		}
	}()
	manDir := filepath.Join(opts.Dir, "share", "man", "man1")
	for _, dir := range []string{"bin", "share", filepath.Join("share", "man"), filepath.Join("share", "man", "man1")} {
		if err := mkdirMode(filepath.Join(opts.Dir, dir), 0o755); err != nil {
			return fmt.Errorf("stage: create %s: %w", dir, err)
		}
	}
	if err := copyExecutable(source, filepath.Join(opts.Dir, "bin", "harnez")); err != nil {
		return fmt.Errorf("stage: copy binary: %w", err)
	}
	if err := opts.WriteMan(manDir); err != nil {
		return fmt.Errorf("stage: man pages: %w", err)
	}
	fmt.Fprintf(out, "Staged Harnez binary and man pages in %s\n", opts.Dir)
	return nil
}

func validateStageDir(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return fmt.Errorf("stage: destination %q must be an absolute clean path", dir)
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		return fmt.Errorf("stage: destination %q must name a new directory", dir)
	}
	info, err := os.Lstat(parent)
	if err != nil {
		return fmt.Errorf("stage: parent %s: %w", parent, err)
	}
	real, err := filepath.EvalSymlinks(parent)
	if err != nil || real != parent || !info.IsDir() {
		return fmt.Errorf("stage: parent %s must be a real directory without symlink components", parent)
	}
	if info.Mode().Perm() != 0o700 {
		return fmt.Errorf("stage: parent %s must have mode 0700, has %04o", parent, info.Mode().Perm())
	}
	if _, err := os.Lstat(dir); err == nil {
		return fmt.Errorf("stage: destination %s already exists", dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stage: destination %s: %w", dir, err)
	}
	return nil
}

// mkdirMode creates one new directory with exactly mode, independent of umask.
func mkdirMode(dir string, mode os.FileMode) error {
	if err := os.Mkdir(dir, mode); err != nil {
		return err
	}
	return os.Chmod(dir, mode)
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	outFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outFile, in); err != nil {
		_ = outFile.Close()
		return err
	}
	if err := outFile.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, 0o755)
}
