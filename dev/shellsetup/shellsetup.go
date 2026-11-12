// Package shellsetup puts a command's directory on PATH and installs the
// command's shell completion into the user's shell startup files.
//
// The caller sets what to install (Completions, PathDir) and which shells to
// wire (Shells). Apply writes it, Checks and Pending report drift, and Clean
// removes it again. The package uses only the standard library, so it can move
// out of harnez without change.
//
// Startup files, per shell:
//
//   - bash: ~/.bashrc carries the PATH section and the completion section. Both
//     are written only when ~/.bashrc already exists.
//   - zsh: ~/.zshenv carries the PATH section (zsh reads it for every
//     invocation). ~/.zshrc carries the completion section and is written only
//     when it already exists. The completion section runs compinit when
//     compdef is missing.
//   - fish: a conf.d file carries the PATH line, and the completion script is
//     autoloaded from ~/.config/fish/completions. Both files are owned outright.
//
// Completion scripts for bash and zsh live in ~/.<owner>/completions/ and are
// sourced from the rc file. Apply writes them once, so a new shell starts no
// process. Re-run Apply after upgrading the command, because the scripts are
// not regenerated at shell start.
package shellsetup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Shell names a shell the installer can wire.
type Shell string

// The shells the installer knows. Config.Shells selects among them.
const (
	Bash Shell = "bash"
	Zsh  Shell = "zsh"
	Fish Shell = "fish"
)

// Feature says which part of the setup a Change or Check belongs to.
type Feature string

// The two parts of the setup.
const (
	FeaturePath       Feature = "PATH"
	FeatureCompletion Feature = "completion"
)

// Completion is one command's shell completion.
type Completion struct {
	// Command is the executable name, such as "harnez". It names the script files.
	Command string
	// Script renders the completion script for shell. Apply and Checks call it,
	// so it must be deterministic.
	Script func(shell Shell) ([]byte, error)
}

// Config says what the installer manages. Only Completions and PathDir have
// no default: an empty Completions skips completion, and an empty PathDir
// skips PATH.
type Config struct {
	// Home is the user's home directory. Empty means os.UserHomeDir.
	Home string
	// Owner tags the managed sections ("# Owner:begin path") and names the
	// ~/.Owner/completions directory. Empty means the first completion's
	// command. It is required when PathDir is set and Completions is empty.
	Owner string
	// PathDir is the directory to put on PATH. Empty skips PATH.
	PathDir string
	// Completions are the commands to complete. Empty skips completion.
	Completions []Completion
	// Shells selects the shells to wire. Empty means Bash, Zsh and Fish.
	Shells []Shell
}

// Installer writes, checks and removes the setup described by one Config.
type Installer struct {
	home        string
	owner       string
	pathDir     string
	completions []Completion
	shells      []Shell
	// lookPath reports whether a shell is installed. Tests replace it.
	lookPath func(string) (string, error)
}

// Result reports what Apply did.
type Result struct {
	// Changes lists every file or section that Apply wrote or updated.
	Changes []Change
	// Unchanged lists the files and sections that were already up to date.
	// Their Action is empty.
	Unchanged []Change
	// PathMissing is true when PathDir is set but is not on this process's PATH.
	PathMissing bool
}

// Change is one file or managed section.
type Change struct {
	Feature Feature
	// Action is "wrote", "updated", "removed" or "cleaned".
	Action string
	// Path is the absolute path of the file.
	Path string
	// Section names the managed section inside Path. It is empty for files the
	// installer owns outright.
	Section string
}

// Check is one drift row for status output.
type Check struct {
	// Label is the display path and its feature, such as "~/.zshenv [PATH ~/go/bin]".
	Label string
	// State is "ok", "missing" or "drifted".
	State string
}

