package tui

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/harnez/internal/skillreg"
)

// pickItem is one picker row; Name is inserted after the trigger.
type pickItem struct {
	Name     string
	Desc     string
	norm     string // find.Normalize(Name)
	baseNorm string // normalized basename (files) or description (commands)
}

func newPickItem(name, desc, alt string) pickItem {
	return pickItem{Name: name, Desc: desc, norm: find.Normalize(name), baseNorm: find.Normalize(alt)}
}

const (
	lazyIdle = iota
	lazyLoading
	lazyReady
)

// lazyList is a picker source loaded once, off the UI loop.
type lazyList struct {
	state int
	items []pickItem
}

// pickerSources feeds the pickers. Loads run in goroutines; their results
// reach the widget only through post, so items are read and written on the
// UI loop alone.
type pickerSources struct {
	dir       string
	skillDirs []string
	maxFiles  int
	post      func(func())
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	files     lazyList
	skills    lazyList
}

// AttachPickers enables the @ file picker over the repository at dir and adds
// the skills found in skillDirs (<dir>/<name>/SKILL.md) to the / picker.
// post must run its callback on the UI loop (loom Pane.Post).
func (app *AppWidget) AttachPickers(dir string, skillDirs []string, post func(func())) {
	ctx, cancel := context.WithCancel(context.Background())
	app.sources = &pickerSources{
		dir: dir, skillDirs: skillDirs, maxFiles: app.Spec.Picker.MaxIndexedFiles,
		post: post, ctx: ctx, cancel: cancel,
	}
}

// ensure starts loading l once; done runs on the UI loop after the items land.
func (s *pickerSources) ensure(l *lazyList, load func(context.Context) []pickItem, done func()) {
	if l.state != lazyIdle {
		return
	}
	l.state = lazyLoading
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		items := load(s.ctx)
		s.post(func() {
			l.items, l.state = items, lazyReady
			done()
		})
	}()
}

// shutdown stops a running index build and waits for its goroutine.
func (s *pickerSources) shutdown() {
	s.cancel()
	s.wg.Wait()
}

func (s *pickerSources) loadFiles(ctx context.Context) []pickItem {
	return fileItems(listRepoFiles(ctx, s.dir, s.maxFiles))
}

// fileItems wraps slash paths as picker items ranked by path and basename.
func fileItems(paths []string) []pickItem {
	items := make([]pickItem, len(paths))
	for i, p := range paths {
		items[i] = newPickItem(p, "", path.Base(p))
	}
	return items
}

func (s *pickerSources) loadSkills(context.Context) []pickItem {
	return listSkills(s.skillDirs)
}

// listRepoFiles lists up to limit repository files relative to dir, tracked
// and untracked, honoring .gitignore (git ls-files). Outside a git work tree
// it walks dir, skipping hidden directories.
func listRepoFiles(ctx context.Context, dir string, limit int) []string {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	out, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		return walkFiles(ctx, dir, limit)
	}
	seen := map[string]bool{}
	var files []string
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	sc.Split(splitNUL)
	for len(files) < limit && sc.Scan() {
		if p := sc.Text(); p != "" && !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}
	if len(files) >= limit {
		cancel() // the cap cut the listing short; stop git
	}
	if err := cmd.Wait(); err != nil && len(files) == 0 && ctx.Err() == nil {
		return walkFiles(ctx, dir, limit)
	}
	sort.Strings(files) // git lists untracked files apart from tracked ones
	return files
}

func splitNUL(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func walkFiles(ctx context.Context, dir string, limit int) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(dir, p); err == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		if len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	return files
}

// listSkills reads <dir>/<name>/SKILL.md in each dir; the first dir that
// names a skill wins. Unreadable entries are skipped.
func listSkills(dirs []string) []pickItem {
	seen := map[string]bool{}
	var items []pickItem
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			sk, err := skillreg.ReadSkill(filepath.Join(d, e.Name(), "SKILL.md"))
			if err != nil || seen[sk.Name] || strings.ContainsAny(sk.Name, " \t\n") {
				continue
			}
			seen[sk.Name] = true
			items = append(items, newPickItem(sk.Name, oneLine(sk.Description), sk.Description))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}
