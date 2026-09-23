package find

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

type ScoredChunk struct {
	Chunk Chunk
	Score float64
}

// Tokenize split text into normalized tokens
func Tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		lr := unicode.ToLower(r)
		if unicode.IsLetter(lr) || unicode.IsNumber(lr) {
			current.WriteRune(lr)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// ScoreChunk calculates BM25/Token-Density similarity score between query terms and chunk fields.
func ScoreChunk(chunk Chunk, queryTerms []string) float64 {
	if len(queryTerms) == 0 {
		return 1.0
	}

	score := 0.0

	identNorm := strings.ToLower(chunk.Identifier)
	pkgNorm := strings.ToLower(chunk.Package)
	pathNorm := strings.ToLower(chunk.FilePath)
	summaryNorm := strings.ToLower(chunk.Summary)

	identTokens := Tokenize(identNorm)
	pkgTokens := Tokenize(pkgNorm)
	pathTokens := Tokenize(pathNorm)
	summaryTokens := Tokenize(summaryNorm)

	for _, term := range queryTerms {
		term = strings.ToLower(term)
		termLen := len(term)
		if termLen == 0 {
			continue
		}

		termScore := 0.0

		// Identifier match
		if strings.Contains(identNorm, term) {
			termScore += 10.0
		}
		for _, tok := range identTokens {
			if tok == term {
				termScore += 15.0
			} else if strings.HasPrefix(tok, term) {
				termScore += 5.0
			}
		}

		// Package match
		if strings.Contains(pkgNorm, term) {
			termScore += 8.0
		}
		for _, tok := range pkgTokens {
			if tok == term {
				termScore += 10.0
			}
		}

		// File path match
		if strings.Contains(pathNorm, term) {
			termScore += 5.0
		}
		for _, tok := range pathTokens {
			if tok == term {
				termScore += 6.0
			}
		}

		// Summary match
		if strings.Contains(summaryNorm, term) {
			termScore += 3.0
		}
		matchCount := 0
		for _, tok := range summaryTokens {
			if tok == term {
				matchCount++
			} else if strings.HasPrefix(tok, term) {
				matchCount++
			}
		}
		if matchCount > 0 {
			// Sub-linear scaling for frequency
			termScore += 2.0 * (1.0 + math.Log(float64(matchCount)))
		}

		score += termScore
	}

	return score
}

// SearchChunks evaluates all chunks against query and builds a DiscoveryResult.
func SearchChunks(chunks []Chunk, query string) DiscoveryResult {
	queryTerms := Tokenize(query)

	var scored []ScoredChunk
	for _, chunk := range chunks {
		s := ScoreChunk(chunk, queryTerms)
		if s > 0 {
			scored = append(scored, ScoredChunk{
				Chunk: chunk,
				Score: s,
			})
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Chunk.FilePath < scored[j].Chunk.FilePath
	})

	var matchedChunks []Chunk
	pkgSet := make(map[string]bool)

	var decls []Chunk
	var docs []Chunk

	for _, sc := range scored {
		c := sc.Chunk
		matchedChunks = append(matchedChunks, c)

		if c.Package != "" {
			pkgSet[c.Package] = true
		}

		switch c.Kind {
		case KindType, KindFunc:
			decls = append(decls, c)
		case KindPackageDoc, KindDocSection:
			docs = append(docs, c)
		}
	}

	var packages []string
	for p := range pkgSet {
		packages = append(packages, p)
	}
	sort.Strings(packages)

	patterns := DetectPatterns(matchedChunks)
	sort.Strings(patterns)

	return DiscoveryResult{
		Query:        query,
		Packages:     packages,
		Patterns:     patterns,
		Declarations: decls,
		Docs:         docs,
	}
}
