// toolname.go groups tool_calls rows whose tool_name is (or contains) a
// full shell command into one bucket per leading command word, so
// aggregations (harnez stats, `harnez usage export` and everything
// downstream of it, e.g. the ubunatic.com dashboard) don't fragment "git"
// usage across "git", "git status", "git status --short", "git diff", etc.
//
// Two related but distinct problems live here (issue 344's follow-up):
//   - ShellMetacharacters/ShellKeywords are the shared boundary set also
//     used by cmd/harnez/exec.go's inferToolFromArgs and
//     isSimpleShellCommand at *capture* time, to stop a single-token scan
//     at the first operator/keyword instead of returning it as a fake tool
//     name (e.g. "2>/dev/null" -> "null").
//   - CanonicalToolName is a *read-time* grouping step for tool_name values
//     already sitting in the database, some of which (predating today's
//     capture logic, or from an older code path — see issue 344's
//     Resolution) are full multi-word command strings rather than a single
//     token. It cannot recover data that was never captured, but it can
//     still group "git status --short" under "git" for display purposes
//     without needing a destructive DB rewrite.
package telemetry

import (
	"path/filepath"
	"strings"
)

// ShellMetacharacters are the control-operator/substitution characters that
// make a token (or a whole command string) shell-structural rather than a
// plain command word — a redirect target ("2>/dev/null"), an operator
// ("&&", "||"), a pipe, a subshell, etc.
var ShellMetacharacters = []string{"\n", "\r", ";", "&", "|", "<", ">", "$", "`", "(", ")", "{", "}"}

// ShellKeywords are shell keywords/builtins with no independent binary on
// PATH — never a real "tool" on their own.
var ShellKeywords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true,
	"case": true, "esac": true, "for": true, "while": true, "until": true,
	"do": true, "done": true, "in": true, "select": true, "time": true,
	"function": true, "export": true, "set": true, "unset": true,
	"alias": true, "unalias": true, "source": true, ".": true, "eval": true,
	"exec": true, "trap": true, "return": true, "exit": true,
	"builtin": true, "command": true, "shopt": true, "cd": true,
	"read": true, "pushd": true, "popd": true, "dirs": true,
	"declare": true, "typeset": true, "local": true, "readonly": true,
	"type": true, "ulimit": true, "umask": true, "disown": true,
	"jobs": true, "bg": true, "fg": true, "wait": true, "!": true,
	"[[": true, "]]": true,
}

// LooksLikeDataFile reports whether base looks like a plain data/document
// filename or glob rather than an executable command word.
func LooksLikeDataFile(base string) bool {
	if strings.ContainsAny(base, "*?[") {
		return true
	}
	switch filepath.Ext(base) {
	case ".md", ".txt", ".log", ".lock", ".json", ".yaml", ".yml", ".gz", ".tar", ".zip", ".csv":
		return true
	}
	return false
}

// CanonicalToolName groups an already-recorded tool_name down to its
// leading command word when it looks like a full shell command rather than
// a single tool identifier. A plain single-token name (the overwhelming
// majority: "Bash", "Edit", "git", "heartbeat", ...) passes through
// unchanged — this only rewrites values containing whitespace or a shell
// metacharacter. Falls back to "Bash" when no clean leading command word
// can be identified, mirroring cmd/harnez/exec.go's inferToolFromArgs.
func CanonicalToolName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	hasMeta := false
	for _, meta := range ShellMetacharacters {
		if strings.Contains(trimmed, meta) {
			hasMeta = true
			break
		}
	}
	fields := strings.Fields(trimmed)
	if !hasMeta && len(fields) <= 1 && !ShellKeywords[trimmed] && !LooksLikeDataFile(stripQuotes(trimmed)) {
		return trimmed
	}

	fallback := "Bash"
	skipNext := false
	for _, tok := range fields {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.Contains(tok, "=") && !strings.HasPrefix(tok, "-") {
			continue // skip VAR=val
		}
		if strings.HasPrefix(tok, "-") {
			if tok == "-u" || tok == "-g" || tok == "-C" || tok == "-w" || tok == "--user" || tok == "--group" || tok == "--directory" {
				skipNext = true
			}
			continue
		}
		for _, meta := range ShellMetacharacters {
			if strings.Contains(tok, meta) {
				return fallback
			}
		}
		if ShellKeywords[tok] {
			return fallback
		}
		base := filepath.Base(stripQuotes(tok))
		if base == "" || base == "bash" || base == "sh" || base == "sudo" || base == "env" || base == "doas" || base == "nohup" {
			continue
		}
		if LooksLikeDataFile(base) {
			return fallback
		}
		return base
	}
	return fallback
}

// stripQuotes trims a single matching pair of leading/trailing quote
// characters. A stray, unmatched quote (e.g. from a mis-escaped --tool
// value) is common corruption in captured tool_name strings — it should
// not defeat the extension/glob checks in LooksLikeDataFile.
func stripQuotes(s string) string {
	return strings.Trim(s, `"'`)
}
