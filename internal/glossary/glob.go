package glossary

import (
	"path/filepath"
	"regexp"
	"strings"
)

// NormalizeRelPath converts a file path to a clean slash-separated relative path
// without a leading "./".
func NormalizeRelPath(p string) string {
	s := filepath.ToSlash(filepath.Clean(p))
	s = strings.TrimPrefix(s, "./")
	if s == "." {
		return ""
	}
	return s
}

// MatchGlob reports whether path matches the glob pattern.
// Supported wildcards:
//   - "**" matches zero or more directory segments
//   - "*"  matches any sequence of non-slash characters
//   - "?"  matches any single non-slash character
func MatchGlob(pattern, path string) bool {
	pat := NormalizeRelPath(pattern)
	target := NormalizeRelPath(path)
	if pat == "" || target == "" {
		return pat == target
	}

	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pat); {
		if strings.HasPrefix(pat[i:], "**/") {
			b.WriteString("(?:.+/)?")
			i += 3
			continue
		}
		if strings.HasPrefix(pat[i:], "/**") {
			b.WriteString("(?:/.*)?")
			i += 3
			continue
		}
		if strings.HasPrefix(pat[i:], "**") {
			b.WriteString(".*")
			i += 2
			continue
		}
		switch pat[i] {
		case '*':
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '[', ']', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(pat[i])
		default:
			b.WriteByte(pat[i])
		}
		i++
	}
	b.WriteString("$")

	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(target)
}

// EffectiveScope returns the term's scope merged with defaultScope:
// if s defines Include patterns, they override defaultScope.Include; otherwise defaultScope.Include is used.
// Exclude patterns from both defaultScope.Exclude and s.Exclude are combined.
func (s *Scope) EffectiveScope(defaultScope Scope) Scope {
	if s == nil {
		return defaultScope
	}
	out := Scope{
		Include: defaultScope.Include,
		Exclude: append([]string(nil), defaultScope.Exclude...),
	}
	if len(s.Include) > 0 {
		out.Include = s.Include
	}
	if len(s.Exclude) > 0 {
		out.Exclude = append(out.Exclude, s.Exclude...)
	}
	return out
}

// Matches reports whether relPath is included and not excluded by the effective scope.
func (s Scope) Matches(relPath string, defaultScope Scope) bool {
	eff := (&s).EffectiveScope(defaultScope)
	target := NormalizeRelPath(relPath)
	if target == "" {
		return false
	}
	for _, ex := range eff.Exclude {
		if MatchGlob(ex, target) {
			return false
		}
	}
	if len(eff.Include) == 0 {
		return true
	}
	for _, inc := range eff.Include {
		if MatchGlob(inc, target) {
			return true
		}
	}
	return false
}
