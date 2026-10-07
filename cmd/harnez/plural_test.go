package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestRewritePluralCommands(t *testing.T) {
	root := newRootCmd()
	for _, tc := range []struct{ in, want string }{
		{"channels list", "channel list"},
		{"agents send --name x hi", "agent send --name x hi"},
		{"agent sends hi", "agent send hi"},
		{"skills", "skills"},                                               // existing command wins over skill+s
		{"issues list", "issues list"},                                     // already a command
		{"agent send --name channels hi", "agent send --name channels hi"}, // stops at first flag
		{"agent send channels", "agent send channels"},                     // leaf positional stays
		{"-d . channels list", "-d . channels list"},                       // leading flag: no rewrite
		{"s", "s"},
		{"", ""},
	} {
		got := rewritePluralCommands(root, strings.Fields(tc.in))
		if want := strings.Fields(tc.want); !reflect.DeepEqual(got, want) && !(len(got) == 0 && len(want) == 0) {
			t.Errorf("%q -> %q, want %q", tc.in, got, want)
		}
	}
}

// Walks the whole tree: every plural either resolves to its singular command or is a
// distinct existing command; never to anything else.
func TestPluralResolvesForEveryCommand(t *testing.T) {
	root := newRootCmd()
	for _, cmd := range root.Commands() {
		plural := cmd.Name() + "s"
		got := rewritePluralCommands(root, []string{plural})[0]
		if existing := findChild(root, plural); existing != nil {
			if got != plural {
				t.Errorf("%s shadowed by %s", plural, got)
			}
			continue
		}
		if got != cmd.Name() {
			t.Errorf("%s -> %s, want %s", plural, got, cmd.Name())
		}
	}
}

func TestPluralNotInHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"channel", "--help"}} {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "channels") {
			t.Fatalf("help %v mentions plural:\n%s", args, out.String())
		}
	}
}
