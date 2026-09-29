package skillreg

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// bundledPrefix starts the marker of a skill that ships inside the harnez
// binary (config.yaml bundled_skills). `harnez apply` owns those copies;
// registry installs never touch them and vice versa.
const bundledPrefix = "bundled: true\n"

// Bundled is one third-party skill vendored into harnez.
type Bundled struct {
	Name   string
	URL    string
	Commit string
	Auto   bool
	FS     fs.FS // the skill directory
}

// IsBundled reports whether dir holds a bundled skill copy.
func IsBundled(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, MarkerFile))
	return err == nil && bytes.HasPrefix(data, []byte(bundledPrefix))
}

// SyncBundled makes every target hold exactly the bundled skills it should
// get: explicit-only skills go to agents with an explicit switch, as for
// registry installs. Copies are rewritten only when their content differs,
// and bundled copies no longer configured are removed. A dir of the same
// name that harnez did not bundle is left alone and reported in notes.
func SyncBundled(targets []Target, skills []Bundled) (changed, notes []string, err error) {
	for _, t := range targets {
		// Sweep staging dirs a crashed run left behind.
		if left, _ := filepath.Glob(filepath.Join(t.Dir, ".*.harnez-staging")); len(left) > 0 {
			for _, l := range left {
				_ = os.RemoveAll(l)
			}
		}
		var want []string
		for _, s := range skills {
			if !t.receives(s.Auto) {
				continue
			}
			want = append(want, s.Name)
			c, note, err := syncOne(t, s)
			if err != nil {
				return changed, notes, fmt.Errorf("bundled skill %s [%s]: %w", s.Name, t.Agent, err)
			}
			if c != "" {
				changed = append(changed, c)
			}
			if note != "" {
				notes = append(notes, note)
			}
		}
		entries, err := os.ReadDir(t.Dir)
		if err != nil {
			continue // no target dir, nothing to remove
		}
		for _, e := range entries {
			dir := filepath.Join(t.Dir, e.Name())
			if e.IsDir() && !slices.Contains(want, e.Name()) && IsBundled(dir) {
				if err := os.RemoveAll(dir); err != nil {
					return changed, notes, err
				}
				changed = append(changed, "removed "+dir)
			}
		}
	}
	return changed, notes, nil
}

func syncOne(t Target, s Bundled) (changed, note string, err error) {
	dst := filepath.Join(t.Dir, s.Name)
	if _, err := os.Stat(dst); err == nil && !IsBundled(dst) {
		how := "not managed by harnez"
		if _, err := os.Stat(filepath.Join(dst, MarkerFile)); err == nil {
			how = "installed by 'harnez skill install'; run 'harnez skill remove " + s.Name + "' to use the bundled copy"
		}
		return "", fmt.Sprintf("skipped bundled skill %s in %s: %s", s.Name, t.Dir, how), nil
	}
	st := filepath.Join(t.Dir, "."+s.Name+".harnez-staging")
	defer os.RemoveAll(st)
	if err := os.RemoveAll(st); err != nil {
		return "", "", err
	}
	if err := copyFS(s.FS, st); err != nil {
		return "", "", err
	}
	marker := fmt.Sprintf("%surl: %s\ncommit: %s\n", bundledPrefix, s.URL, s.Commit)
	if err := os.WriteFile(filepath.Join(st, MarkerFile), []byte(marker), 0o644); err != nil {
		return "", "", err
	}
	if !s.Auto {
		if err := makeExplicit(t.Agent, st); err != nil {
			return "", "", err
		}
	}
	if same, err := sameTree(st, dst); err != nil || same {
		return "", "", err
	}
	if err := os.RemoveAll(dst); err != nil {
		return "", "", err
	}
	if err := os.Rename(st, dst); err != nil {
		return "", "", err
	}
	return "wrote " + dst, "", nil
}

// copyFS writes an fs.FS tree to dst. Embedded files carry no mode, so
// files starting with "#!" become executable.
func copyFS(fsys fs.FS, dst string) error {
	return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if bytes.HasPrefix(data, []byte("#!")) {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
}

// sameTree reports whether two dirs hold the same files with the same content.
func sameTree(a, b string) (bool, error) {
	read := func(root string) (map[string]string, error) {
		files := map[string]string{}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			files[rel] = string(data)
			return nil
		})
		return files, err
	}
	fa, err := read(a)
	if err != nil {
		return false, err
	}
	fb, err := read(b)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(fa) != len(fb) {
		return false, nil
	}
	for k, v := range fa {
		if w, ok := fb[k]; !ok || w != v {
			return false, nil
		}
	}
	return true, nil
}

// NameOK reports whether name is usable as a skill directory name.
func NameOK(name string) bool {
	return validName.MatchString(name) && !strings.Contains(name, "..")
}
