package hookdispatch

import (
	"path"
	"strings"
)

// RunsHarnez reports whether a host hook command runs harnez, so apply can
// replace the entries harnez wrote and keep every foreign one. The command is
// split into shell words with POSIX quoting (single quotes, double quotes and
// backslashes). After any leading `env` and VAR=value words, the first word must
// have the base name "harnez". Absolute and relative paths count; harnez-*
// names, wrapper scripts, quoted values that merely contain "harnez", env
// options (`env -i`) and commands with an unterminated quote do not.
func RunsHarnez(command string) bool {
	words, ok := shellWords(command)
	if !ok {
		return false
	}
	i := 0
	if i < len(words) && words[i] == "env" {
		i++
	}
	for i < len(words) && isAssignment(words[i]) {
		i++
	}
	if i >= len(words) {
		return false
	}
	return path.Base(words[i]) == "harnez"
}

// shellWords splits command into words the way a POSIX shell does for the
// quoting that hook commands use. Quotes are removed from the words. It reports
// false for an unterminated quote or a trailing backslash, which the shell would
// reject, so the command is not one harnez can read.
func shellWords(command string) ([]string, bool) {
	var words []string
	var cur strings.Builder
	inWord := false
	flush := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		case r == '\'':
			inWord = true
			end := i + 1
			for end < len(runes) && runes[end] != '\'' {
				end++
			}
			if end >= len(runes) {
				return nil, false
			}
			cur.WriteString(string(runes[i+1 : end]))
			i = end
		case r == '"':
			inWord = true
			i++
			for {
				if i >= len(runes) {
					return nil, false
				}
				c := runes[i]
				if c == '"' {
					break
				}
				if c == '\\' && i+1 < len(runes) && strings.ContainsRune("$`\"\\\n", runes[i+1]) {
					i++
					c = runes[i]
					if c == '\n' {
						// An escaped newline is a line continuation: it adds nothing.
						i++
						continue
					}
				}
				cur.WriteRune(c)
				i++
			}
		case r == '\\':
			if i+1 >= len(runes) {
				return nil, false
			}
			i++
			if runes[i] == '\n' {
				continue
			}
			inWord = true
			cur.WriteRune(runes[i])
		default:
			inWord = true
			cur.WriteRune(r)
		}
	}
	flush()
	return words, true
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
