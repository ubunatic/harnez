package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSummaryFromRealGoTestJSON runs go test -json on a module with a passing
// test, a failing test, a failing subtest and a package that does not build.
func TestSummaryFromRealGoTestJSON(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":            "module example.com/m\n\ngo 1.24\n",
		"ok/ok_test.go":     "package ok\nimport \"testing\"\nfunc TestFine(t *testing.T) { t.Log(\"fine\") }\n",
		"bad/bad_test.go":   "package bad\nimport \"testing\"\nfunc TestBroken(t *testing.T) { t.Fatal(\"want 1, got 2\") }\nfunc TestTable(t *testing.T) { t.Run(\"case_a\", func(t *testing.T) { t.Error(\"case a broke\") }) }\n",
		"nobuild/x_test.go": "package nobuild\nimport \"testing\"\nfunc TestX(t *testing.T) { undefinedCall() }\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-json", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	out, err := cmd.Output()
	if err == nil {
		t.Fatal("go test passed, want failures")
	}
	in := append([]byte("gofmt: some/file.go\n"), out...)

	rep, err := parse(bytes.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(rep.tests(), "\n")
	want := "example.com/m/bad TestBroken\nexample.com/m/bad TestTable/case_a\nexample.com/m/nobuild"
	if got != want {
		t.Fatalf("failing = %q, want %q", got, want)
	}

	var md bytes.Buffer
	markdown(&md, rep, 40)
	for _, part := range []string{"### go test: 3 failing", "`example.com/m/bad TestBroken`", "want 1, got 2", "case a broke", "undefined: undefinedCall", "gofmt: some/file.go"} {
		if !strings.Contains(md.String(), part) {
			t.Errorf("summary lacks %q:\n%s", part, md.String())
		}
	}
	if strings.Contains(md.String(), "TestFine") {
		t.Errorf("summary lists a passing test:\n%s", md.String())
	}

	var ann bytes.Buffer
	annotate(&ann, rep, 10)
	lines := strings.Split(strings.TrimSpace(ann.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "::error title=example.com/m/bad TestBroken::") || !strings.Contains(lines[0], "want 1, got 2") {
		t.Fatalf("annotations = %q", ann.String())
	}
	if strings.Contains(lines[0], "\n") || !strings.Contains(lines[1], "%0A") {
		t.Fatalf("annotation newlines not encoded: %q", ann.String())
	}
}

func TestSummaryAllPass(t *testing.T) {
	rep, err := parse(strings.NewReader(`{"Action":"pass","Package":"p","Test":"TestA"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	var md bytes.Buffer
	markdown(&md, rep, 5)
	if md.String() != "### go test: no failing tests\n" {
		t.Fatalf("summary = %q", md.String())
	}
}

func TestEscapeProp(t *testing.T) {
	if got := escapeProp("a:b,c%\nd"); got != "a%3Ab%2Cc%25%0Ad" {
		t.Fatalf("escapeProp = %q", got)
	}
}