// New validates cfg and returns an installer for it. It does not touch the file system.
func New(cfg Config) (*Installer, error) {
	home := cfg.Home
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("shellsetup: home directory: %w", err)
		}
		home = h
	}
	owner := cfg.Owner
	if owner == "" && len(cfg.Completions) > 0 {
		owner = cfg.Completions[0].Command
	}
	if cfg.PathDir != "" && owner == "" {
		return nil, errors.New("shellsetup: Owner is required when PathDir is set without completions")
	}
	if owner != "" && !validName(owner) {
		return nil, fmt.Errorf("shellsetup: invalid owner %q", owner)
	}

	shells := cfg.Shells
	if len(shells) == 0 {
		shells = []Shell{Bash, Zsh, Fish}
	}
	seenShell := map[Shell]bool{}
	for _, s := range shells {
		if s != Bash && s != Zsh && s != Fish {
			return nil, fmt.Errorf("shellsetup: unsupported shell %q", s)
		}
		if seenShell[s] {
			return nil, fmt.Errorf("shellsetup: shell %q listed twice", s)
		}
		seenShell[s] = true
	}

	seenCmd := map[string]bool{}
	for _, c := range cfg.Completions {
		if !validName(c.Command) {
			return nil, fmt.Errorf("shellsetup: invalid completion command %q", c.Command)
		}
		if seenCmd[c.Command] {
			return nil, fmt.Errorf("shellsetup: completion command %q listed twice", c.Command)
		}
		if c.Script == nil {
			return nil, fmt.Errorf("shellsetup: completion command %q has no Script", c.Command)
		}
		seenCmd[c.Command] = true
	}

	return &Installer{
		home:        home,
		owner:       owner,
		pathDir:     cfg.PathDir,
		completions: cfg.Completions,
		shells:      shells,
		lookPath:    exec.LookPath,
	}, nil
}

// Apply writes the PATH entry and completion wiring for every wanted shell.
// A shell is wanted when it is installed or its startup file already exists,
// and bash and zsh rc files are never created.
func (in *Installer) Apply() (Result, error) {
	var res Result
	if in.pathDir != "" {
		for _, t := range in.pathTargets() {
			if !t.wanted {
				continue
			}
			if err := in.applyPath(&res, t); err != nil {
				return res, err
			}
		}
		res.PathMissing = !dirOnPath(in.pathDir, os.Getenv("PATH"))
	}
	for _, t := range in.completionTargets() {
		if !t.wanted {
			continue
		}
		if err := in.applyCompletion(&res, t); err != nil {
			return res, err
		}
	}
	return res, nil
}

// Checks compares every wanted file and section with what Apply would write.
func (in *Installer) Checks() []Check {
	var out []Check
	if in.pathDir != "" {
		for _, t := range in.pathTargets() {
			if !t.wanted {
				continue
			}
			out = append(out, Check{
				Label: in.display(t.file) + " [PATH " + in.display(in.pathDir) + "]",
				State: in.pathState(t),
			})
		}
	}
	for _, t := range in.completionTargets() {
		if !t.wanted {
			continue
		}
		for _, sc := range t.scripts {
			state := scriptState(sc)
			if state == "ok" && t.rc != "" {
				state = sectionState(t.rc, in.owner, completionSection, completionBody(t))
			}
			out = append(out, Check{
				Label: in.display(sc.file) + " [completion " + string(t.shell) + "]",
				State: state,
			})
		}
	}
	return out
}

// Pending reports whether any wanted file or section is missing or drifted.
func (in *Installer) Pending() bool {
	for _, c := range in.Checks() {
		if c.State != "ok" {
			return true
		}
	}
	return false
}

