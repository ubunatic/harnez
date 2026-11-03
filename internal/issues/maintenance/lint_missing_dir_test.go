package maintenance

import (
	"bytes"
	"strings"
	"testing"
)

// A fresh harnez init has no issues/ yet; the pre-commit lint must pass (issue 1072).
func TestIssuesLintMissingIssuesDir(t *testing.T) {
	var out bytes.Buffer
	if err := runIssuesLintMode(&out, t.TempDir(), true, nil); err != nil {
		t.Fatalf("lint without issues/: %v", err)
	}
	if !strings.Contains(out.String(), "nothing to lint") {
		t.Fatalf("output = %q", out.String())
	}
}
