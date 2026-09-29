// Package skillreg manages external (third-party) agent skills (issue 631).
//
// An external skill is a directory with a SKILL.md that lives in some git
// repository. The registry clones the repository into a cache, pins the
// commit, and copies only the skill directory into every agent skill target.
// Plugin extras (hooks, MCP servers, commands, agents) are never installed;
// Explore reports them so the user sees what was left out.
//
// Skills are explicit-only by default: an agent uses one only when the user
// names it. Claude Code gets `disable-model-invocation: true` in its copy,
// Codex gets agents/openai.yaml with allow_implicit_invocation: false (both
// verified 2026-09-29), and agents without such a switch get no copy; they
// reach the skill through `harnez skill show`. InstallOptions.Auto installs a
// plain copy everywhere instead.
//
// Installed copies carry a MarkerFile so Remove and Install never touch
// harnez-managed or hand-installed skills of the same name. `harnez apply`
// only writes its own configured skills and removes only names listed under
// decommissioned.skills, so it leaves registry installs alone.
package skillreg

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// validName keeps a skill name usable as a single path element.
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// MarkerFile marks a skill directory as installed by the external registry.
const MarkerFile = ".harnez-external"

// Entry is one installed external skill in the registry file.
type Entry struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description,omitempty"`
	URL         string    `yaml:"url"`
	Ref         string    `yaml:"ref,omitempty"` // requested ref; empty means default branch
	Commit      string    `yaml:"commit"`
	Path        string    `yaml:"path"`               // skill dir relative to the repo root
	Auto        bool      `yaml:"auto,omitempty"`     // agents may trigger it on their own
	Upstream    string    `yaml:"upstream,omitempty"` // original name when installed with --as
	Warnings    []string  `yaml:"-"`
	Installed   time.Time `yaml:"installed"`
}

type registryFile struct {
	Skills []Entry `yaml:"skills"`
}

// Registry is the on-disk state under Root: registry.yaml plus src/ clones.
type Registry struct {
	Root     string   // e.g. ~/.harnez/skills
	Targets  []Target // agent skill directories
	Reserved []string // names harnez itself manages; never usable by external skills
}

// Target is one agent's skill directory. Agent selects how an explicit-only
// skill is installed there: "claude", "codex", or anything else (no copy).
type Target struct {
	Agent string
	Dir   string
}

// receives reports whether t gets a copy of a skill in the given mode.
func (t Target) receives(auto bool) bool {
	return auto || t.Agent == "claude" || t.Agent == "codex"
}

// Skill is a SKILL.md found in a repository.
type Skill struct {
	Name         string
	Description  string
	AllowedTools string
	Path         string // relative to the repo root
}

// DefaultRoot returns $HARNEZ_SKILLS_HOME or ~/.harnez/skills.
func DefaultRoot() (string, error) {
	if v := os.Getenv("HARNEZ_SKILLS_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".harnez", "skills"), nil
}

func (r *Registry) file() string { return filepath.Join(r.Root, "registry.yaml") }

// SrcDir is the cached clone for a skill at a commit.
func (r *Registry) SrcDir(name, commit string) string {
	return filepath.Join(r.Root, "src", name+"@"+short(commit))
}

// Load reads the registry; a missing file is an empty registry.
func (r *Registry) Load() ([]Entry, error) {
	data, err := os.ReadFile(r.file())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rf registryFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("%s: %w", r.file(), err)
	}
	return rf.Skills, nil
}

func (r *Registry) save(entries []Entry) error {
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	data, err := yaml.Marshal(registryFile{Skills: entries})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.Root, 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.file(), data, 0o644)
}

