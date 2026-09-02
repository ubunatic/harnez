package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestToolFeedbackProtocolConfigEntry verifies issue 122's decision: the
// Tool Feedback Protocol instruction snippet (telling agents to call
// `harnez rate`, see issue 117) is a declarative agents_md.global.sections
// entry in config.yaml, consumed by the same managed-section mechanism
// apply.go already uses for the "Instructions Hierarchy" section — not a
// new code path.
func TestToolFeedbackProtocolConfigEntry(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	var found *MDSection
	for i := range cfg.AgentsMD.Global.Sections {
		s := &cfg.AgentsMD.Global.Sections[i]
		if s.Name == "Tool Feedback Protocol" {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an agents_md.global.sections entry named %q in embedded config.yaml", "Tool Feedback Protocol")
	}
	if !strings.Contains(found.Content, "harnez rate <tool_name> <1-5>") {
		t.Errorf("expected Tool Feedback Protocol section to contain the harnez rate invocation, got:\n%s", found.Content)
	}
	// Issue 179: the section also documents the lean ok-heartbeat rule
	// alongside the original failure-only rule.
	if !strings.Contains(found.Content, "harnez rate --ok") {
		t.Errorf("expected Tool Feedback Protocol section to also document `harnez rate --ok`, got:\n%s", found.Content)
	}

	// Word-budget check, extended from issue 122's original ~25-token
	// estimate to accommodate issue 179's genuinely added second rule
	// (rough word count, not a real tokenizer). The upper bound still
	// exists to catch a regression back to a verbose, pre-181-style
	// instruction — it isn't unbounded.
	words := strings.Fields(found.Content)
	if len(words) < 15 || len(words) > 90 {
		t.Errorf("expected Tool Feedback Protocol content to stay concise (~15-90 words for two short rules), got %d words:\n%s", len(words), found.Content)
	}
}

// TestApplyInstallsToolFeedbackProtocol exercises the same install →
// idempotent re-apply → clean lifecycle TestApplyInstallsTelemetryHook (119)
// covers for the telemetry hook, focused on the new Tool Feedback Protocol
// managed section. It also checks the section reaches both global agent
// targets (~/.claude/CLAUDE.md-equivalent and Prime Agent's AGENTS.md).
func TestApplyInstallsToolFeedbackProtocol(t *testing.T) {
	targetDir := t.TempDir()

	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	// Isolate every other target this apply run would touch so this test
	// only exercises the global AGENTS.md/CLAUDE.md managed sections.
	claudeMDPath := filepath.Join(t.TempDir(), "CLAUDE.md")
	primeAgentDir := filepath.Join(t.TempDir(), "prime-agent")
	cfg.SkillsTarget = filepath.Join(t.TempDir(), "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(t.TempDir(), "codex-skills")
	cfg.ClaudeSkillsTarget = filepath.Join(t.TempDir(), "claude-skills")
	cfg.PrimeAgentTarget = primeAgentDir
	cfg.AgentsMD.Global.Target = claudeMDPath
	cfg.AgentsMD.Global.Symlink = ""
	cfg.DistillAutopipe.PiExtensionTarget = filepath.Join(t.TempDir(), "pi", "harnez-distill.ts")
	cfg.DistillAutopipe.OpenCodePluginTarget = filepath.Join(t.TempDir(), "opencode", "harnez-distill.ts")

	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("first ApplyAll failed: %v", err)
	}

	primeAGENTSPath := filepath.Join(primeAgentDir, "AGENTS.md")
	for _, path := range []string{claudeMDPath, primeAGENTSPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s to be written: %v", path, err)
		}
		content := string(data)
		if !strings.Contains(content, "<!-- harnez:begin Tool Feedback Protocol -->") {
			t.Errorf("expected %s to contain the Tool Feedback Protocol managed section markers, got:\n%s", path, content)
		}
		if !strings.Contains(content, "### Tool Feedback Protocol") {
			t.Errorf("expected %s to contain the Tool Feedback Protocol heading, got:\n%s", path, content)
		}
		if !strings.Contains(content, "harnez rate <tool_name> <1-5>") {
			t.Errorf("expected %s to contain the harnez rate invocation, got:\n%s", path, content)
		}
	}

	// Idempotency: a second apply must not duplicate the section.
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("second ApplyAll failed: %v", err)
	}
	countOccurrences := func(path, substr string) int {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return strings.Count(string(data), substr)
	}
	for _, path := range []string{claudeMDPath, primeAGENTSPath} {
		if n := countOccurrences(path, "<!-- harnez:begin Tool Feedback Protocol -->"); n != 1 {
			t.Errorf("expected exactly one Tool Feedback Protocol section in %s after repeated apply, got %d", path, n)
		}
	}

	// clean must remove the managed section from both targets, the same way
	// it removes every other agents_md.global.sections entry.
	if err := CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	for _, path := range []string{claudeMDPath, primeAGENTSPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // whole file removed because it only held managed content
			}
			t.Fatalf("unexpected error reading %s after clean: %v", path, err)
		}
		if strings.Contains(string(data), "Tool Feedback Protocol") {
			t.Errorf("expected Tool Feedback Protocol section to be removed from %s by CleanAll, got:\n%s", path, data)
		}
	}
}