// Clean removes what this Config would write: the PATH entries, the completion
// scripts and the managed sections. It covers every configured shell, whether
// or not the shell is still installed. It returns the changes it made, and
// keeps going after an error so one stuck file does not block the rest.
func (in *Installer) Clean() ([]Change, error) {
	var changes []Change
	var errs []error
	for _, t := range in.pathTargets() {
		if t.fish {
			err := os.Remove(t.file)
			switch {
			case err == nil:
				changes = append(changes, Change{Feature: FeaturePath, Action: "removed", Path: t.file})
			case !os.IsNotExist(err):
				errs = append(errs, err)
			}
			continue
		}
		action, err := cleanSection(t.file, in.owner, pathSection)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if action != "" {
			changes = append(changes, Change{Feature: FeaturePath, Action: action, Path: t.file, Section: pathSection})
		}
	}
	for _, t := range in.completionTargets() {
		for _, sc := range t.scripts {
			err := os.Remove(sc.file)
			switch {
			case err == nil:
				changes = append(changes, Change{Feature: FeatureCompletion, Action: "removed", Path: sc.file})
			case !os.IsNotExist(err):
				errs = append(errs, err)
			}
		}
		if t.rc == "" {
			continue
		}
		action, err := cleanSection(t.rc, in.owner, completionSection)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if action != "" {
			changes = append(changes, Change{Feature: FeatureCompletion, Action: action, Path: t.rc, Section: completionSection})
		}
	}
	// Remove the harnez-owned completion directories when they are now empty.
	// Fish's completions directory belongs to the user, so it is left alone.
	if len(in.completions) > 0 && in.owner != "" {
		ownDir := filepath.Join(in.home, "."+in.owner)
		_ = os.Remove(filepath.Join(ownDir, "completions"))
		_ = os.Remove(ownDir)
	}
	return changes, errors.Join(errs...)
}

func (in *Installer) applyPath(res *Result, t pathTarget) error {
	if t.fish {
		changed, err := writeIfChanged(t.file, []byte(fishPathContent(in.owner, in.pathDir, in.home)))
		if err != nil {
			return fmt.Errorf("fish PATH %s: %w", t.file, err)
		}
		res.record(changed, FeaturePath, "wrote", t.file, "")
		return nil
	}
	changed, existed, err := applySection(t.file, in.owner, pathSection, posixPathBody(in.pathDir, in.home))
	if err != nil {
		return fmt.Errorf("PATH section %s: %w", t.file, err)
	}
	res.record(changed, FeaturePath, verbFor(existed), t.file, pathSection)
	return nil
}

func (in *Installer) applyCompletion(res *Result, t completionTarget) error {
	for _, sc := range t.scripts {
		script, err := sc.script(t.shell)
		if err != nil {
			return fmt.Errorf("%s completion script for %s: %w", t.shell, sc.command, err)
		}
		changed, err := writeIfChanged(sc.file, script)
		if err != nil {
			return fmt.Errorf("%s completion %s: %w", t.shell, sc.file, err)
		}
		res.record(changed, FeatureCompletion, "wrote", sc.file, "")
	}
	if t.rc == "" {
		return nil
	}
	changed, existed, err := applySection(t.rc, in.owner, completionSection, completionBody(t))
	if err != nil {
		return fmt.Errorf("completion section %s: %w", t.rc, err)
	}
	res.record(changed, FeatureCompletion, verbFor(existed), t.rc, completionSection)
	return nil
}

func (in *Installer) pathState(t pathTarget) string {
	if t.fish {
		data, err := os.ReadFile(t.file)
		if err != nil {
			return "missing"
		}
		if string(data) == fishPathContent(in.owner, in.pathDir, in.home) {
			return "ok"
		}
		return "drifted"
	}
	return sectionState(t.file, in.owner, pathSection, posixPathBody(in.pathDir, in.home))
}

// scriptState compares one completion script with what its renderer produces.
func scriptState(sc scriptFile) string {
	want, err := sc.script(sc.shell)
	if err != nil {
		return "drifted"
	}
	data, err := os.ReadFile(sc.file)
	if err != nil {
		return "missing"
	}
	if !bytes.Equal(data, want) {
		return "drifted"
	}
	return "ok"
}

// record files one outcome under changes or unchanged.
func (r *Result) record(changed bool, f Feature, verb, path, section string) {
	if changed {
		r.Changes = append(r.Changes, Change{Feature: f, Action: verb, Path: path, Section: section})
		return
	}
	r.Unchanged = append(r.Unchanged, Change{Feature: f, Path: path, Section: section})
}

// verbFor names a section write: "updated" when the section already existed.
func verbFor(existed bool) string {
	if existed {
		return "updated"
	}
	return "wrote"
}

// display shortens a path under the home directory to ~/.
func (in *Installer) display(p string) string {
	if in.home != "" && strings.HasPrefix(p, in.home+string(filepath.Separator)) {
		return "~" + p[len(in.home):]
	}
	return p
}
