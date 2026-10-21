package find

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"ubunatic.com/harnez/internal/issues"
)

// ErrTooFewSimilarityTokens means a proposed title contains too little
// meaningful text for a duplicate check to be useful.
var ErrTooFewSimilarityTokens = errors.New("too few meaningful similarity tokens")

// SimilarResult is a candidate ticket ranked for a proposed issue title.
// Score is in [0,1], and Strong is determined by the spec threshold.
type SimilarResult struct {
	Number     string  `json:"number"`
	RawStatus  string  `json:"raw_status"`
	PlainTitle string  `json:"title"`
	Path       string  `json:"path"`
	Score      float64 `json:"score"`
	Strong     bool    `json:"strong"`

	titleMatches int
	matched      int
}

// Similar ranks every ticket by independent query-token matches. A query
// token can match the title or body; title matches receive the larger
// spec-defined weight. Tickets are not filtered by lifecycle status.
func Similar(files []issues.IssueFile, query string) ([]SimilarResult, error) {
	policy, err := issues.DuplicateGateSettings()
	if err != nil {
		return nil, err
	}
	stopwords := make(map[string]bool, len(policy.Stopwords))
	for _, word := range policy.Stopwords {
		stopwords[strings.ToLower(word)] = true
	}
	seen := make(map[string]bool)
	var tokens []string
	for _, token := range strings.Fields(Normalize(query)) {
		if utf8.RuneCountInString(token) < policy.MinimumTokenLength || stopwords[token] || seen[token] {
			continue
		}
		seen[token] = true
		tokens = append(tokens, token)
	}
	if len(tokens) < policy.MinimumQueryTokens {
		return nil, fmt.Errorf("find issues similar: title needs at least %d meaningful words after stopwords: %w", policy.MinimumQueryTokens, ErrTooFewSimilarityTokens)
	}

	denominator := float64(len(tokens) * policy.TitleWeight)
	results := make([]SimilarResult, 0, len(files))
	for _, file := range files {
		title := Normalize(issues.StripTicketNumber(file.Title))
		body := Normalize(file.Body)
		weight := 0
		titleMatches := 0
		matched := 0
		for _, token := range tokens {
			titleClass := MatchClass(token, title)
			if titleClass != ClassNone {
				weight += policy.TitleWeight
				titleMatches++
				matched++
				continue
			}
			if MatchClass(token, body) != ClassNone {
				weight += policy.BodyWeight
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		score := float64(weight) / denominator
		results = append(results, SimilarResult{
			Number:       file.Number,
			RawStatus:    file.RawStatus,
			PlainTitle:   issues.PlainTitle(file.Title),
			Path:         file.RelPath,
			Score:        score,
			Strong:       score >= policy.Threshold,
			titleMatches: titleMatches,
			matched:      matched,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.titleMatches != b.titleMatches {
			return a.titleMatches > b.titleMatches
		}
		if a.matched != b.matched {
			return a.matched > b.matched
		}
		if a.Number != b.Number {
			return issues.LessID(a.Number, b.Number)
		}
		return a.Path < b.Path
	})
	if len(results) > policy.MaxResults {
		results = results[:policy.MaxResults]
	}
	return results, nil
}
