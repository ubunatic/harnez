package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/find"
)

func TestHarnezFindBroadIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sample Go files and Markdown doc
	authDir := filepath.Join(tmpDir, "auth")
	os.MkdirAll(authDir, 0o755)

	authGo := `package auth

import "golang-jwt/jwt"

// SessionManager handles session tokens.
type SessionManager struct {
	Secret string
}

// Login verifies user credentials and returns JWT.
func Login(user, pass string) (string, error) {
	return "", nil
}
`
	os.WriteFile(filepath.Join(authDir, "auth.go"), []byte(authGo), 0o644)

	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0o755)
	authMd := `# Auth Architecture

Overview of JWT session state tracking.
`
	os.WriteFile(filepath.Join(docsDir, "auth.md"), []byte(authMd), 0o644)

	// 1. Run harnez index
	var outBuf, errBuf bytes.Buffer
	opts := indexOptions{Dir: tmpDir}
	if err := runIndex(&outBuf, opts); err != nil {
		t.Fatalf("runIndex failed: %v", err)
	}

	indexPath := filepath.Join(tmpDir, ".harnez", "index.json")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected .harnez/index.json to be created by index: %v", err)
	}

	// 2. Run harnez find --broad "session"
	outBuf.Reset()
	errBuf.Reset()
	findOpts := findRunOptions{
		Dir:       tmpDir,
		Broad:     true,
		MaxTokens: 800,
	}
	if err := runFindWithOptions(&outBuf, &errBuf, []string{"session"}, findOpts); err != nil {
		t.Fatalf("runFindWithOptions failed: %v", err)
	}

	outStr := outBuf.String()
	if !strings.Contains(outStr, "# Architectural Discovery") {
		t.Errorf("expected markdown header in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "auth") {
		t.Errorf("expected package auth in output")
	}

	// 3. Run harnez find --broad --json "session"
	outBuf.Reset()
	errBuf.Reset()
	findOpts.JSON = true
	if err := runFindWithOptions(&outBuf, &errBuf, []string{"session"}, findOpts); err != nil {
		t.Fatalf("runFindWithOptions --json failed: %v", err)
	}

	var discoveryRes find.DiscoveryResult
	if err := json.Unmarshal(outBuf.Bytes(), &discoveryRes); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if len(discoveryRes.Packages) == 0 {
		t.Errorf("expected packages in json discovery result")
	}

	// 4. Test fallback on-the-fly scan when index is missing
	os.Remove(indexPath)
	outBuf.Reset()
	errBuf.Reset()
	findOpts.JSON = false
	if err := runFindWithOptions(&outBuf, &errBuf, []string{"session"}, findOpts); err != nil {
		t.Fatalf("runFindWithOptions on-the-fly fallback failed: %v", err)
	}

	if !strings.Contains(errBuf.String(), "falling back to on-the-fly scan") {
		t.Errorf("expected warning on stderr when index is missing, got: %s", errBuf.String())
	}
}
