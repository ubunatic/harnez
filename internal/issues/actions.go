package issues

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DuplicateMatch is the dependency-free projection of a similarity result.
type DuplicateMatch struct {
	Number, RawStatus, PlainTitle string
	Score                         float64
	Strong                        bool
}

// SimilaritySearch is provided by the caller because find owns matching.
type SimilaritySearch func(files []IssueFile, title string) ([]DuplicateMatch, error)

// CheckDuplicates scans existing tickets and applies an injected similarity
// search before building the configured lifecycle-aware duplicate error.
func CheckDuplicates(issuesDir, title string, search SimilaritySearch) error {
	if strings.TrimSpace(title) == "" {
		return nil
	}
	files, err := Scan(issuesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("issues new: duplicate scan: %w", err)
	}
	if search == nil {
		return fmt.Errorf("issues new: duplicate search is not configured")
	}
	matches, err := search(files, title)
	if err != nil {
		return fmt.Errorf("issues new: duplicate scan: %w", err)
	}
	threshold := 0
	if policy, err := DuplicateGateSettings(); err == nil {
		threshold = int(policy.Threshold * 100)
	}
	return DuplicateError(title, matches, threshold)
}

// DuplicateError formats the duplicate gate's actionable error. Similarity
// scoring is injected by the caller so the issue core does not depend on find.
func DuplicateError(title string, matches []DuplicateMatch, thresholdPercent int) error {
	if strings.TrimSpace(title) == "" {
		return nil
	}
	var message strings.Builder
	count := 0
	for _, match := range matches {
		if !match.Strong {
			continue
		}
		verb := "open"
		if CanonicalizeStatus(match.RawStatus) == StatusBacklog {
			verb = "promote"
		}
		if count == 0 {
			fmt.Fprintf(&message, "issues new: likely duplicate ticket(s) found (strong-match threshold %d%%):", thresholdPercent)
		}
		fmt.Fprintf(&message, "\n  #%s %s — %s (score %.3f)\n    harnez issues %s %s", match.Number, match.RawStatus, match.PlainTitle, match.Score, verb, match.Number)
		count++
	}
	if count == 0 {
		return nil
	}
	message.WriteString("\nUse --force to create anyway.")
	return errors.New(message.String())
}

// ComposeStatus renders the status text produced by an issues lifecycle verb.
func ComposeStatus(verb, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	statusFor := func(key string) (string, error) {
		label, ok := StatusLabel(key)
		if !ok {
			return "", fmt.Errorf("issues: missing lifecycle label for %q", key)
		}
		if reason == "" {
			return label, nil
		}
		return label + " — " + reason, nil
	}
	switch verb {
	case "open":
		return statusFor("open")
	case "start":
		return statusFor("in-progress")
	case "block":
		if reason == "" {
			return "", fmt.Errorf(`issues block: reason is required, e.g. 'harnez issues block <number> "waiting on upstream fix"'`)
		}
		return statusFor("blocked")
	case "close", "done":
		return statusFor("closed")
	case "draft":
		return statusFor("draft")
	case "backlog":
		return statusFor("backlog")
	case "promote":
		if reason != "" {
			return "", fmt.Errorf("issues promote: does not accept a reason")
		}
		return statusFor("open")
	default:
		return "", fmt.Errorf("issues: unknown verb %q (expected one of: open, start, block, close, done, draft, backlog, promote)", verb)
	}
}

// ValidateLifecycleTransition rejects lifecycle changes that violate the
// Backlog/Closed transition rules.
func ValidateLifecycleTransition(verb, oldStatus string) error {
	switch verb {
	case "backlog":
		if CanonicalizeStatus(oldStatus) == StatusClosed {
			return fmt.Errorf("issues backlog: cannot move a Closed ticket to Backlog")
		}
	case "promote":
		if CanonicalizeStatus(oldStatus) != StatusBacklog {
			return fmt.Errorf("issues promote: ticket must be Backlog")
		}
	}
	return nil
}

// CloseStatus preserves an existing closure unless explicitly amended.
func CloseStatus(verb, oldStatus, newStatus string, amend bool) (string, error) {
	closing := verb == "close" || verb == "done"
	closed := CanonicalizeStatus(oldStatus) == StatusClosed
	if amend && (!closing || !closed) {
		return "", fmt.Errorf("--amend-reason requires close/done on an already Closed ticket")
	}
	if closing && closed && !amend {
		return oldStatus, nil
	}
	return newStatus, nil
}

// MessageReferencesTicket reports whether a Git subject contains the ticket
// number as a numeric token, ignoring leading zeroes.
func MessageReferencesTicket(message, ticketNumber string) bool {
	for _, field := range strings.FieldsFunc(message, func(r rune) bool { return r < '0' || r > '9' }) {
		if strings.TrimLeft(field, "0") == strings.TrimLeft(ticketNumber, "0") {
			return true
		}
	}
	return false
}

// FindTicketFile resolves a numeric ID to its unique issue file.
func FindTicketFile(issuesDir, ticketArg string) (IssueFile, error) {
	ticketArg = strings.TrimSpace(ticketArg)
	files, err := Scan(issuesDir)
	if err != nil {
		return IssueFile{}, fmt.Errorf("issues: %w", err)
	}
	ticketNum, idErr := NormalizeID(ticketArg)
	if idErr == nil {
		var matches []IssueFile
		for _, f := range files {
			if f.Number == ticketNum {
				matches = append(matches, f)
			}
		}
		switch len(matches) {
		case 0:
			return IssueFile{}, fmt.Errorf("issues: no ticket found for number %q under %s (checked issues/*.md and issues/archive/*.md)", ticketNum, issuesDir)
		case 1:
			return matches[0], nil
		default:
			paths := make([]string, len(matches))
			for i := range matches {
				paths[i] = matches[i].RelPath
			}
			return IssueFile{}, fmt.Errorf("issues: ambiguous ticket number %q matches multiple files: %s", ticketNum, strings.Join(paths, ", "))
		}
	}
	cleaned := strings.TrimPrefix(filepath.ToSlash(ticketArg), "issues/")
	var matches []IssueFile
	for _, f := range files {
		base := filepath.Base(f.RelPath)
		slug := strings.TrimSuffix(base, ".md")
		if f.RelPath == cleaned || base == cleaned || slug == cleaned {
			matches = append(matches, f)
		}
	}
	switch len(matches) {
	case 0:
		return IssueFile{}, fmt.Errorf("issues: no ticket found for %q under %s (checked issues/*.md and issues/archive/*.md)", ticketArg, issuesDir)
	case 1:
		return matches[0], nil
	default:
		paths := make([]string, len(matches))
		for i := range matches {
			paths[i] = matches[i].RelPath
		}
		return IssueFile{}, fmt.Errorf("issues: ambiguous ticket %q matches multiple files: %s", ticketArg, strings.Join(paths, ", "))
	}
}
