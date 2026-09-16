// agyhook implements `harnez hook agy` / `harnez hook agy-tool` (and hidden legacy `harnez agy-hook`),
// the native lifecycle observation hook for Google Antigravity (AGY).
// It accepts Antigravity's PreToolUse JSON payload on stdin, parses the tool name
// and conversationId, records the invocation into ~/.harnez/tool_catalog.sqlite,
// and returns `{"decision":"allow"}` on stdout so tool execution continues unimpeded.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/telemetry"
)

// agyPreToolUseInput represents the JSON payload received from Antigravity on stdin
// during a PreToolUse lifecycle event.
type agyPreToolUseInput struct {
	ConversationID string `json:"conversationId"`
	ToolCall       struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	} `json:"toolCall"`
	StepIdx int `json:"stepIdx"`
}

func newAgyHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "agy",
		Short:  "Antigravity PreToolUse tool observation hook",
		Hidden: true,
		Long: `agy accepts Antigravity's PreToolUse JSON payload on stdin,
records tool invocations into ~/.harnez/tool_catalog.sqlite under agent_id='agy',
and outputs {"decision":"allow"} to stdout.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgyToolHook(cmd.InOrStdin(), cmd.OutOrStdout(), agyHookOptions{})
		},
	}
	return cmd
}

// newHookCmd provisions the `harnez hook` command group hosting agent-specific lifecycle hooks.
func newHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hook",
		Short:  "Agent lifecycle hook handlers",
		Hidden: true,
	}
	agyCmd := newAgyHookCmd()
	agyCmd.Aliases = []string{"agy-tool"}
	cmd.AddCommand(agyCmd)
	return cmd
}

type agyHookOptions struct {
	DBPath   string
	StateDir string
	Insert   func(dbPath string, call telemetry.ToolCall) error
}

func runAgyToolHook(in io.Reader, out io.Writer, opts agyHookOptions) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintln(out, `{"decision":"allow"}`)
		return fmt.Errorf("read agy hook payload: %w", err)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		fmt.Fprintln(out, `{"decision":"allow"}`)
		return nil
	}

	var payload agyPreToolUseInput
	if err := json.Unmarshal(raw, &payload); err != nil {
		// Output allow anyway to not block the agent
		fmt.Fprintln(out, `{"decision":"allow"}`)
		return fmt.Errorf("decode agy hook payload: %w", err)
	}

	toolName := payload.ToolCall.Name
	if toolName == "" {
		toolName = "unknown"
	}

	sessionID := payload.ConversationID
	if sessionID == "" {
		sessionID, _ = resolve.Session(resolve.SessionOptions{LockDir: opts.StateDir})
	}

	ticketID := ""
	if sessionID != "" {
		ticketID, _ = resolve.Ticket(resolve.TicketOptions{
			SessionID: sessionID,
			StateDir:  opts.StateDir,
		})
	}

	callType := "hook:rpc"
	note := ""
	if toolName == "run_command" {
		callType = "hook:prep"
		if cmdStr, ok := payload.ToolCall.Args["CommandLine"].(string); ok && cmdStr != "" {
			note = "preps:Bash | " + cmdStr
		} else {
			note = "preps:Bash"
		}
	}

	scoreVal := 5
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	tc := telemetry.ToolCall{
		CreatedAt:   time.Now().UTC(),
		SessionID:   sessionID,
		TicketID:    ticketID,
		ProjectName: filepath.Base(wd),
		WorkingDir:  wd,
		AgentID:     "agy",
		ToolName:    toolName,
		CallType:    callType,
		Score:       &scoreVal,
		Note:        note,
	}

	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath, _ = telemetry.DefaultDBPath()
	}

	if dbPath != "" {
		insertFn := opts.Insert
		if insertFn == nil {
			insertFn = defaultInsertExecRow
		}
		_ = insertFn(dbPath, tc) // best-effort insert
	}

	fmt.Fprintln(out, `{"decision":"allow"}`)
	return nil
}
