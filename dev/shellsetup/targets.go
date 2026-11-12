package shellsetup

import (
	"os"
	"path/filepath"
)

// pathSection and completionSection name the managed sections in bash and zsh files.
const (
	pathSection       = "path"
	completionSection = "completion"
)

// pathTarget is one startup file that carries the PATH entry.
type pathTarget struct {
	shell Shell
	file  string
	// fish marks the conf.d file, which holds only the PATH line.
	fish bool
	// wanted: the shell is installed or its startup file already exists.
	wanted bool
}

// scriptFile is one command's completion script for one shell.
type scriptFile struct {
	command string
	shell   Shell
	file    string
	// rel is the script's path relative to $HOME, for bash and zsh rc snippets.
	rel    string
	script func(Shell) ([]byte, error)
}

// completionTarget is one shell's completion wiring.
type completionTarget struct {
	shell Shell
	// wanted: the shell is installed or its startup file already exists.
	wanted bool
	// rc is the startup file that sources the scripts. It is "" for fish,
	// which autoloads its scripts.
	rc      string
	scripts []scriptFile
}

// pathTargets lists the PATH file of every configured shell. It returns nil
// without an owner, because the section markers need one.
func (in *Installer) pathTargets() []pathTarget {
	if in.owner == "" {
		return nil
	}
	var out []pathTarget
	for _, s := range in.shells {
		switch s {
		case Zsh:
			f := filepath.Join(in.home, ".zshenv")
			out = append(out, pathTarget{shell: s, file: f, wanted: in.installed(s) || fileExists(f)})
		case Bash:
			f := filepath.Join(in.home, ".bashrc")
			out = append(out, pathTarget{shell: s, file: f, wanted: fileExists(f)})
		case Fish:
			dir := in.fishDir()
			f := filepath.Join(dir, "conf.d", in.owner+"-path.fish")
			out = append(out, pathTarget{shell: s, file: f, fish: true, wanted: in.installed(s) || fileExists(dir)})
		}
	}
	return out
}

// completionTargets lists the completion wiring of every configured shell.
// It returns nil when no completion command is configured.
func (in *Installer) completionTargets() []completionTarget {
	if len(in.completions) == 0 {
		return nil
	}
	var out []completionTarget
	for _, s := range in.shells {
		t := completionTarget{shell: s}
		switch s {
		case Bash, Zsh:
			rcName := ".bashrc"
			if s == Zsh {
				rcName = ".zshrc"
			}
			t.rc = filepath.Join(in.home, rcName)
			t.wanted = fileExists(t.rc)
			for _, c := range in.completions {
				rel := "." + in.owner + "/completions/" + c.Command + "." + string(s)
				t.scripts = append(t.scripts, scriptFile{
					command: c.Command,
					shell:   s,
					file:    filepath.Join(in.home, filepath.FromSlash(rel)),
					rel:     rel,
					script:  c.Script,
				})
			}
		case Fish:
			dir := in.fishDir()
			t.wanted = in.installed(s) || fileExists(dir)
			for _, c := range in.completions {
				t.scripts = append(t.scripts, scriptFile{
					command: c.Command,
					shell:   s,
					file:    filepath.Join(dir, "completions", c.Command+".fish"),
					script:  c.Script,
				})
			}
		}
		out = append(out, t)
	}
	return out
}

// fishDir is fish's configuration directory. Fish reads it from this fixed
// place under $HOME in this package.
func (in *Installer) fishDir() string {
	return filepath.Join(in.home, ".config", "fish")
}

// installed reports whether a shell's binary is on PATH.
func (in *Installer) installed(s Shell) bool {
	_, err := in.lookPath(string(s))
	return err == nil
}

// fileExists reports whether anything, file or directory, exists at path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
