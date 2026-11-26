package hookdispatch

import (
	"path"
	"strings"
)

// RunsHarnez reports whether a host hook command runs harnez, so apply can
// replace the entries harnez wrote and keep every foreign one. The first word,
// after any leading `env` and VAR=value prefixes, must be a quoted or bare word
// whose base name is exactly "harnez". Absolute and relative paths count;
// harnez-* names, wrapper scripts and env options (`env -i`) do not.
func RunsHarnez(command string) bool {
	fields := strings.Fields(command)
	i := 0
	if i < len(fields) && fields[i] == "env" {
		i++
	}
	for i < len(fields) && isAssignment(fields[i]) {
		i++
	}
	if i >= len(fields) {
		return false
	}
	word := strings.Trim(fields[i], `"'`)
	return path.Base(word) == "harnez"
}

// isAssignment reports whether word is a shell VAR=value prefix.
func isAssignment(word string) bool {
	eq := strings.IndexByte(word, '=')
	if eq <= 0 {
		return false
	}
	for i, r := range word[:eq] {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
