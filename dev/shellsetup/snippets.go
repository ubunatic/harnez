package shellsetup

import (
	"path/filepath"
	"strings"
)

// pathRef spells dir with $HOME when it lies under home, so the written file
// stays valid for any user who inherits it.
func pathRef(dir, home string) string {
	if home != "" && (dir == home || strings.HasPrefix(dir, home+string(filepath.Separator))) {
		return "$HOME" + dir[len(home):]
	}
	return dir
}

// posixPathBody is the managed bash and zsh body: prepend dir to PATH once.
func posixPathBody(dir, home string) string {
	ref := pathRef(dir, home)
	return "case \":$PATH:\" in\n" +
		"    *\":" + ref + ":\"*) ;;\n" +
		"    *) export PATH=\"" + ref + ":$PATH\" ;;\n" +
		"esac\n"
}

// fishPathContent is the whole fish conf.d file for dir.
func fishPathContent(owner, dir, home string) string {
	ref := pathRef(dir, home)
	return "# Managed by " + owner + " apply: puts the " + owner + " install directory on PATH.\n" +
		"if not contains -- \"" + ref + "\" $PATH\n" +
		"    set -gx PATH \"" + ref + "\" $PATH\n" +
		"end\n"
}

// completionBody is the managed bash or zsh body that sources t's scripts.
// Each script gets its own guarded block. zsh loads compinit only when compdef
// is missing, so a user who already runs compinit is not slowed down.
func completionBody(t completionTarget) string {
	var b strings.Builder
	for _, sc := range t.scripts {
		ref := "\"$HOME/" + sc.rel + "\""
		if t.shell == Zsh {
			b.WriteString("if test -r " + ref + "\n" +
				"then\n" +
				"    (( $+functions[compdef] )) || { autoload -Uz compinit && compinit -i }\n" +
				"    . " + ref + "\n" +
				"fi\n")
			continue
		}
		b.WriteString("if test -r " + ref + "\n" +
			"then . " + ref + "\n" +
			"fi\n")
	}
	return b.String()
}

// dirOnPath reports whether dir is one of the entries of a PATH value.
func dirOnPath(dir, pathValue string) bool {
	for _, entry := range filepath.SplitList(pathValue) {
		if filepath.Clean(entry) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}
