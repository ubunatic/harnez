package claude

import (
	"fmt"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
)

func TestClaudeInstructionsHookIsClaudeOnly(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded: %v", err)
	}

	found := false
	for _, hook := range cfg.Hooks {
		if hook.Event == "SessionStart" && hook.Command == "harnez hook claude-instructions" {
			found = true
		}
	}
	if !found {
		t.Fatal("Claude SessionStart instructions hook missing from config")
	}

	claudeSettings := fmt.Sprint(buildSettingsDoc(cfg, nil))
	if !strings.Contains(claudeSettings, "harnez hook claude-instructions") {
		t.Fatal("Claude settings do not contain the instruction hook")
	}
	for agent, output := range map[string]string{
		"Codex": fmt.Sprint(codex.BuildHooksDoc()),
		"agy":   fmt.Sprint(agy.BuildHooksDoc()),
	} {
		if strings.Contains(output, "claude-instructions") || strings.Contains(output, "CxxxE.md") {
			t.Errorf("Claude-only instruction leaked into %s hook output", agent)
		}
	}
}
