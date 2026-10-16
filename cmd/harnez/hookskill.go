package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// claudeSkillHookInput is the part of Claude Code's PostToolUse,
// PostToolUseFailure and UserPromptSubmit payloads the skill hook reads.
type claudeSkillHookInput struct {
	HookEventName string `json:"hook_event_name"`
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		Skill string `json:"skill"`
	} `json:"tool_input"`
	Prompt string `json:"prompt"`
}

type skillHookOptions struct {
	DBPath string
	Home   string
	Insert func(dbPath string, call telemetry.ToolCall) error
}

func newSkillHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "skill",
		Short:  "Claude Code hook that records skill calls",
		Hidden: true,
		Long: `skill reads a Claude Code PostToolUse or PostToolUseFailure payload for the Skill tool,
or a UserPromptSubmit payload, and records one tool_calls row with tool_name Skill and the skill
name (issue 735). A typed /name counts only when an installed skill or command has that name.
It prints nothing, so it never adds to the prompt, and it never fails the agent's turn.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runClaudeSkillHook(cmd.InOrStdin(), skillHookOptions{})
			return nil
		},
	}
}

// runClaudeSkillHook records the skill call in the payload, if any. It is
// best-effort: errors are dropped so the hook never blocks a prompt.
func runClaudeSkillHook(in io.Reader, opts skillHookOptions) {
	var payload claudeSkillHookInput
	if err := json.NewDecoder(in).Decode(&payload); err != nil {
		return
	}
	home := opts.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	var name, callType string
	switch payload.HookEventName {
	case "PostToolUse", "PostToolUseFailure":
		if payload.ToolName != telemetry.SkillToolName {
			return
		}
		name = telemetry.NormalizeSkillName(payload.ToolInput.Skill)
		callType = "hook:post"
		if payload.HookEventName == "PostToolUseFailure" {
			callType = "hook:failure"
		}
	case "UserPromptSubmit":
		name = typedSkillName(payload.Prompt, payload.Cwd, home)
		callType = telemetry.CallTypeSkillPrompt
	}
	if name == "" {
		return
	}
	ticket, _ := resolve.Ticket(resolve.TicketOptions{SessionID: payload.SessionID})
	call := telemetry.ToolCall{
		CreatedAt:   time.Now().UTC(),
		SessionID:   payload.SessionID,
		TicketID:    ticket,
		ProjectName: filepath.Base(payload.Cwd),
		WorkingDir:  payload.Cwd,
		AgentID:     "claude",
		ToolName:    telemetry.SkillToolName,
		CallType:    callType,
		SkillName:   name,
	}
	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath, _ = telemetry.DefaultDBPath()
	}
	if dbPath == "" {
		return
	}
	insert := opts.Insert
	if insert == nil {
		insert = defaultInsertExecRow
	}
	_ = insert(dbPath, call)
}

// typedSkillName returns the skill a prompt invokes as /name, or "" when the
// prompt does not start with a plain /name (a path such as /tmp/a.md is not
// one) or no installed skill or command has that name. Claude Code's
// built-ins (/clear, /model, ...) do not reach UserPromptSubmit (issue 735
// probe), and the installed-name check keeps them out if that changes.
// Only file stats, so the hook stays fast on every prompt.
func typedSkillName(prompt, cwd, home string) string {
	prompt = strings.TrimSpace(prompt)
	if !strings.HasPrefix(prompt, "/") {
		return ""
	}
	name := telemetry.NormalizeSkillName(prompt)
	if name == "" || !claudeSkillInstalled(name, cwd, home) {
		return ""
	}
	return name
}

// claudeSkillInstalled reports whether a user, project or plugin skill or
// command is named name (plugin:skill for a plugin's).
func claudeSkillInstalled(name, cwd, home string) bool {
	var candidates []string
	if plugin, skill, ok := strings.Cut(name, ":"); ok {
		for _, root := range []string{
			filepath.Join(home, ".claude", "plugins", "cache", "*", plugin, "*"),
			filepath.Join(home, ".claude", "plugins", "marketplaces", "*", "plugins", plugin),
		} {
			for _, pattern := range []string{filepath.Join(root, "skills", skill, "SKILL.md"), filepath.Join(root, "commands", skill+".md")} {
				if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
					return true
				}
			}
		}
		return false
	}
	for _, base := range []string{home, cwd} {
		if base == "" {
			continue
		}
		candidates = append(candidates,
			filepath.Join(base, ".claude", "skills", name, "SKILL.md"),
			filepath.Join(base, ".claude", "commands", name+".md"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}
