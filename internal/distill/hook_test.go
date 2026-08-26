package distill

import "testing"

func TestRewriteBashCommand_Noisy(t *testing.T) {
	cases := []string{
		"go test ./...",
		"go build ./...",
		"go vet ./...",
		"cargo test",
		"cargo build --release",
		"pytest -v",
		"npm test",
		"npm run test",
		"make test",
		"make build",
		"make check",
		"git status",
		"git diff",
		"git log --oneline",
		"cd /repo && go test ./...",
	}
	for _, cmd := range cases {
		got, ok := RewriteBashCommand(cmd)
		if !ok {
			t.Errorf("RewriteBashCommand(%q): expected rewrite, got none", cmd)
			continue
		}
		want := "set -o pipefail; ( " + cmd + " ) 2>&1 | harnez distill"
		if got != want {
			t.Errorf("RewriteBashCommand(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestRewriteBashCommand_NotNoisy(t *testing.T) {
	cases := []string{
		"",
		"ls -la",
		"vim foo.go",
		"ssh host",
		"docker exec -it container bash",
		"npm run dev",
		"cat foo.txt",
		"echo hello",
	}
	for _, cmd := range cases {
		got, ok := RewriteBashCommand(cmd)
		if ok {
			t.Errorf("RewriteBashCommand(%q): expected no rewrite, got %q", cmd, got)
		}
		if got != cmd {
			t.Errorf("RewriteBashCommand(%q) returned %q, want unchanged", cmd, got)
		}
	}
}

func TestRewriteBashCommand_AlreadyPiped(t *testing.T) {
	cmd := "go test ./... | harnez distill"
	got, ok := RewriteBashCommand(cmd)
	if ok {
		t.Errorf("RewriteBashCommand(%q): expected no double-pipe rewrite, got %q", cmd, got)
	}
	if got != cmd {
		t.Errorf("RewriteBashCommand(%q) = %q, want unchanged", cmd, got)
	}
}