func TestIssueTrackerDiscoveryConfigEntry(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	var found *MDSection
	for i := range cfg.AgentsMD.Global.Sections {
		s := &cfg.AgentsMD.Global.Sections[i]
		if s.Name == "Issue Tracker Discovery" {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an agents_md.global.sections entry named %q in embedded config.yaml", "Issue Tracker Discovery")
	}
	if !strings.Contains(found.Content, "harnez find -d <repo> issues status:open") {
		t.Errorf("expected Issue Tracker Discovery section to contain harnez find status:open, got:\n%s", found.Content)
	}
	if !strings.Contains(found.Content, "harnez index -d <repo>") {
		t.Errorf("expected Issue Tracker Discovery section to contain harnez index, got:\n%s", found.Content)
	}
}

// TestAgentFeedbackConfigEntry verifies issue 185's decision: the
// "reverse-feedback" instruction telling agents to call `harnez feedback`
// (issue 184) when they themselves observe a genuine harnez bug or a
// broken/contradictory instruction is a declarative
// agents_md.global.sections entry in config.yaml, mirroring how the Tool
// Feedback Protocol (issue 122) and Issue Tracker Discovery sections are
// defined, rather than a new code path. It also enforces issue 181's
// word-budget discipline (~50-60 words) that this ticket was asked to
// match, and checks the instruction is narrowly event-triggered rather than
// a blanket "log anything" mandate.
func TestAgentFeedbackConfigEntry(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	var found *MDSection
	for i := range cfg.AgentsMD.Global.Sections {
		s := &cfg.AgentsMD.Global.Sections[i]
		if s.Name == "Agent-Filed Feedback" {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an agents_md.global.sections entry named %q in embedded config.yaml", "Agent-Filed Feedback")
	}
	if !strings.Contains(found.Content, "harnez feedback issue") {
		t.Errorf("expected Agent-Filed Feedback section to contain the harnez feedback invocation, got:\n%s", found.Content)
	}
	if !strings.Contains(found.Content, "not routine friction") {
		t.Errorf("expected Agent-Filed Feedback section to scope the trigger narrowly (not routine friction), got:\n%s", found.Content)
	}

	// Word-budget discipline from issue 181's resolution note, ~50-60 words
	// (rough word count, not a real tokenizer — matches
	// TestToolFeedbackProtocolConfigEntry's own tolerance band).
	words := strings.Fields(found.Content)
	if len(words) < 40 || len(words) > 65 {
		t.Errorf("expected Agent-Filed Feedback content to be roughly 50-60 words, got %d words:\n%s", len(words), found.Content)
	}
}

