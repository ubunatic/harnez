package skillreg

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Update states, from PlanUpdate.
const (
	Unchanged = "unchanged" // same content upstream
	Changed   = "changed"   // skill changed at the same path
	Moved     = "moved"     // same name found at another path; update follows it
	Gone      = "gone"      // no skill of that name upstream; the install is kept
)

// UpdatePlan describes what updating one entry would do, before anything is written.
type UpdatePlan struct {
	Entry      Entry
	Ref        string // ref the update resolves; empty means default branch
	Commit     string
	Path       string // skill path in the new commit (empty when Gone)
	Status     string
	DiffStat   string   // git diff --stat of the skill dir, old -> new
	Diff       string   // full diff
	Candidates []string // for Gone: skills named next to the old name in the repo's docs
	Hints      []string // for Gone: file:line where the old name is mentioned
	NewSkills  []Skill  // skills in the new commit that the old commit did not have
}

// PlanUpdate clones the entry's source and compares it with the cached commit.
// latest drops a pinned ref and follows the default branch.
func (r *Registry) PlanUpdate(name string, latest bool) (UpdatePlan, error) {
	e, ok, err := r.Get(name)
	if err != nil {
		return UpdatePlan{}, err
	}
	if !ok {
		return UpdatePlan{}, fmt.Errorf("skill %q is not an installed external skill", name)
	}
	plan := UpdatePlan{Entry: e, Ref: e.Ref}
	if latest {
		plan.Ref = ""
	}
	tmp, err := os.MkdirTemp("", "harnez-skill-plan-")
	if err != nil {
		return UpdatePlan{}, err
	}
	defer os.RemoveAll(tmp)
	clone := filepath.Join(tmp, "repo")
	if plan.Commit, err = Clone(e.URL, plan.Ref, clone); err != nil {
		return UpdatePlan{}, err
	}
	newSkills, err := FindSkills(clone)
	if err != nil {
		return UpdatePlan{}, err
	}
	oldRepo := r.SrcDir(e.Name, e.Commit)
	if _, err := os.Stat(oldRepo); err != nil {
		// Cache pruned: fetch the installed commit again to compare against.
		oldRepo = filepath.Join(tmp, "old")
		if _, err := Clone(e.URL, e.Commit, oldRepo); err != nil {
			return UpdatePlan{}, fmt.Errorf("cached source missing and re-fetch failed: %w", err)
		}
	}
	oldSkills, err := FindSkills(oldRepo)
	if err != nil {
		return UpdatePlan{}, err
	}
	for _, s := range newSkills {
		if !slices.ContainsFunc(oldSkills, func(o Skill) bool { return o.Name == s.Name }) {
			plan.NewSkills = append(plan.NewSkills, s)
		}
	}

	upstream := e.Name
	if e.Upstream != "" {
		upstream = e.Upstream
	}
	i := slices.IndexFunc(newSkills, func(s Skill) bool { return s.Name == upstream && s.Path == e.Path })
	if i < 0 {
		i = slices.IndexFunc(newSkills, func(s Skill) bool { return s.Name == upstream })
	}
	if i < 0 {
		plan.Status = Gone
		plan.Candidates, plan.Hints = renameHints(clone, upstream, newSkills)
		return plan, nil
	}
	plan.Path = newSkills[i].Path
	if plan.DiffStat, plan.Diff, err = diffDirs(filepath.Join(oldRepo, e.Path), filepath.Join(clone, plan.Path), tmp); err != nil {
		return UpdatePlan{}, err
	}
	switch {
	case plan.Path != e.Path:
		plan.Status = Moved
	case plan.Diff != "":
		plan.Status = Changed
	default:
		plan.Status = Unchanged
	}
	return plan, nil
}

// Update applies PlanUpdate. A Gone skill is left installed at its old commit
// and reported, not treated as an error. The --as rename and mode are kept.
func (r *Registry) Update(name string, latest bool) (UpdatePlan, Entry, error) {
	plan, err := r.PlanUpdate(name, latest)
	same := plan.Status == Unchanged && plan.Ref == plan.Entry.Ref && plan.Commit == plan.Entry.Commit
	if err != nil || plan.Status == Gone || same {
		return plan, plan.Entry, err
	}
	e := plan.Entry
	opts := InstallOptions{URL: e.URL, Ref: plan.Ref, Path: plan.Path, Name: e.Name, Auto: e.Auto}
	if e.Upstream != "" {
		opts.Name, opts.As = e.Upstream, e.Name
	}
	after, err := r.Install(opts)
	return plan, after, err
}

// diffDirs diffs two skill dirs in a scratch git repo, so paths read
// relative to the skill dir: the old dir is staged, the new one is the worktree.
func diffDirs(oldDir, newDir, tmp string) (stat, full string, err error) {
	d := filepath.Join(tmp, "diff")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", "", err
	}
	if err := git(d, "init", "--quiet"); err != nil {
		return "", "", err
	}
	if err := copyTree(oldDir, d); err != nil {
		return "", "", err
	}
	if err := git(d, "add", "-A"); err != nil {
		return "", "", err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return "", "", err
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			if err := os.RemoveAll(filepath.Join(d, e.Name())); err != nil {
				return "", "", err
			}
		}
	}
	if err := copyTree(newDir, d); err != nil {
		return "", "", err
	}
	if err := git(d, "add", "-A", "--intent-to-add"); err != nil {
		return "", "", err
	}
	diff := func(extra ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-C", d, "diff", "--no-color"}, extra...)...).Output()
		if err != nil {
			return "", fmt.Errorf("git diff: %w", err)
		}
		return string(out), nil
	}
	if stat, err = diff("--stat"); err != nil {
		return "", "", err
	}
	full, err = diff()
	return stat, full, err
}

// renameHints finds markdown lines in the repo that mention a vanished skill
// name and collects the current skill names on those lines as candidates.
func renameHints(repo, old string, skills []Skill) (candidates, hints []string) {
	count := map[string]int{}
	word := func(w string) *regexp.Regexp {
		return regexp.MustCompile(`(^|[^a-z0-9_-])` + regexp.QuoteMeta(w) + `($|[^a-z0-9_-])`)
	}
	oldRe := word(old)
	res := make([]*regexp.Regexp, len(skills))
	for i, s := range skills {
		res[i] = word(s.Name)
	}
	_ = filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(repo, p)
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(nil, 1<<20)
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			if !oldRe.MatchString(line) {
				continue
			}
			hit := false
			for i, s := range skills {
				if s.Name != old && res[i].MatchString(line) {
					hit = true
					if count[s.Name] == 0 {
						candidates = append(candidates, s.Name)
					}
					count[s.Name]++
				}
			}
			if hit && len(hints) < 5 {
				hints = append(hints, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), n, around(line, oldRe)))
			}
		}
		return nil
	})
	// Most co-mentioned first; stable keeps first-seen order on ties.
	slices.SortStableFunc(candidates, func(a, b string) int { return count[b] - count[a] })
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates, hints
}

// around cuts a long line to a window around the first match of re.
func around(line string, re *regexp.Regexp) string {
	line = strings.Join(strings.Fields(line), " ")
	r := []rune(line)
	if len(r) <= 120 {
		return line
	}
	loc := re.FindStringIndex(line)
	start := len([]rune(line[:loc[0]])) - 50
	start = max(start, 0)
	end := min(start+120, len(r))
	out := string(r[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(r) {
		out += "…"
	}
	return out
}
