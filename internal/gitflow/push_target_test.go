package gitflow

import (
	"strings"
	"testing"
)

// TestGuardPushTargetRemotes covers which destinations protected_main guards.
// origin and Forgejo (AGit) URLs refuse a direct push to the default branch;
// GitHub and named mirrors pass, so a release can mirror main there.
func TestGuardPushTargetRemotes(t *testing.T) {
	const (
		codeberg = "https://codeberg.org/ubunatic/harnez.git"
		github   = "https://github.com/ubunatic/harnez.git"
		bare     = "/srv/git/harnez.git"
	)
	for _, tc := range []struct {
		name, remote, url string
		refused           bool
	}{
		{name: "origin on Codeberg refuses", remote: "origin", url: codeberg, refused: true},
		{name: "origin over ssh refuses", remote: "origin", url: "git@codeberg.org:ubunatic/harnez.git", refused: true},
		{name: "origin on a local bare repo refuses", remote: "origin", url: bare, refused: true},
		{name: "origin on GitHub passes", remote: "origin", url: github},
		{name: "both to Codeberg refuses", remote: "both", url: codeberg, refused: true},
		{name: "both to GitHub passes", remote: "both", url: github},
		{name: "github mirror passes", remote: "github", url: github},
		{name: "named fork on a local path passes", remote: "fork", url: "/srv/forks/harnez.git"},
		{name: "named fork on Codeberg refuses", remote: "fork", url: "https://codeberg.org/someone/harnez.git", refused: true},
		{name: "unknown target refuses", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, stderr := guardRepo(t, "main", teamProtectedYAML)
			err := GuardPushTarget(dir, tc.remote, tc.url, "refs/heads/main")
			if !tc.refused {
				if err != nil {
					t.Fatalf("GuardPushTarget(%q, %q) = %v, want nil", tc.remote, tc.url, err)
				}
				if stderr.Len() != 0 {
					t.Fatalf("allowed push must not warn, got %q", stderr)
				}
				return
			}
			if err == nil {
				t.Fatalf("GuardPushTarget(%q, %q) = nil, want refusal", tc.remote, tc.url)
			}
			if !strings.Contains(err.Error(), "refusing a direct push to refs/heads/main") {
				t.Errorf("error %q does not name the refused push", err)
			}
		})
	}
}
