package find

import (
	"testing"
)

func TestSearchChunks(t *testing.T) {
	chunks := []Chunk{
		{
			FilePath:   "auth/jwt.go",
			Package:    "auth",
			Kind:       KindFunc,
			Identifier: "VerifyToken",
			Imports:    []string{"golang-jwt/jwt"},
			Summary:    "VerifyToken verifies incoming JWT bearer token and login credentials",
		},
		{
			FilePath:   "db/user.go",
			Package:    "db",
			Kind:       KindType,
			Identifier: "UserRecord",
			Imports:    []string{"database/sql"},
			Summary:    "UserRecord represents database user row",
		},
		{
			FilePath:   "docs/auth.md",
			Kind:       KindDocSection,
			Identifier: "Login Flow",
			Summary:    "Overview of authentication and login flow in harnez",
		},
	}

	result := SearchChunks(chunks, "auth and login logic")

	if len(result.Packages) == 0 {
		t.Errorf("expected packages, got empty")
	}

	foundAuthPkg := false
	for _, pkg := range result.Packages {
		if pkg == "auth" {
			foundAuthPkg = true
		}
	}
	if !foundAuthPkg {
		t.Errorf("expected package 'auth' in discovery result")
	}

	if len(result.Declarations) == 0 {
		t.Errorf("expected key declarations, got empty")
	}

	if len(result.Docs) == 0 {
		t.Errorf("expected relevant docs, got empty")
	}

	if len(result.Patterns) == 0 {
		t.Errorf("expected patterns detected, got empty")
	}
}