// Get returns the registry entry for name.
func (r *Registry) Get(name string) (Entry, bool, error) {
	entries, err := r.Load()
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range entries {
		if e.Name == name {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// ParseSource splits "url[@ref]". The ref separator is an "@" after the
// last "/", so "git@host:owner/repo" stays intact.
func ParseSource(src string) (url, ref string) {
	slash := strings.LastIndex(src, "/")
	if at := strings.LastIndex(src, "@"); at > slash && slash >= 0 {
		return src[:at], src[at+1:]
	}
	return src, ""
}

// InstallOptions selects what to install.
type InstallOptions struct {
	URL  string
	Ref  string
	Path string // skill dir inside the repo; needed when the repo has several
	Name string // alternative selector: the skill's frontmatter name
	Auto bool   // let agents trigger the skill on their own
	As   string // install under this name instead of the upstream name
}

// Install clones, pins, and copies one skill into every target.
func (r *Registry) Install(opts InstallOptions) (Entry, error) {
	// Clone under Root so the final rename never crosses filesystems
	// (/tmp is often tmpfs).
	if err := os.MkdirAll(r.Root, 0o755); err != nil {
		return Entry{}, err
	}
	tmp, err := os.MkdirTemp(r.Root, ".tmp-")
	if err != nil {
		return Entry{}, err
	}
	defer os.RemoveAll(tmp)
	clone := filepath.Join(tmp, "repo")
	commit, err := Clone(opts.URL, opts.Ref, clone)
	if err != nil {
		return Entry{}, err
	}
	skills, err := FindSkills(clone)
	if err != nil {
		return Entry{}, err
	}
	skill, err := pickSkill(skills, opts.Path, opts.Name)
	if err != nil {
		return Entry{}, err
	}
	upstream := ""
	if opts.As != "" && opts.As != skill.Name {
		upstream, skill.Name = skill.Name, opts.As
	}
	if !validName.MatchString(skill.Name) || strings.Contains(skill.Name, "..") {
		return Entry{}, fmt.Errorf("skill name %q is not a safe directory name; pick one with --as", skill.Name)
	}
	if slices.Contains(r.Reserved, skill.Name) {
		return Entry{}, fmt.Errorf("%q is reserved for a harnez skill; retry with --as <prefix>-%s", skill.Name, skill.Name)
	}
	var targets []Target // targets that receive a copy
	for _, t := range r.Targets {
		if err := checkTarget(filepath.Join(t.Dir, skill.Name)); err != nil {
			return Entry{}, err
		}
		if t.receives(opts.Auto) {
			targets = append(targets, t)
		}
	}

	entries, err := r.Load()
	if err != nil {
		return Entry{}, err
	}
	var old *Entry
	for i := range entries {
		if entries[i].Name == skill.Name {
			old = &entries[i]
		}
	}
	if old != nil && old.URL != opts.URL {
		return Entry{}, fmt.Errorf("skill %q is already installed from %s; retry with --as <prefix>-%s", skill.Name, old.URL, skill.Name)
	}

	entry := Entry{
		Name: skill.Name, Description: skill.Description,
		URL: opts.URL, Ref: opts.Ref, Commit: commit, Path: skill.Path, Auto: opts.Auto,
		Upstream:  upstream,
		Installed: time.Now().UTC().Truncate(time.Second),
	}
	// Stage every copy (with its marker) beside its target first, so a
	// failed copy never leaves a half-written, marker-less skill behind.
	marker := fmt.Sprintf("url: %s\ncommit: %s\npath: %s\n", entry.URL, entry.Commit, entry.Path)
	var staged []string
	defer func() {
		for _, st := range staged {
			_ = os.RemoveAll(st)
		}
	}()
	for _, t := range targets {
		st := filepath.Join(t.Dir, "."+skill.Name+".harnez-staging")
		staged = append(staged, st)
		if err := os.RemoveAll(st); err != nil {
			return Entry{}, err
		}
		if err := os.MkdirAll(st, 0o755); err != nil {
			return Entry{}, err
		}
		if err := os.WriteFile(filepath.Join(st, MarkerFile), []byte(marker), 0o644); err != nil {
			return Entry{}, err
		}
		if err := copyTree(filepath.Join(clone, skill.Path), st); err != nil {
			return Entry{}, err
		}
		if upstream != "" {
			if err := renameSkill(st, skill.Name); err != nil {
				return Entry{}, err
			}
		}
		if !opts.Auto {
			if err := makeExplicit(t.Agent, st); err != nil {
				return Entry{}, err
			}
		}
	}
	if upstream != "" {
		if refs := mentions(filepath.Join(clone, skill.Path), upstream); len(refs) > 0 {
			entry.Warnings = append(entry.Warnings, fmt.Sprintf(
				"renamed %s -> %s, but these files still mention %q: %s",
				upstream, skill.Name, upstream, strings.Join(refs, ", ")))
		}
	}
	src := r.SrcDir(skill.Name, commit)
	if err := os.RemoveAll(src); err != nil {
		return Entry{}, err
	}
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		return Entry{}, err
	}
	if err := os.Rename(clone, src); err != nil {
		return Entry{}, err
	}
	for _, t := range r.Targets {
		// checkTarget above guarantees any existing dir is ours.
		if err := os.RemoveAll(filepath.Join(t.Dir, skill.Name)); err != nil {
			return Entry{}, err
		}
	}
	for i, t := range targets {
		if err := os.Rename(staged[i], filepath.Join(t.Dir, skill.Name)); err != nil {
			return Entry{}, err
		}
	}
	if old != nil && short(old.Commit) != short(commit) {
		_ = os.RemoveAll(r.SrcDir(old.Name, old.Commit))
	}
	entries = slices.DeleteFunc(entries, func(e Entry) bool { return e.Name == skill.Name })
	entries = append(entries, entry)
	return entry, r.save(entries)
}

// Remove deletes the installed copies (only those with a marker), the cache,
// and the registry entry.
func (r *Registry) Remove(name string) error {
	e, ok, err := r.Get(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("skill %q is not an installed external skill", name)
	}
	for _, t := range r.Targets {
		dst := filepath.Join(t.Dir, name)
		if _, err := os.Stat(filepath.Join(dst, MarkerFile)); err == nil {
			if err := os.RemoveAll(dst); err != nil {
				return err
			}
		}
	}
	if err := os.RemoveAll(r.SrcDir(e.Name, e.Commit)); err != nil {
		return err
	}
	entries, err := r.Load()
	if err != nil {
		return err
	}
	return r.save(slices.DeleteFunc(entries, func(x Entry) bool { return x.Name == name }))
}

// SearchResult is a matching skill, installed or only present in a cached repo.
type SearchResult struct {
	Skill     Skill
	URL       string
	Installed bool
}

// Search matches all query terms (case-insensitive) against name and
// description of installed skills and of sibling skills in cached repos.
func (r *Registry) Search(query string) ([]SearchResult, error) {
	entries, err := r.Load()
	if err != nil {
		return nil, err
	}
	terms := strings.Fields(strings.ToLower(query))
	var out []SearchResult
	seen := map[string]bool{}
	for _, e := range entries {
		skills, err := FindSkills(r.SrcDir(e.Name, e.Commit))
		if err != nil {
			skills = []Skill{{Name: e.Name, Description: e.Description, Path: e.Path}}
		}
		for _, s := range skills {
			key := e.URL + "#" + s.Path
			if seen[key] || !matches(terms, s.Name+" "+s.Description) {
				continue
			}
			seen[key] = true
			out = append(out, SearchResult{Skill: s, URL: e.URL, Installed: s.Path == e.Path})
		}
	}
	return out, nil
}

func matches(terms []string, text string) bool {
	text = strings.ToLower(text)
	for _, t := range terms {
		if !strings.Contains(text, t) {
			return false
		}
	}
	return true
}

// Report is what Explore found in a repository.
type Report struct {
	URL, Commit string
	Skills      []SkillReport
	Plugin      []string // plugin extras that install never copies
	EnvKeys     []string // keys from .env.example files
}

// SkillReport describes one skill directory.
type SkillReport struct {
	Skill
	Files   int
	Scripts []string // files under scripts/
	Doctor  string   // a doctor/preflight script, if any
}

// Explore inspects a repository directory without executing anything from it.
func Explore(repo string) (Report, error) {
	var rep Report
	skills, err := FindSkills(repo)
	if err != nil {
		return rep, err
	}
	for _, s := range skills {
		sr := SkillReport{Skill: s}
		dir := filepath.Join(repo, s.Path)
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			sr.Files++
			rel, _ := filepath.Rel(dir, p)
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "scripts/") {
				sr.Scripts = append(sr.Scripts, rel)
				base := strings.ToLower(filepath.Base(rel))
				if strings.HasPrefix(base, "doctor") || strings.HasPrefix(base, "preflight") {
					sr.Doctor = rel
				}
			}
			return nil
		})
		rep.Skills = append(rep.Skills, sr)
	}
	_ = filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(repo, p)
		rel = filepath.ToSlash(rel)
		switch {
		case d.IsDir() && (d.Name() == "hooks" || d.Name() == "commands" || d.Name() == "agents") && underPlugin(p):
			rep.Plugin = append(rep.Plugin, rel+"/ ("+d.Name()+")")
		case !d.IsDir() && d.Name() == ".mcp.json":
			rep.Plugin = append(rep.Plugin, rel+" (MCP servers)")
		case !d.IsDir() && d.Name() == "plugin.json":
			rep.Plugin = append(rep.Plugin, pluginExtras(p, rel)...)
		case !d.IsDir() && d.Name() == ".env.example":
			rep.EnvKeys = append(rep.EnvKeys, envKeys(p)...)
		}
		return nil
	})
	return rep, nil
}

