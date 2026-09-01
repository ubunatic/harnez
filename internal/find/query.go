// Package find implements the query grammar and fuzzy ranking behind
// `harnez find <entity> <query...>` (issue 158). It is intentionally kept
// independent of Cobra and of internal/issues' file scanning so the grammar,
// evaluator, and ranking can be unit tested in isolation from I/O.
package find

import (
	"fmt"
	"strings"
)

// StatusValue is one of the five query-spellings accepted by a status/is
// filter. Only these exact spellings are accepted; matching against a
// ticket's raw status text is case-insensitive and suffix-tolerant, but the
// query vocabulary itself is fixed (issue 158).
type StatusValue string

const (
	StatusOpenValue       StatusValue = "open"
	StatusInProgressValue StatusValue = "in-progress"
	StatusBlockedValue    StatusValue = "blocked"
	StatusClosedValue     StatusValue = "closed"
	StatusDraftValue      StatusValue = "draft"
)

// validStatusValues lists the accepted query spellings, in the order they
// should appear in an error hint.
var validStatusValues = []StatusValue{
	StatusOpenValue, StatusInProgressValue, StatusBlockedValue, StatusClosedValue, StatusDraftValue,
}

// Filter is a field:value term, ANDed with every other term in the query.
// Version one supports only the status field (with alias "is").
type Filter struct {
	Value StatusValue
}

// Group is one whitespace-separated AND term of the query, holding its bare
// text alternatives. A single-element Group is a plain term; a multi-element
// Group came from a "a|b|c" OR term.
type Group struct {
	Alternatives []string
}

// Query is a fully parsed `harnez find` query: every Filter and every Group
// must match (AND) for an issue to be a result; within a Group, any one
// Alternative matching is enough (OR).
type Query struct {
	Filters []Filter
	Groups  []Group
}

// ParseQuery parses the raw, whitespace-joined query text produced by
// joining a Cobra command's trailing args with a single space. It returns a
// usage error, with an actionable hint, for every malformed-query case
// specified by issue 158: a missing/whitespace-only query, a standalone,
// leading, trailing, or doubled '|', a '|' inside a field filter,
// parentheses, literal quote characters, leading-'-'/'!' negation syntax, an
// empty filter value, an unknown field, or an unsupported status value.
func ParseQuery(raw string) (*Query, error) {
	terms := strings.Fields(raw)
	if len(terms) == 0 {
		return nil, fmt.Errorf("find: query must not be empty")
	}

	q := &Query{}
	for _, term := range terms {
		if strings.ContainsAny(term, `()"'`) {
			return nil, fmt.Errorf("find: query term %q uses unsupported syntax (parentheses/quotes are not supported; regex-looking punctuation is treated as ordinary text, not executed)", term)
		}
		if strings.HasPrefix(term, "-") || strings.HasPrefix(term, "!") {
			return nil, fmt.Errorf("find: query term %q looks like negation, which v1 does not support", term)
		}

		if field, value, ok := splitFilter(term); ok {
			filter, err := parseFilter(field, value)
			if err != nil {
				return nil, err
			}
			q.Filters = append(q.Filters, filter)
			continue
		}

		group, err := parseGroup(term)
		if err != nil {
			return nil, err
		}
		q.Groups = append(q.Groups, group)
	}

	return q, nil
}

// splitFilter reports whether term looks like a "field:value" token: an
// alphabetic field name immediately followed by ':'. Terms without a colon,
// or with a non-alphabetic prefix, are treated as plain text so incidental
// colons in ordinary search text never get parsed as filters.
func splitFilter(term string) (field, value string, ok bool) {
	idx := strings.Index(term, ":")
	if idx <= 0 {
		return "", "", false
	}
	field = term[:idx]
	for _, r := range field {
		if !isASCIILetter(r) {
			return "", "", false
		}
	}
	return field, term[idx+1:], true
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func parseFilter(field, value string) (Filter, error) {
	switch strings.ToLower(field) {
	case "status", "is":
	default:
		return Filter{}, fmt.Errorf("find: unknown filter field %q (supported: status, is)", field)
	}

	if strings.Contains(value, "|") {
		return Filter{}, fmt.Errorf("find: filter %q cannot use '|' alternatives; filters do not support OR", field+":"+value)
	}
	if value == "" {
		return Filter{}, fmt.Errorf("find: filter %q has no value (accepted values: open, in-progress, blocked, closed, draft)", field+":")
	}

	sv := StatusValue(strings.ToLower(value))
	for _, v := range validStatusValues {
		if sv == v {
			return Filter{Value: sv}, nil
		}
	}
	return Filter{}, fmt.Errorf("find: unsupported status value %q (accepted values: open, in-progress, blocked, closed, draft)", value)
}

// parseGroup splits a whitespace-delimited term on '|' into its OR
// alternatives, rejecting any empty alternative -- which covers a standalone
// '|', a leading or trailing '|', and a doubled '|' ("a||b") in one check.
func parseGroup(term string) (Group, error) {
	alts := strings.Split(term, "|")
	for _, a := range alts {
		if a == "" {
			return Group{}, fmt.Errorf("find: query term %q has an empty '|' alternative (no standalone, leading, trailing, or doubled '|')", term)
		}
	}
	return Group{Alternatives: alts}, nil
}
