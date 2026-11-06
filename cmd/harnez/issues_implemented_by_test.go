package main

import (
	"os"
	"strings"
	"testing"
)

// TestIssuesCloseImplementedBy covers closing a ticket whose work landed in a
// commit that does not name it (issue 1174).
func TestIssuesCloseImplementedBy(t *testing.T) {
	t.Run("unreferencing code commit closes and is recorded", func(t *testing.T) {
		dir, ticket := issuesFixtureRepo(t, sampleTicket)
		commitFile(t, dir, "impl.go", "package main\n", "fix parser")
		sha := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
		cmd := newIssuesCmd()
		cmd.SetArgs([]string{"close", "42", "resolved", "--implemented-by", sha[:8], "-d", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(ticket)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(after), "Status**: Closed — resolved") {
			t.Errorf("ticket not closed: %q", after)
		}
		body := repoRunGitOutput(t, dir, "log", "-1", "--format=%B")
		if !strings.Contains(body, "Implemented by "+sha[:12]) {
			t.Errorf("close commit %q does not name the implementing commit %s", body, sha[:12])
		}
	})

	refusals := []struct {
		name   string
		commit func(t *testing.T, dir string) string
		args   []string
		want   string
	}{
		{
			name: "commit touching only issues/",
			commit: func(t *testing.T, dir string) string {
				commitFile(t, dir, "issues/README.md", "# Issues\n\nnote\n", "docs(issues): note")
				return strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
			},
			want: "only issues/",
		},
		{
			name:   "unknown commit",
			commit: func(t *testing.T, dir string) string { return "deadbeefdeadbeef" },
			want:   "deadbeefdeadbeef",
		},
		{
			name: "combined with --no-code",
			commit: func(t *testing.T, dir string) string {
				commitFile(t, dir, "impl.go", "package main\n", "fix parser")
				return strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
			},
			args: []string{"--no-code"},
			want: "--no-code",
		},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			dir, _ := issuesFixtureRepo(t, sampleTicket)
			sha := tc.commit(t, dir)
			head := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
			cmd := newIssuesCmd()
			cmd.SetArgs(append([]string{"close", "42", "resolved", "--implemented-by", sha, "-d", dir}, tc.args...))
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want refusal containing %q, got %v", tc.want, err)
			}
			if got := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD")); got != head {
				t.Error("refused close moved HEAD")
			}
		})
	}

	t.Run("rejected outside close", func(t *testing.T) {
		dir, _ := issuesFixtureRepo(t, sampleTicket)
		cmd := newIssuesCmd()
		cmd.SetArgs([]string{"start", "42", "--implemented-by", "abc123", "-d", dir})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--implemented-by requires close or done") {
			t.Fatalf("want close-only refusal, got %v", err)
		}
	})
}
