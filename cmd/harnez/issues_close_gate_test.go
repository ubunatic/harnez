package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// commitFile writes content to file (relative to dir), commits it with subject,
// and leaves the tree clean. A path outside issues/ makes it an implementation
// commit for the close gate; a path inside issues/ does not.
func commitFile(t *testing.T, dir, file, content, subject string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", file)
	repoRunGit(t, dir, "commit", "-q", "-m", subject)
}

func TestIssuesCloseRequiresCodeCommit(t *testing.T) {
	cases := []struct {
		name    string
		commit  func(t *testing.T, dir string)
		noCode  bool
		wantErr bool
	}{
		{name: "no commit at all", wantErr: true},
		{
			name: "code commit without ticket reference",
			commit: func(t *testing.T, dir string) {
				commitFile(t, dir, "impl.go", "package main\n", "fix parser")
			},
			wantErr: true,
		},
		{
			name: "commit touching only issues/ names the ticket",
			commit: func(t *testing.T, dir string) {
				commitFile(t, dir, "issues/README.md", "# Issues\n\nnote\n", "docs(issues): note (issue 42)")
			},
			wantErr: true,
		},
		{
			name: "near-miss ticket number does not count",
			commit: func(t *testing.T, dir string) {
				commitFile(t, dir, "impl.go", "package main\n", "fix parser (issue 420)")
			},
			wantErr: true,
		},
		{
			name: "code commit naming the ticket",
			commit: func(t *testing.T, dir string) {
				commitFile(t, dir, "impl.go", "package main\n", "fix parser (issue 42)")
			},
		},
		{name: "--no-code override without any commit", noCode: true},
	}
	for _, tc := range cases {
		for _, verb := range []string{"close", "done"} {
			t.Run(verb+"/"+tc.name, func(t *testing.T) {
				dir, ticket := issuesFixtureRepo(t, sampleTicket)
				if tc.commit != nil {
					tc.commit(t, dir)
				}
				head := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
				before, err := os.ReadFile(ticket)
				if err != nil {
					t.Fatal(err)
				}
				args := []string{verb, "42", "resolved", "-d", dir}
				if tc.noCode {
					args = append(args, "--no-code")
				}
				cmd := newIssuesCmd()
				cmd.SetArgs(args)
				err = cmd.Execute()
				if !tc.wantErr {
					if err != nil {
						t.Fatal(err)
					}
					after, err := os.ReadFile(ticket)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(after), "Status**: Closed — resolved") {
						t.Errorf("ticket not closed: %q", after)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "--no-code") {
					t.Fatalf("want refusal naming --no-code, got %v", err)
				}
				if got := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD")); got != head {
					t.Error("refused close moved HEAD")
				}
				if got := repoRunGitOutput(t, dir, "status", "--porcelain"); got != "" {
					t.Errorf("refused close wrote files: %s", got)
				}
				after, err := os.ReadFile(ticket)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Error("refused close changed the ticket")
				}
			})
		}
	}
}

func TestIssuesCloseNoCodeStatesItInCommit(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	cmd := newIssuesCmd()
	cmd.SetArgs([]string{"close", "42", "resolved", "--no-code", "-d", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body := repoRunGitOutput(t, dir, "log", "-1", "--format=%B")
	for _, want := range []string{"docs(issues): close 042, resolved", "No code change", "(--no-code)"} {
		if !strings.Contains(body, want) {
			t.Errorf("close commit message %q missing %q", body, want)
		}
	}
}

func TestIssuesCloseBatchRefusesWithoutCodeCommit(t *testing.T) {
	dir, ticket := issuesFixtureRepo(t, sampleTicket)
	other := strings.ReplaceAll(sampleTicket, "042", "043")
	otherPath := filepath.Join(dir, "issues", "043-other.md")
	if err := os.WriteFile(otherPath, []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRunGit(t, dir, "add", "issues")
	repoRunGit(t, dir, "commit", "-qm", "second ticket")
	head := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
	before, err := os.ReadFile(ticket)
	if err != nil {
		t.Fatal(err)
	}
	cmd := newIssuesCmd()
	cmd.SetArgs([]string{"close", "42", "43", "resolved", "-d", dir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--no-code") {
		t.Fatalf("want batch refusal naming --no-code, got %v", err)
	}
	if got := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD")); got != head {
		t.Error("refused batch close moved HEAD")
	}
	if got := repoRunGitOutput(t, dir, "status", "--porcelain"); got != "" {
		t.Errorf("refused batch close wrote files: %s", got)
	}
	after, err := os.ReadFile(ticket)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("refused batch close changed a ticket")
	}
}

func TestIssuesNonCloseVerbsIgnoreCodeGate(t *testing.T) {
	for _, args := range [][]string{{"start", "42"}, {"block", "42", "waiting on review"}} {
		t.Run(args[0], func(t *testing.T) {
			dir, _ := issuesFixtureRepo(t, sampleTicket)
			var stderr bytes.Buffer
			cmd := newIssuesCmd()
			cmd.SetErr(&stderr)
			cmd.SetArgs(append(args, "-d", dir))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("%s refused without a code commit: %v", args[0], err)
			}
			if strings.Contains(stderr.String(), "warning") || strings.Contains(stderr.String(), "--no-code") {
				t.Errorf("%s printed a close-gate message: %q", args[0], stderr.String())
			}
		})
	}
}

func TestIssuesNoCodeRejectedOutsideClose(t *testing.T) {
	dir, _ := issuesFixtureRepo(t, sampleTicket)
	cmd := newIssuesCmd()
	cmd.SetArgs([]string{"start", "42", "--no-code", "-d", dir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--no-code requires close or done") {
		t.Fatalf("want --no-code rejected for start, got %v", err)
	}
}
