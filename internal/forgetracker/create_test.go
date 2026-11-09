package forgetracker

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/forge"
)

func lookOnly(tools ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, t := range tools {
			if t == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

var (
	cbTarget = Target{Forge: forge.Forge{Host: "codeberg.org", Kind: "forgejo", IssueCLI: []string{"tea", "fj"}}, Owner: "o", Repo: "r"}
	ghTarget = Target{Forge: forge.Forge{Host: "github.com", Kind: "github", IssueCLI: []string{"gh"}}, Owner: "o", Repo: "r"}
)

func TestPlanCreatePicksInstalledCLI(t *testing.T) {
	req := CreateRequest{Title: "Fix it", BodyFile: "b.md", Labels: []string{"bug", "P2"}}
	cases := []struct {
		name   string
		target Target
		look   []string
		req    CreateRequest
		want   Command
	}{
		{"tea first", cbTarget, []string{"tea", "fj"}, req, Command{"tea", []string{"issues", "create", "--repo", "o/r", "--title", "Fix it", "--description-file", "b.md", "--labels", "bug,P2"}}},
		{"fj without labels", cbTarget, []string{"fj"}, CreateRequest{Title: "Fix it", BodyFile: "b.md"}, Command{"fj", []string{"--host", "codeberg.org", "issue", "create", "--repo", "o/r", "--body-file", "b.md", "Fix it"}}},
		{"gh", ghTarget, []string{"gh", "tea"}, req, Command{"gh", []string{"issue", "create", "--repo", "github.com/o/r", "--title", "Fix it", "--body-file", "b.md", "--label", "bug", "--label", "P2"}}},
	}
	for _, c := range cases {
		got, err := PlanCreate(c.target, c.req, lookOnly(c.look...))
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
}

func TestPlanCreateErrors(t *testing.T) {
	req := CreateRequest{Title: "T", BodyFile: "b.md"}
	cases := []struct {
		name   string
		target Target
		req    CreateRequest
		look   []string
		want   string
	}{
		{"missing tool named", cbTarget, req, nil, "install one of tea, fj"},
		{"gh missing on github", ghTarget, req, []string{"tea", "fj"}, "install one of gh"},
		{"fj cannot label", cbTarget, CreateRequest{Title: "T", BodyFile: "b.md", Labels: []string{"bug"}}, []string{"fj"}, "fj cannot set labels"},
		{"no title", cbTarget, CreateRequest{Title: " ", BodyFile: "b.md"}, []string{"tea"}, "title is required"},
		{"no body", cbTarget, CreateRequest{Title: "T"}, []string{"tea"}, "--body-file is required"},
		{"no spec cli", Target{Forge: forge.Forge{Host: "x.org"}}, req, []string{"tea"}, "lists no issue_cli for x.org"},
	}
	for _, c := range cases {
		_, err := PlanCreate(c.target, c.req, lookOnly(c.look...))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, want %q", c.name, err, c.want)
		}
	}
}

func TestParseCreated(t *testing.T) {
	cases := []struct {
		out string
		n   int
		url string
		ok  bool
	}{
		{"https://github.com/o/r/issues/12\n", 12, "https://github.com/o/r/issues/12", true},
		{"#218 New ticket\nstate: open", 218, "https://codeberg.org/o/r/issues/218", true},
		{"see https://codeberg.org/o/r/issues/5.", 5, "https://codeberg.org/o/r/issues/5", true},
		{"error-free but silent", 0, "", false},
	}
	for _, c := range cases {
		n, url, ok := ParseCreated(cbTarget, c.out)
		if n != c.n || url != c.url || ok != c.ok {
			t.Errorf("%q: got %d %q %v", c.out, n, url, ok)
		}
	}
}

func TestCommandStringQuotes(t *testing.T) {
	c := Command{"gh", []string{"issue", "--title", "two words"}}
	if got := c.String(); got != `gh issue --title "two words"` {
		t.Fatalf("got %s", got)
	}
}
