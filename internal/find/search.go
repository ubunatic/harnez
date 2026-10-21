package find

import (
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/issues"
)

// Result is one matched ticket, carrying only the fields `harnez find`
// prints plus what's needed to sort them.
type Result struct {
	Number     string `json:"number"`
	RawStatus  string `json:"raw_status"`
	PlainTitle string `json:"title"`
	Path       string `json:"path"`

	worstClass int
	sumClasses int
}

// statusMatches delegates lifecycle vocabulary and matching to the embedded
// issue spec so the query parser and evaluator share one definition.
func statusMatches(f issues.IssueFile, v StatusValue) bool {
	return issues.StatusFilterMatches(f.RawStatus, string(v))
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
		if a.Number != b.Number {
			return issues.LessID(a.Number, b.Number)
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
