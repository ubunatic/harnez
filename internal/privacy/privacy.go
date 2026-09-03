// Package privacy defines the shared privacy-level vocabulary and text
// scrubbing helpers used by `harnez usage export` (issue 204). It is
// deliberately independent of both internal/telemetry and internal/usage
// (no imports either way) so both can depend on it without introducing a
// cycle; the actual export-shape wiring (which fields survive at which
// level) lives in each of those packages' own export.go.
package privacy

import (
	"fmt"
	"regexp"
)

// Level is one of the four privacy tiers from issue 204 section 2
// ("Proposed Solution & Architecture").
type Level int

const (
	// LevelPublic ("public", the default) drops all free-text fields
	// (notes, Sources, Details) entirely and reduces structural
	// identifiers (paths, accounts, hostnames) to non-identifying forms.
	// This is the exact behavior `harnez usage export` always had before
	// the --privacy flag existed — see export.go's "Level" doc comments
	// in internal/telemetry and internal/usage for the field-by-field
	// mapping.
	LevelPublic Level = iota
	// LevelAgentSanitized ("agent-sanitized") keeps free-text notes but
	// rewrites them through a NoteSanitizer (see sanitizer.go) into a
	// high-level summary before they leave the machine. Only
	// internal/telemetry's tool_calls.note field has a sanitizer wired to
	// it (issue 204's LLM-in-the-loop pipeline targets that field
	// specifically); internal/usage has no equivalent "note" concept, so
	// at this level its Sources/Details fields fall back to the same
	// regex-scrubbed behavior as LevelInternal rather than being sent to
	// an LLM — see usage/export.go's BuildUsageExportLevel doc comment.
	LevelAgentSanitized
	// LevelInternal ("internal") keeps free-text/structural fields but
	// runs them through ScrubText first, replacing obvious sensitive
	// substrings (home paths, emails, API-key-shaped tokens) in place
	// rather than dropping the whole field.
	LevelInternal
	// LevelRaw ("raw") exports every field completely unscrubbed. Local/
	// private use only — never intended for anything published externally.
	LevelRaw
)

// String renders the level the same way ParseLevel parses it, so
// round-tripping through --privacy=<Level.String()> always works.
func (l Level) String() string {
	switch l {
	case LevelPublic:
		return "public"
	case LevelAgentSanitized:
		return "agent-sanitized"
	case LevelInternal:
		return "internal"
	case LevelRaw:
		return "raw"
	default:
		return fmt.Sprintf("privacy.Level(%d)", int(l))
	}
}

// ParseLevel parses a --privacy flag value. An empty string parses as
// LevelPublic so callers that never pass --privacy at all get the
// original (pre-issue-204-v2) default behavior without special-casing it.
func ParseLevel(s string) (Level, error) {
	switch s {
	case "", "public":
		return LevelPublic, nil
	case "agent-sanitized":
		return LevelAgentSanitized, nil
	case "internal":
		return LevelInternal, nil
	case "raw":
		return LevelRaw, nil
	default:
		return LevelPublic, fmt.Errorf("privacy: unknown level %q (want one of: public, agent-sanitized, internal, raw)", s)
	}
}

// Regexes backing ScrubText. Compiled once at package init rather than per
// call.
var (
	// homePathRe matches an absolute home-directory path's leading
	// "/home/<user>" or "/Users/<user>" segment (issue 204's explicit
	// example), leaving the rest of the path (which is usually a project
	// or file name, not identifying on its own) intact after the "~"
	// swap.
	homePathRe = regexp.MustCompile(`(?:/home/|/Users/)[^/\s"'` + "`" + `]+`)
	// emailRe matches a standard local@domain.tld shape.
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// tokenRe matches common API-key/token shapes: well-known vendor
	// prefixes (sk-, ghp_, gho_, xox*-, AKIA...) plus a generic fallback
	// of any long (32+) run of token-ish characters, which catches
	// arbitrary hex/base64-ish secrets that don't carry a recognizable
	// prefix at the cost of also matching some long non-secret
	// identifiers (e.g. git SHAs) — an acceptable false-positive for a
	// scrub-in-place pass whose failure mode (over-redaction) is far
	// safer than a leaked secret.
	tokenRe = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{10,}|gh[po]_[A-Za-z0-9]{10,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{12,}|[A-Za-z0-9_-]{32,})\b`)
)

// ScrubText redacts obvious sensitive substrings in s in place, without
// dropping the field entirely — this is LevelInternal's behavior, as
// opposed to LevelPublic's "drop the whole field" and LevelAgentSanitized's
// LLM-rewrite. Order matters: home paths first (so a path containing what
// looks like a long token isn't independently token-redacted mid-path),
// then emails, then generic tokens.
func ScrubText(s string) string {
	if s == "" {
		return s
	}
	s = homePathRe.ReplaceAllString(s, "~")
	s = emailRe.ReplaceAllString(s, "[redacted-email]")
	s = tokenRe.ReplaceAllString(s, "[redacted-token]")
	return s
}

// ScrubStrings applies ScrubText to every element of a slice, returning a
// new slice (ss is not mutated in place).
func ScrubStrings(ss []string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = ScrubText(s)
	}
	return out
}

// ScrubMap applies ScrubText to every value (not key) of a map, returning
// a new map (m is not mutated in place).
func ScrubMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = ScrubText(v)
	}
	return out
}
