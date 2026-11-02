package forge

import "testing"

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		raw, host, owner, repo string
		ok                     bool
	}{
		{"ssh://git@codeberg.org/ubunatic/harnez.git", "codeberg.org", "ubunatic", "harnez", true},
		{"git@github.com:ubunatic/harnez.git", "github.com", "ubunatic", "harnez", true},
		{"https://github.com/ubunatic/harnez.git", "github.com", "ubunatic", "harnez", true},
		{"https://codeberg.org/ubunatic/harnez", "codeberg.org", "ubunatic", "harnez", true},
		{"  https://forge.example.org/team/tool  ", "forge.example.org", "team", "tool", true},
		{"/local/path/repo", "", "", "", false},
		{"", "", "", "", false},
	}
	for _, c := range cases {
		host, owner, repo, ok := ParseRemoteURL(c.raw)
		if host != c.host || owner != c.owner || repo != c.repo || ok != c.ok {
			t.Errorf("ParseRemoteURL(%q) = (%q,%q,%q,%v), want (%q,%q,%q,%v)",
				c.raw, host, owner, repo, ok, c.host, c.owner, c.repo, c.ok)
		}
	}
}
