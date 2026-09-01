package find

import (
	"sort"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/issues"
)

// Result is one matched ticket, carrying only the fields `harnez find`
// prints plus what's needed to sort them.
type Result struct {
	Number     string
	RawStatus  string
	PlainTitle string
	Path       string

	worstClass int
	sumClasses int
}

// statusMatches implements the status/is filter semantics from issue 158:
// open/in-progress/blocked compare the ticket's leading raw lifecycle token
// (case-insensitive, suffix-stripped), so they can distinguish stages that
// issues.CanonicalizeStatus otherwise collapses together; closed/draft
// compare the canonical category.
func statusMatches(f issues.IssueFile, v StatusValue) bool {
	switch v {
	case StatusClosedValue:
		return f.Canonical == issues.StatusClosed
	case StatusDraftValue:
		return f.Canonical == issues.StatusDraft
	case StatusInProgressValue:
		return issues.LeadingLifecycle(f.RawStatus) == "in progress"
	case StatusBlockedValue:
		return issues.LeadingLifecycle(f.RawStatus) == "blocked"
	case StatusOpenValue:
		switch issues.LeadingLifecycle(f.RawStatus) {
		case "open", "in progress", "blocked":
			return true
		}
		return false
	default:
		return false
	}
}

// Search evaluates q against every issue file, returning matches ordered by
// the ranking contract from issue 158: worst (numerically highest) group
// class ascending, then the sum of all group classes ascending, then
// numeric ticket number, then repository-relative path.
func Search(files []issues.IssueFile, q *Query) []Result {
	var results []Result

	for _, f := range files {
		matchesFilters := true
		for _, filter := range q.Filters {
			if !statusMatches(f, filter.Value) {
				matchesFilters = false
				break
			}
		}
		if !matchesFilters {
			continue
		}

		titleNorm := Normalize(issues.StripTicketNumber(f.Title))
		bodyNorm := Normalize(f.Body)

		worst, sum := ClassNone, 0
		allMatch := true
		for _, g := range q.Groups {
			c := groupClass(g, titleNorm, bodyNorm)
			if c == ClassNone {
				allMatch = false
				break
			}
			if c > worst {
				worst = c
			}
			sum += c
		}
		if !allMatch {
			continue
		}

		results = append(results, Result{
			Number:     f.Number,
			RawStatus:  f.RawStatus,
			PlainTitle: issues.PlainTitle(f.Title),
			Path:       f.RelPath,
			worstClass: worst,
			sumClasses: sum,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.worstClass != b.worstClass {
			return a.worstClass < b.worstClass
		}
		if a.sumClasses != b.sumClasses {
			return a.sumClasses < b.sumClasses
		}
		an, aerr := strconv.Atoi(a.Number)
		bn, berr := strconv.Atoi(b.Number)
		if aerr == nil && berr == nil && an != bn {
			return an < bn
		}
		if a.Number != b.Number {
			return a.Number < b.Number
		}
		return a.Path < b.Path
	})

	return results
}

// sanitizeField replaces embedded tabs/newlines so a single result stays on
// one TSV line, per issue 158's output contract.
func sanitizeField(s string) string {
	r := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	return r.Replace(s)
}

// FormatTSV renders one result as the tab-separated output line:
// NUMBER<TAB>RAW_STATUS<TAB>PLAIN_TITLE<TAB>PATH.
func FormatTSV(r Result) string {
	return strings.Join([]string{
		sanitizeField(r.Number),
		sanitizeField(r.RawStatus),
		sanitizeField(r.PlainTitle),
		sanitizeField(r.Path),
	}, "\t")
}