// underPlugin reports whether dir sits in a Claude Code plugin root.
func underPlugin(dir string) bool {
	_, err := os.Stat(filepath.Join(filepath.Dir(dir), ".claude-plugin", "plugin.json"))
	return err == nil
}

// pluginExtras lists hook/MCP/command/agent keys in a plugin.json. JSON is
// valid YAML, so the yaml parser already in use reads it.
func pluginExtras(path, rel string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]any
	if yaml.Unmarshal(data, &m) != nil {
		return nil
	}
	var out []string
	for _, k := range []string{"hooks", "mcpServers", "commands", "agents"} {
		if _, ok := m[k]; ok {
			out = append(out, rel+" declares "+k)
		}
	}
	return out
}

func envKeys(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var keys []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, _, ok := strings.Cut(line, "="); ok {
			keys = append(keys, strings.TrimSpace(k))
		}
	}
	return keys
}

// FindSkills lists every directory containing SKILL.md, skipping .git.
func FindSkills(repo string) ([]Skill, error) {
	var out []Skill
	err := filepath.WalkDir(repo, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != "SKILL.md" || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		s, err := ReadSkill(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repo, filepath.Dir(p))
		s.Path = filepath.ToSlash(rel)
		out = append(out, s)
		return nil
	})
	return out, err
}

