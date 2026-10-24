package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuesCloseAlreadyClosed(t *testing.T) {
	for _, verb := range []string{"close", "done"} {
		for _, reason := range []string{"resolved", "different outcome", ""} {
			t.Run(verb+"/"+reason, func(t *testing.T) {
				dir, ticket := issuesFixtureRepo(t, sampleTicket)
				cmd := newIssuesCmd()
				cmd.SetArgs([]string{"close", "42", "resolved", "--no-code", "-d", dir})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				head := repoRunGitOutput(t, dir, "rev-parse", "HEAD")
				before, err := os.ReadFile(ticket)
				if err != nil {
					t.Fatal(err)
				}
				readme := filepath.Join(dir, "issues", "README.md")
				beforeReadme, err := os.ReadFile(readme)
				if err != nil {
					t.Fatal(err)
				}
				var out bytes.Buffer
				cmd = newIssuesCmd()
				cmd.SetOut(&out)
				cmd.SetArgs([]string{verb, "42", reason, "-d", dir})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(ticket)
				if err != nil {
					t.Fatal(err)
				}
				afterReadme, err := os.ReadFile(readme)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) || !bytes.Equal(beforeReadme, afterReadme) {
					t.Error("no-op changed ticket or README")
				}
				if got := repoRunGitOutput(t, dir, "rev-parse", "HEAD"); got != head {
					t.Error("second close moved HEAD")
				}
				for _, want := range []string{"already Closed — resolved", "no change", "--amend-reason"} {
					if !strings.Contains(out.String(), want) {
						t.Errorf("notice %q missing %q", out.String(), want)
					}
				}
			})
		}
	}
}

func TestIssuesCloseAmendReason(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(map[bool]string{false: "open", true: "closed"}[closed], func(t *testing.T) {
			content := sampleTicket
			if closed {
				content = strings.Replace(content, "Status**: Open", "Status**: Closed — old outcome", 1)
			}
			dir, ticket := issuesFixtureRepo(t, content)
			head := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
			cmd := newIssuesCmd()
			cmd.SetArgs([]string{"close", "42", "new outcome", "--amend-reason", "--no-code", "-d", dir})
			err := cmd.Execute()
			if !closed {
				if err == nil || !strings.Contains(err.Error(), "already Closed") {
					t.Fatalf("expected already Closed requirement, got %v", err)
				}
				if got := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD")); got != head {
					t.Error("invalid amend moved HEAD")
				}
				if got := repoRunGitOutput(t, dir, "status", "--porcelain"); got != "" {
					t.Errorf("invalid amend wrote files: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if count := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-list", "--count", head+"..HEAD")); count != "1" {
				t.Errorf("amend commits = %s, want 1", count)
			}
			got, err := os.ReadFile(ticket)
			if err != nil {
				t.Fatal(err)
			}
			if want := strings.Replace(content, "old outcome", "new outcome", 1); string(got) != want {
				t.Errorf("amended ticket = %q, want %q", got, want)
			}
		})
	}
}

func TestIssuesCloseBatchPreservesClosure(t *testing.T) {
	for _, mode := range []string{"noop", "mixed", "amend", "invalid-amend"} {
		t.Run(mode, func(t *testing.T) {
			closed := strings.Replace(sampleTicket, "Status**: Open", "Status**: Closed — original", 1)
			dir, ticket := issuesFixtureRepo(t, closed)
			other := strings.ReplaceAll(closed, "042", "043")
			if mode == "mixed" || mode == "invalid-amend" {
				other = strings.ReplaceAll(sampleTicket, "042", "043")
			}
			otherPath := filepath.Join(dir, "issues", "043-other.md")
			if err := os.WriteFile(otherPath, []byte(other), 0o644); err != nil {
				t.Fatal(err)
			}
			repoRunGit(t, dir, "add", "issues")
			repoRunGit(t, dir, "commit", "-qm", "second ticket")
			head := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-parse", "HEAD"))
			args := []string{"close", "42", "43", "new reason", "--no-code", "-d", dir}
			if mode == "amend" || mode == "invalid-amend" {
				args = append(args, "--amend-reason")
			}
			cmd := newIssuesCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(args)
			err := cmd.Execute()
			if mode == "invalid-amend" {
				if err == nil || !strings.Contains(err.Error(), "already Closed") {
					t.Fatalf("expected amend validation error, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			wantCommits := "1"
			if mode == "noop" || mode == "invalid-amend" {
				wantCommits = "0"
			}
			if got := strings.TrimSpace(repoRunGitOutput(t, dir, "rev-list", "--count", head+"..HEAD")); got != wantCommits {
				t.Errorf("commits = %s, want %s", got, wantCommits)
			}
			wantTicket := closed
			if mode == "amend" {
				wantTicket = strings.Replace(closed, "original", "new reason", 1)
			}
			for path, want := range map[string]string{ticket: wantTicket, otherPath: other} {
				if path == otherPath && (mode == "amend" || mode == "mixed") {
					want = strings.Replace(other, "Status**: Open", "Status**: Closed — new reason", 1)
					want = strings.Replace(want, "original", "new reason", 1)
				}
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("%s = %q, want %q", path, got, want)
				}
			}
			if mode == "noop" && !strings.Contains(out.String(), "--amend-reason") {
				t.Errorf("missing amend hint: %s", out.String())
			}
			if mode == "noop" || mode == "invalid-amend" {
				if got := repoRunGitOutput(t, dir, "status", "--porcelain"); got != "" {
					t.Errorf("unexpected writes: %s", got)
				}
			}
		})
	}
}
