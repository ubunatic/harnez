package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/channel"
)

func TestComposeOutputFromSpec(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/channel.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec channel.Config
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, want string }{
		{"claude-channels-prompt", spec.Instructions + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newComposeCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{tc.name})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("output = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestComposeArgsDefaultName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my repo")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for range 3 {
		cmd := newComposeCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"claude-channels-args"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(`^-n '(my repo-[a-z0-9]{4})' (.*)\n$`).FindStringSubmatch(out.String())
		if m == nil {
			t.Fatalf("output = %q", out.String())
		}
		if want := "--channels " + shellQuote("plugin:"+cfg.PluginName+"@"+cfg.MarketplaceName) + " --append-system-prompt " + shellQuote(cfg.Instructions); m[2] != want {
			t.Fatalf("args = %q, want %q", m[2], want)
		}
		names[m[1]] = true
	}
	if len(names) < 2 {
		t.Fatalf("names not random: %v", names)
	}
}

func TestComposeNames(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		cmd := newComposeCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{})
		if unknown {
			cmd.SetArgs([]string{"unknown"})
		}
		err := cmd.Execute()
		if (err != nil) != unknown {
			t.Fatalf("unknown=%v: error=%v", unknown, err)
		}
		for _, name := range []string{"claude-channels-prompt", "claude-channels-args"} {
			if !strings.Contains(out.String(), name) {
				t.Fatalf("missing %s: %q", name, out.String())
			}
		}
	}
}

func TestComposeSetupPrompt(t *testing.T) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var setup, compose bytes.Buffer
	if err := printChannelSetup(&setup, cfg, "/usr/bin/harnez"); err != nil {
		t.Fatal(err)
	}
	cmd := newComposeCmd()
	cmd.SetOut(&compose)
	cmd.SetArgs([]string{"claude-channels-prompt"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(setup.String(), "\n") {
		_, quoted, ok := strings.Cut(line, "--append-system-prompt ")
		if !ok {
			continue
		}
		prompt, err := strconv.Unquote(quoted)
		if err != nil || prompt+"\n" != compose.String() {
			t.Fatalf("setup prompt %q differs from compose %q: %v", quoted, compose.String(), err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("checked %d setup prompts, want both launch routes", count)
	}
}

func TestComposeArgsShellRoundTrip(t *testing.T) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, prompt := range []string{cfg.Instructions, cfg.Instructions + "\nIt's \"quoted\"; $HOME `exit 91` $(exit 92) \\ end"} {
		cfg.Instructions = prompt
		// A shell builtin captures arguments; no Claude process is launched.
		script := "set -- " + composeChannelArgs(cfg, "it's a repo-ab12") + "\nprintf '%s\\000' \"$@\""
		out, err := exec.Command("sh", "-c", script).Output()
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
		want := []string{"-n", "it's a repo-ab12", "--channels", "plugin:" + cfg.PluginName + "@" + cfg.MarketplaceName, "--append-system-prompt", prompt}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("shell arguments = %q, want %q", got, want)
		}
	}
}

func TestComposeCommandNoCollision(t *testing.T) {
	root := newRootCmd()
	count := 0
	for _, cmd := range root.Commands() {
		if cmd.Name() == "compose" {
			count++
		}
		for _, alias := range cmd.Aliases {
			if alias == "compose" {
				t.Fatalf("compose collides with alias of %s", cmd.Name())
			}
		}
	}
	if count != 1 {
		t.Fatalf("compose registered %d times", count)
	}
}