// ReadSkill parses the YAML frontmatter of a SKILL.md.
func ReadSkill(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	var fm struct {
		Name         string `yaml:"name"`
		Description  string `yaml:"description"`
		AllowedTools any    `yaml:"allowed-tools"`
	}
	if rest, ok := bytes.CutPrefix(data, []byte("---\n")); ok {
		if head, _, ok := bytes.Cut(rest, []byte("\n---")); ok {
			if err := yaml.Unmarshal(head, &fm); err != nil {
				return Skill{}, fmt.Errorf("%s: frontmatter: %w", path, err)
			}
		}
	}
	s := Skill{Name: fm.Name, Description: strings.TrimSpace(fm.Description)}
	switch v := fm.AllowedTools.(type) {
	case string:
		s.AllowedTools = v
	case []any:
		parts := make([]string, 0, len(v))
		for _, x := range v {
			parts = append(parts, fmt.Sprint(x))
		}
		s.AllowedTools = strings.Join(parts, ", ")
	}
	if s.Name == "" {
		s.Name = filepath.Base(filepath.Dir(path))
	}
	return s, nil
}

func pickSkill(skills []Skill, path, name string) (Skill, error) {
	if len(skills) == 0 {
		return Skill{}, errors.New("no SKILL.md found in repository")
	}
	var hits []Skill
	for _, s := range skills {
		if (path == "" || s.Path == strings.Trim(path, "/")) && (name == "" || s.Name == name) {
			hits = append(hits, s)
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	var names []string
	for _, s := range skills {
		names = append(names, s.Name+" ("+s.Path+")")
	}
	if len(hits) == 0 {
		return Skill{}, fmt.Errorf("no matching skill; repository has: %s", strings.Join(names, ", "))
	}
	return Skill{}, fmt.Errorf("repository has several skills, pick one with --skill or --path: %s", strings.Join(names, ", "))
}

// makeExplicit switches off implicit use in an agent's copy of a skill.
func makeExplicit(agent, dir string) error {
	switch agent {
	case "claude":
		path := filepath.Join(dir, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, disableModelInvocation(data), 0o644)
	case "codex":
		path := filepath.Join(dir, "agents", "openai.yaml")
		doc := map[string]any{}
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, &doc); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if doc == nil {
				doc = map[string]any{}
			}
		}
		policy, _ := doc["policy"].(map[string]any)
		if policy == nil {
			policy = map[string]any{}
		}
		policy["allow_implicit_invocation"] = false
		doc["policy"] = policy
		out, err := yaml.Marshal(doc)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, out, 0o644)
	}
	return nil
}

