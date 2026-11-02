package prwait

import (
	"strings"
	"testing"
)

func TestParsePRURL(t *testing.T) {
	cases := []struct {
		raw, host, owner, repo string
		n                      int
		ok                     bool
	}{
		{"https://codeberg.org/ubunatic/harnez/pulls/78", "codeberg.org", "ubunatic", "harnez", 78, true},
		{"https://github.com/ubunatic/harnez/pull/12", "github.com", "ubunatic", "harnez", 12, true},
		{"https://codeberg.org/ubunatic/harnez/pulls/0", "", "", "", 0, false},
		{"https://codeberg.org/ubunatic/harnez/issues/78", "", "", "", 0, false},
		{"ssh://git@codeberg.org/ubunatic/harnez.git", "", "", "", 0, false},
		{"https://codeberg.org/ubunatic/harnez/pulls/78/files", "", "", "", 0, false},
	}
	for _, c := range cases {
		host, owner, repo, n, err := ParsePRURL(c.raw)
		if (err == nil) != c.ok {
			t.Errorf("ParsePRURL(%q) err = %v, want ok=%v", c.raw, err, c.ok)
			continue
		}
		if c.ok && (host != c.host || owner != c.owner || repo != c.repo || n != c.n) {
			t.Errorf("ParsePRURL(%q) = %s %s %s %d", c.raw, host, owner, repo, n)
		}
	}
}

func TestResolveTarget(t *testing.T) {
	spec := Spec{Forges: []Forge{
		{Host: "codeberg.org", Kind: "forgejo", APIBase: "https://codeberg.org/api/v1"},
		{Host: "github.com", Kind: "github", APIBase: "https://api.github.com"},
	}}
	t.Run("number from ssh remote", func(t *testing.T) {
		tg, err := ResolveTarget(spec, "78", "ssh://git@codeberg.org/ubunatic/harnez.git")
		if err != nil {
			t.Fatal(err)
		}
		if tg.Forge.Kind != "forgejo" || tg.Owner != "ubunatic" || tg.Repo != "harnez" || tg.Number != 78 {
			t.Fatalf("target = %+v", tg)
		}
	})
	t.Run("number from github https remote", func(t *testing.T) {
		tg, err := ResolveTarget(spec, "5", "https://github.com/ubunatic/harnez.git")
		if err != nil {
			t.Fatal(err)
		}
		if tg.Forge.Kind != "github" || tg.Number != 5 {
			t.Fatalf("target = %+v", tg)
		}
	})
	t.Run("URL ignores the remote", func(t *testing.T) {
		tg, err := ResolveTarget(spec, "https://codeberg.org/ubunatic/harnez/pulls/78", "")
		if err != nil {
			t.Fatal(err)
		}
		if tg.Forge.Host != "codeberg.org" || tg.Number != 78 {
			t.Fatalf("target = %+v", tg)
		}
	})
	t.Run("unknown host is refused", func(t *testing.T) {
		_, err := ResolveTarget(spec, "https://gitlab.example/ubunatic/harnez/pulls/1", "")
		if err == nil || !strings.Contains(err.Error(), "not configured") {
			t.Fatalf("err = %v, want a not-configured error", err)
		}
	})
	t.Run("bad number is refused", func(t *testing.T) {
		for _, arg := range []string{"abc", "0", "-3"} {
			if _, err := ResolveTarget(spec, arg, "https://codeberg.org/ubunatic/harnez.git"); err == nil {
				t.Errorf("ResolveTarget(%q) accepted a bad number", arg)
			}
		}
	})
	t.Run("unparseable remote is refused", func(t *testing.T) {
		if _, err := ResolveTarget(spec, "78", "/local/bare/repo"); err == nil {
			t.Fatal("expected an error for a local remote path")
		}
	})
}

func TestPageURLUsesForgeSegment(t *testing.T) {
	forgejo := Target{Forge: Forge{Host: "codeberg.org", Kind: "forgejo"}, Owner: "o", Repo: "r", Number: 1}
	gh := Target{Forge: Forge{Host: "github.com", Kind: "github"}, Owner: "o", Repo: "r", Number: 1}
	if got := forgejo.PageURL(); got != "https://codeberg.org/o/r/pulls/1" {
		t.Errorf("forgejo page = %s", got)
	}
	if got := gh.PageURL(); got != "https://github.com/o/r/pull/1" {
		t.Errorf("github page = %s", got)
	}
}
