package issues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDuplicateNumbersWithoutHeadersOrReadme(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"096-a.md", "096-b.md", "096-c.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# Unnumbered\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := CheckDuplicateNumbers(dir)
	if err == nil {
		t.Fatal("duplicates accepted")
	}
	for _, want := range []string{"issues/096-a.md", "issues/096-b.md", "issues/096-c.md", "harnez issues mv -d . <file>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v missing %q", err, want)
		}
	}
}