// disableModelInvocation sets `disable-model-invocation: true` in the
// SKILL.md frontmatter, adding a frontmatter block if there is none.
func disableModelInvocation(data []byte) []byte {
	return setFrontmatter(data, "disable-model-invocation", "true")
}

// renameSkill sets the frontmatter name of the SKILL.md in dir.
func renameSkill(dir, name string) error {
	path := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, setFrontmatter(data, "name", name), 0o644)
}

// setFrontmatter replaces or appends `key: value` in the YAML frontmatter,
// adding a block if there is none. It matches whole `---` lines and
// normalises CRLF to LF. Only single-line values are replaced cleanly.
func setFrontmatter(data []byte, key, value string) []byte {
	prefix := key + ":"
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	end := -1
	if len(lines) > 0 && lines[0] == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimRight(lines[i], " \t") == "---" {
				end = i
				break
			}
		}
	}
	if end < 0 {
		return []byte("---\n" + prefix + " " + value + "\n---\n" + strings.Join(lines, "\n"))
	}
	// Replace the first key line in place (smaller diffs vs upstream),
	// drop duplicates, append when absent.
	out := []string{"---"}
	set := false
	for _, l := range lines[1:end] {
		if !strings.HasPrefix(l, prefix) {
			out = append(out, l)
		} else if !set {
			out = append(out, prefix+" "+value)
			set = true
		}
	}
	if !set {
		out = append(out, prefix+" "+value)
	}
	out = append(out, lines[end:]...)
	return []byte(strings.Join(out, "\n"))
}

// mentions lists text files in dir that contain word, outside SKILL.md's
// name line. Used to warn that a renamed skill still refers to itself.
func mentions(dir, word string) []string {
	re := regexp.MustCompile(`(^|[^a-z0-9_-])` + regexp.QuoteMeta(word) + `($|[^a-z0-9_-])`)
	nameLine := regexp.MustCompile(`(?m)^name:.*$`)
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		text := string(data)
		if d.Name() == "SKILL.md" {
			text = nameLine.ReplaceAllString(text, "")
		}
		if re.MatchString(text) {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

// checkTarget refuses to overwrite a skill that the registry did not install.
func checkTarget(dst string) error {
	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dst, MarkerFile)); err != nil {
		return fmt.Errorf("%s exists and is not an external skill; refusing to install; retry with --as <prefix>-%s (install checks every agent dir, even ones that get no copy)", dst, filepath.Base(dst))
	}
	return nil
}

// Clone clones url into dst at ref (default branch when empty) and returns
// the full commit hash. Local paths work as url.
func Clone(url, ref, dst string) (string, error) {
	if strings.HasPrefix(url, "-") || strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("refusing source %q@%q: must not start with '-'", url, ref)
	}
	args := []string{"clone", "--quiet"}
	if ref == "" {
		args = append(args, "--depth", "1")
	}
	if err := git("", append(args, "--", url, dst)...); err != nil {
		return "", err
	}
	if ref != "" {
		if err := git(dst, "checkout", "--quiet", "--end-of-options", ref); err != nil {
			return "", err
		}
	}
	out, err := exec.Command("git", "-C", dst, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func git(dir string, args ...string) error {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// copyTree copies regular files and dirs, keeping file modes. Symlinks are
// skipped so a skill cannot point outside its own directory, and .git is
// skipped for skills that sit at the repository root.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
