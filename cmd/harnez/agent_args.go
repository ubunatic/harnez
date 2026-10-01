package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/resolve"
	"ubunatic.com/harnez/internal/subagent"
)

// currentHostSession returns the caller's host session id. HARNEZ_SESSION_ID
// (set for workers) and AGY_CONVERSATION_ID keep their historical priority so
// stored ParentSessionID values stay valid; then agent-provided ids (Claude,
// Codex). Only Codex, which exports a thread id but no session id, falls back
// to the stable PPID lock, so plain terminals keep an empty id (unscoped list).
func currentHostSession() string {
	if v := os.Getenv("HARNEZ_SESSION_ID"); v != "" {
		return v
	}
	if v := os.Getenv("AGY_CONVERSATION_ID"); v != "" {
		return v
	}
	sess, err := resolve.Session(resolve.SessionOptions{DisableFallback: os.Getenv("CODEX_THREAD_ID") == ""})
	if err != nil {
		return ""
	}
	return sess
}

// canManageTarget gates an explicit --name/id target. Host sessions (no
// HARNEZ_SESSION_ID) may address any agent, including legacy agents with an
// empty parent and agents of other hosts (hand-off); harnez leaf workers stay
// bound to their lineage. Bulk operations keep using subagent.CanManage.
func canManageTarget(callerParentID string, target *subagent.Session) bool {
	if os.Getenv(agentSessionEnv) == "" {
		return true
	}
	return subagent.CanManage(callerParentID, target)
}

// hostSessionTrackingLine summarizes the agents a host session started or
// resumed (issue 656 M1), derived from Session.ParentSessionID and
// Session.LastHostSessionID. It returns "" without a host id or agents.
func hostSessionTrackingLine(store *subagent.FileSessionStore, hostSessionID string) string {
	if store == nil || hostSessionID == "" {
		return ""
	}
	sessions, err := store.List(hostSessionID, false)
	if err != nil || len(sessions) == 0 {
		return ""
	}
	var names []string
	runningCount := 0
	for _, s := range sessions {
		if s.Status == "running" || (s.Status == "active" && s.HarnessType == "interactive") {
			runningCount++
		}
		if s.Name != "" {
			names = append(names, s.Name)
		} else if s.ID != "" {
			names = append(names, s.ID)
		}
	}
	sort.Strings(names)
	total := len(sessions)
	agentWord := "agents"
	if total == 1 {
		agentWord = "agent"
	}
	return fmt.Sprintf("harnez: this session started %d %s (%d running): %s", total, agentWord, runningCount, strings.Join(names, ", "))
}

// assemblePrompt joins prompt files, positional words, and the verbatim tail.
func assemblePrompt(files []string, words []string, afterDash []string, stdin io.Reader) (string, error) {
	parts := make([]string, 0, len(files)+2)
	for _, name := range files {
		var data []byte
		var err error
		if name == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			return "", fmt.Errorf("read prompt file %q: %w", name, err)
		}
		parts = append(parts, string(data))
	}
	if len(words) > 0 {
		parts = append(parts, strings.Join(words, " "))
	}
	if len(afterDash) > 0 {
		parts = append(parts, strings.Join(afterDash, " "))
	}
	prompt := strings.Join(parts, "\n\n")
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("no prompt given")
	}
	return prompt, nil
}

func oldStyleModelWord(word string) bool {
	for _, provider := range []string{"codex", "claude", "agy", "local"} {
		if strings.HasPrefix(word, provider+":") && len(word) > len(provider)+1 {
			return true
		}
	}
	return false
}

func promptStorage(files, words, tail []string, prompt string) string {
	if len(files) == 0 {
		return prompt
	}
	parts := make([]string, len(files))
	for i, name := range files {
		if name == "-" {
			parts[i] = "- (stdin)"
			continue
		}
		if info, err := os.Stat(name); err == nil {
			parts[i] = fmt.Sprintf("%s (%d bytes)", name, info.Size())
		} else {
			parts[i] = name
		}
	}
	if len(words) > 0 {
		parts = append(parts, strings.Join(words, " "))
	}
	if len(tail) > 0 {
		parts = append(parts, strings.Join(tail, " "))
	}
	return strings.Join(parts, "\n\n")
}

func resolveSession(store *subagent.FileSessionStore, name, dir string) (*subagent.Session, error) {
	session, err := store.Find(name)
	if err != nil {
		return nil, err
	}
	if dir == "" || dir == "." {
		return session, nil
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve session directory: %w", err)
	}
	have, err := filepath.Abs(session.WorkingDir)
	if err != nil || want != have {
		return nil, fmt.Errorf("session %q is outside directory %q", session.Name, dir)
	}
	return session, nil
}

func promptArgs(cmdArgs []string, dash int) ([]string, []string) {
	if dash < 0 || dash > len(cmdArgs) {
		return cmdArgs, nil
	}
	return cmdArgs[:dash], cmdArgs[dash:]
}

// noArgs rejects positional arguments and says what replaced them.
func noArgs(hint string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return fmt.Errorf("%s: unexpected argument %q; %s", cmd.Name(), args[0], hint)
	}
}

// silenceUsage keeps cobra's usage dump out of runtime and argument errors:
// agent hosts read this output, and every error message names the fix.
func silenceUsage(cmd *cobra.Command) {
	cmd.SilenceUsage = true
	for _, c := range cmd.Commands() {
		silenceUsage(c)
	}
}

// withHostTrackingLine wraps c.RunE so that a successful start or resume
// prints the host's tracking line to stderr. skip reports internal worker
// runs, which must stay silent.
func withHostTrackingLine(c *cobra.Command, store func() (*subagent.FileSessionStore, error), parent func() string, skip func() bool) {
	run := c.RunE
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if err := run(cmd, args); err != nil {
			return err
		}
		if skip() {
			return nil
		}
		s, err := store()
		if err != nil {
			return nil
		}
		if line := hostSessionTrackingLine(s, parent()); line != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), line)
		}
		return nil
	}
}
