package find

import (
	"strings"
	"unicode"
)

// Match classes, best (1) to worst (6), per issue 158 section 2. Lower is
// better; 0 means "no match".
const (
	ClassNone = 0

	ClassTitleExact  = 1
	ClassTitlePrefix = 2
	ClassTitleFuzzy  = 3
	ClassBodyExact   = 4
	ClassBodyPrefix  = 5
	ClassBodyFuzzy   = 6
)

// Normalize implements the search normalization contract: Unicode
// lowercasing, replacing every non-letter/non-number rune (including
// Markdown syntax) with a space, and collapsing whitespace. It is applied to
// both searchable document fields and query alternatives, so a
// punctuation-separated identifier like "time-gauge" and a query of "time
// gauge" line up as adjacent tokens.
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		lr := unicode.ToLower(r)
		if unicode.IsLetter(lr) || unicode.IsNumber(lr) {
			b.WriteRune(lr)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// fuzzBudget returns the number of Damerau-Levenshtein edits a token of the
// given length tolerates: none at 4 characters or fewer (protects short
// identifiers like "gtt"/"vram" from noisy expansion), one edit at 5-8, two
// edits at 9+.
func fuzzBudget(tokenLen int) int {
	switch {
	case tokenLen <= 4:
		return 0
	case tokenLen <= 8:
		return 1
	default:
		return 2
	}
}

// tokenMatch reports whether altToken matches fieldToken, and if so whether
// the match required typo tolerance ("fuzzy") or was a plain prefix match.
func tokenMatch(altToken, fieldToken string) (matched, fuzzy bool) {
	if strings.HasPrefix(fieldToken, altToken) {
		return true, false
	}
	budget := fuzzBudget(len([]rune(altToken)))
	if budget == 0 {
		return false, false
	}
	if damerauLevenshteinBounded(altToken, fieldToken, budget) <= budget {
		return true, true
	}
	return false, false
}

// alternativeClassAgainst scores a single normalized alternative against one
// normalized field (title or body), returning classExact/classPrefix/
// classFuzzy (the caller's field-specific class constants) or ClassNone.
func alternativeClassAgainst(altNorm, fieldNorm string, classExact, classPrefix, classFuzzy int) int {
	if fieldNorm == "" || altNorm == "" {
		return ClassNone
	}
	if strings.Contains(fieldNorm, altNorm) {
		return classExact
	}

	fieldTokens := strings.Fields(fieldNorm)
	usedFuzz := false
	for _, altTok := range strings.Fields(altNorm) {
		found := false
		tokFuzzy := false
		for _, fieldTok := range fieldTokens {
			if m, fz := tokenMatch(altTok, fieldTok); m {
				found = true
				if !fz {
					tokFuzzy = false
					break
				}
				tokFuzzy = true
			}
		}
		if !found {
			return ClassNone
		}
		if tokFuzzy {
			usedFuzz = true
		}
	}
	if usedFuzz {
		return classFuzzy
	}
	return classPrefix
}

// alternativeClass scores a normalized alternative against a normalized
// title and body, returning the best (numerically lowest) of the six match
// classes, or ClassNone if it matches neither field.
func alternativeClass(altNorm, titleNorm, bodyNorm string) int {
	if c := alternativeClassAgainst(altNorm, titleNorm, ClassTitleExact, ClassTitlePrefix, ClassTitleFuzzy); c != ClassNone {
		return c
	}
	return alternativeClassAgainst(altNorm, bodyNorm, ClassBodyExact, ClassBodyPrefix, ClassBodyFuzzy)
}

// groupClass scores an OR group (its Alternatives) against a normalized
// title/body pair, retaining the best-matching alternative. Returns
// ClassNone if no alternative matches either field.
func groupClass(g Group, titleNorm, bodyNorm string) int {
	best := ClassNone
	for _, alt := range g.Alternatives {
		c := alternativeClass(Normalize(alt), titleNorm, bodyNorm)
		if c == ClassNone {
			continue
		}
		if best == ClassNone || c < best {
			best = c
		}
	}
	return best
}

// damerauLevenshtein computes the true Damerau-Levenshtein edit distance
// (adjacent transpositions count as one edit) between a and b, short-
// circuiting once the distance is certain to exceed maxDist -- callers only
// need to know whether the distance is within budget, not its exact value
// beyond that.
func damerauLevenshtein(a, b string) int { return damerauLevenshteinBounded(a, b, -1) }

func damerauLevenshteinBounded(a, b string, maxDist int) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)

	if maxDist >= 0 {
		diff := la - lb
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDist {
			return maxDist + 1
		}
	}

	inf := la + lb
	d := make([][]int, la+2)
	for i := range d {
		d[i] = make([]int, lb+2)
	}
	d[0][0] = inf
	for i := 0; i <= la; i++ {
		d[i+1][0] = inf
		d[i+1][1] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j+1] = inf
		d[1][j+1] = j
	}

	lastRow := make(map[rune]int)
	for i := 1; i <= la; i++ {
		lastCol := 0
		for j := 1; j <= lb; j++ {
			i2 := lastRow[rb[j-1]]
			j2 := lastCol
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
				lastCol = j
			}
			del := d[i][j+1] + 1
			ins := d[i+1][j] + 1
			sub := d[i][j] + cost
			trans := d[i2][j2] + (i-i2-1) + 1 + (j-j2-1)
			best := del
			if ins < best {
				best = ins
			}
			if sub < best {
				best = sub
			}
			if trans < best {
				best = trans
			}
			d[i+1][j+1] = best
		}
		lastRow[ra[i-1]] = i
	}
	return d[la+1][lb+1]
}
