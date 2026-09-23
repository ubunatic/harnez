package find

import (
	"testing"
)

func TestDetectPatterns(t *testing.T) {
	chunks := []Chunk{
		{
			FilePath:   "auth/jwt.go",
			Package:    "auth",
			Kind:       KindFunc,
			Identifier: "VerifyToken",
			Imports:    []string{"golang-jwt/jwt"},
			Summary:    "VerifyToken parses JWT bearer tokens",
		},
		{
			FilePath:   "db/sql.go",
			Package:    "db",
			Kind:       KindFunc,
			Identifier: "Connect",
			Imports:    []string{"database/sql", "jackc/pgx"},
			Summary:    "Connect connects to postgresql database",
		},
	}

	patterns := DetectPatterns(chunks)
	patternMap := make(map[string]bool)
	for _, p := range patterns {
		patternMap[p] = true
	}

	if !patternMap["JWT Bearer Token Authentication"] {
		t.Errorf("expected JWT Bearer Token Authentication pattern")
	}
	if !patternMap["PostgreSQL / Raw SQL Storage"] {
		t.Errorf("expected PostgreSQL / Raw SQL Storage pattern")
	}
}
