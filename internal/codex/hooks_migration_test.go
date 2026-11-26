package codex

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/BurntSushi/toml"
)

// legacyConfig is a config.toml written by earlier harnez versions (bare,
// absolute-path and env-prefixed `codex-hook`/`codex-telemetry` commands, plus
// the legacy [hooks.harnez] table) next to foreign hooks that must survive.
const legacyConfig = `model = "gpt-5"

[features]
hooks = true

[hooks.harnez]
type = "command"
command = "harnez codex-hook"

[hooks.lint]
type = "command"
command = "/opt/lint.sh"

[[hooks.PreToolUse]]
matcher = "Bash"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "harnez codex-hook"

[[hooks.PreToolUse]]
matcher = "Bash"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "/home/u/go/bin/harnez codex-hook"

[[hooks.PreToolUse]]
matcher = "Bash"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "/usr/local/bin/user-guard"

[[hooks.PostToolUse]]
matcher = "*"

[[hooks.PostToolUse.hooks]]
type = "command"
command = "env HARNEZ_TELEMETRY=1 harnez codex-telemetry"

[[hooks.Stop]]

[[hooks.Stop.hooks]]
type = "command"
command = "harnez codex-telemetry"

[[hooks.Stop]]

[[hooks.Stop.hooks]]
type = "command"
command = "/opt/notify.sh"
`

// commandsIn lists every handler command in a decoded config.toml, sorted.
func commandsIn(t *testing.T, path string) []string {
	t.Helper()
	var doc map[string]any
	if _, err := toml.DecodeFile(path, &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]any)
	var out []string
	for _, v := range hooks {
		if named, ok := v.(map[string]any); ok {
			if cmd, ok := named["command"].(string); ok {
				out = append(out, cmd)
			}
			continue
		}
		for _, g := range tableArray(v) {
			for _, h := range tableArray(g["hooks"]) {
				cmd, _ := h["command"].(string)
				out = append(out, cmd)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Apply replaces every old harnez command (bare, absolute path, env prefix,
// legacy named table) with the current entries and keeps foreign ones. A second
// Apply changes nothing (issue 1304 M3 follow-up 7).
func TestApplyReplacesLegacyHarnezCommandsAndKeepsForeign(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(legacyConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := Apply(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first Apply reported no change")
	}
	want := []string{
		"/opt/lint.sh",
		"/opt/notify.sh",
		"/usr/local/bin/user-guard",
		"harnez hook codex PostCompact",
		"harnez hook codex PostToolUse",
		"harnez hook codex PreCompact",
		"harnez hook codex PreToolUse",
		"harnez hook codex SessionEnd",
		"harnez hook codex SessionStart",
		"harnez hook codex Stop",
	}
	if got := commandsIn(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands after Apply\n got %q\nwant %q", got, want)
	}
	changed, err = Apply(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second Apply changed the file")
	}
}

// Remove deletes the harnez commands in every form and keeps the foreign ones.
func TestRemoveDropsLegacyAndCurrentHarnezCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(legacyConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(path); err != nil {
		t.Fatal(err)
	}
	changed, err := Remove(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Remove reported no change")
	}
	want := []string{"/opt/lint.sh", "/opt/notify.sh", "/usr/local/bin/user-guard"}
	if got := commandsIn(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands after Remove\n got %q\nwant %q", got, want)
	}
}

// Remove on a legacy file that was never applied by this version drops every
// harnez form and keeps the foreign commands (issue 1304 M4 item 4).
func TestRemoveLegacyFixtureWithoutApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(legacyConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := Remove(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Remove reported no change on the legacy fixture")
	}
	want := []string{"/opt/lint.sh", "/opt/notify.sh", "/usr/local/bin/user-guard"}
	if got := commandsIn(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands after Remove\n got %q\nwant %q", got, want)
	}
}

// mixedGroupConfig has one Bash group that holds a foreign handler and a legacy
// harnez handler. Ownership is per handler, so the foreign one must survive.
const mixedGroupConfig = `[features]
hooks = true

[[hooks.PreToolUse]]
matcher = "Bash"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "/opt/user-lint"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "harnez codex-hook"
`

// A mixed group keeps its foreign handler on Apply and on Remove, and Apply
// adds the harnez PreToolUse group exactly once (issue 1304 M4 item 3).
func TestMixedGroupKeepsForeignHandlerOnApplyAndRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(mixedGroupConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(path); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/opt/user-lint",
		"harnez hook codex PostCompact",
		"harnez hook codex PostToolUse",
		"harnez hook codex PreCompact",
		"harnez hook codex PreToolUse",
		"harnez hook codex SessionEnd",
		"harnez hook codex SessionStart",
		"harnez hook codex Stop",
	}
	if got := commandsIn(t, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands after Apply\n got %q\nwant %q", got, want)
	}
	if installed, _ := Status(path); !installed {
		t.Fatal("Status false after Apply on a mixed group")
	}
	changed, err := Apply(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second Apply duplicated the mixed group")
	}

	if _, err := Remove(path); err != nil {
		t.Fatal(err)
	}
	if got, want := commandsIn(t, path), []string{"/opt/user-lint"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("commands after Remove\n got %q\nwant %q", got, want)
	}
	if installed, _ := Status(path); installed {
		t.Fatal("Status true after Remove")
	}
	var doc map[string]any
	if _, err := toml.DecodeFile(path, &doc); err != nil {
		t.Fatal(err)
	}
	groups := tableArray(doc["hooks"].(map[string]any)["PreToolUse"])
	if len(groups) != 1 || groups[0]["matcher"] != "Bash" {
		t.Fatalf("PreToolUse groups after Remove = %v, want the user's Bash group", groups)
	}
}
