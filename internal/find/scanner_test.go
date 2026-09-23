package find

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanGoFile(t *testing.T) {
	tmpDir := t.TempDir()
	goContent := `// Package auth provides authentication mechanisms.
package auth

import (
	"net/http"
	"golang-jwt/jwt"
)

// User represents an authenticated user.
type User struct {
	ID   string
	Name string
}

// Authenticate verifies the request token.
func Authenticate(r *http.Request) (*User, error) {
	return nil, nil
}
`
	filePath := filepath.Join(tmpDir, "auth.go")
	if err := os.WriteFile(filePath, []byte(goContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	chunks, err := ScanGoFile(tmpDir, "auth.go", filePath)
	if err != nil {
		t.Fatalf("ScanGoFile failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatalf("expected chunks, got 0")
	}

	hasPkgDoc := false
	hasType := false
	hasFunc := false

	for _, c := range chunks {
		if c.Kind == KindPackageDoc && c.Identifier == "auth" {
			hasPkgDoc = true
		}
		if c.Kind == KindType && c.Identifier == "User" {
			hasType = true
		}
		if c.Kind == KindFunc && c.Identifier == "Authenticate" {
			hasFunc = true
			if len(c.Imports) == 0 {
				t.Errorf("expected imports in chunk")
			}
		}
	}

	if !hasPkgDoc {
		t.Errorf("missing package doc chunk")
	}
	if !hasType {
		t.Errorf("missing User type chunk")
	}
	if !hasFunc {
		t.Errorf("missing Authenticate func chunk")
	}
}

func TestScanMarkdownFile(t *testing.T) {
	tmpDir := t.TempDir()
	mdContent := `# Authentication Overview

This document describes auth design.

## JWT Tokens

We use JWT tokens for stateless authentication.
`
	filePath := filepath.Join(tmpDir, "auth.md")
	if err := os.WriteFile(filePath, []byte(mdContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	chunks, err := ScanMarkdownFile(tmpDir, "auth.md", filePath)
	if err != nil {
		t.Fatalf("ScanMarkdownFile failed: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 markdown section chunks, got %d", len(chunks))
	}

	if chunks[0].Identifier != "Authentication Overview" {
		t.Errorf("expected header 'Authentication Overview', got %q", chunks[0].Identifier)
	}
	if chunks[1].Identifier != "JWT Tokens" {
		t.Errorf("expected header 'JWT Tokens', got %q", chunks[1].Identifier)
	}
}
