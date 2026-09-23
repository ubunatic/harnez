package find

import "strings"

type PatternRule struct {
	Name     string
	Imports  []string
	Keywords []string
}

var defaultPatternRules = []PatternRule{
	{
		Name:     "JWT Bearer Token Authentication",
		Imports:  []string{"golang-jwt/jwt", "passthrough"},
		Keywords: []string{"jwt", "bearer"},
	},
	{
		Name:     "Cookie-based Session Management",
		Imports:  []string{"gorilla/sessions", "net/http/cookiejar"},
		Keywords: []string{"cookie", "session"},
	},
	{
		Name:     "bcrypt Password Hashing",
		Imports:  []string{"golang.org/x/crypto/bcrypt"},
		Keywords: []string{"bcrypt"},
	},
	{
		Name:     "PostgreSQL / Raw SQL Storage",
		Imports:  []string{"database/sql", "jackc/pgx", "lib/pq"},
		Keywords: []string{"sql", "postgres", "pgx"},
	},
	{
		Name:     "Compiled Type-Safe SQL (sqlc)",
		Imports:  []string{"sqlc"},
		Keywords: []string{"sqlc"},
	},
}

// DetectPatterns inspects imports and summary content across matched chunks and returns detected patterns.
func DetectPatterns(chunks []Chunk) []string {
	detectedSet := make(map[string]bool)

	for _, chunk := range chunks {
		// Check imports
		for _, imp := range chunk.Imports {
			impLower := strings.ToLower(imp)
			for _, rule := range defaultPatternRules {
				for _, matchImp := range rule.Imports {
					if strings.Contains(impLower, strings.ToLower(matchImp)) {
						detectedSet[rule.Name] = true
					}
				}
			}
		}

		// Check keywords in summary / identifier
		sumLower := strings.ToLower(chunk.Identifier + " " + chunk.Summary)
		for _, rule := range defaultPatternRules {
			for _, kw := range rule.Keywords {
				if strings.Contains(sumLower, kw) {
					detectedSet[rule.Name] = true
				}
			}
		}
	}

	var result []string
	for p := range detectedSet {
		result = append(result, p)
	}

	return result
}
