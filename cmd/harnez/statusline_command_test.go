package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestAppliedHarnezCommandsResolveOnRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("load embedded config: %v", err)
	}
	target := filepath.Join(home, ".claude")
	if err := claude.ApplyAll(target, cfg, nil, false, false, false); err != nil {
		t.Fatalf("apply embedded config: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, "settings.json"))
	if err != nil {
		t.Fatalf("read applied settings: %v", err)
	}
	var settings struct {
		StatusLine map[string]any `json:"statusLine"`
		Hooks      any            `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse applied settings: %v", err)
	}

	commands := []string{}
	if command, _ := settings.StatusLine["command"].(string); strings.HasPrefix(command, "harnez ") {
		commands = append(commands, command)
	}
	commands = appendHarnezCommands(commands, settings.Hooks)
	if len(commands) == 0 {
		t.Fatal("apply output contained no harnez statusLine or hook commands")
	}

	root := newRootCmd()
	for _, command := range commands {
		tokens := strings.Fields(command)
		if len(tokens) < 2 || tokens[0] != "harnez" {
			t.Fatalf("unexpected configured command %q", command)
		}
		resolved, remaining, err := root.Find(tokens[1:])
		wantPath := "harnez " + strings.Join(tokens[1:], " ")
		if err != nil || len(remaining) != 0 || resolved.CommandPath() != wantPath {
			t.Errorf("configured command %q resolved to %q with remaining %v (error %v)", command, resolved.CommandPath(), remaining, err)
		}
	}
}

func appendHarnezCommands(commands []string, value any) []string {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "command" {
				if command, ok := child.(string); ok && strings.HasPrefix(command, "harnez ") {
					commands = append(commands, command)
				}
			}
			commands = appendHarnezCommands(commands, child)
		}
	case []any:
		for _, child := range v {
			commands = appendHarnezCommands(commands, child)
		}
	}
	return commands
}

func TestStatuslineCommandAcceptsTicketPayload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd := newRootCmd()
	cmd.SetArgs([]string{"statusline"})
	cmd.SetIn(strings.NewReader(`{"workspace":{"current_dir":"/tmp/project","project_dir":"/tmp","added_dirs":[],"repo":{"host":"codeberg.org","owner":"uwe","name":"harnez"}},"cwd":"/tmp/project","session_id":"session-1","session_name":"test","transcript_path":"/tmp/transcript"}`))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("harnez statusline exited with error: %v", err)
	}
}
