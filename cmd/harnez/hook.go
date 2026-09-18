// agyhook implements `harnez hook agy` / `harnez hook agy-tool` (and hidden legacy `harnez agy-hook`),
// the native lifecycle observation hook for Google Antigravity (AGY).
// It accepts Antigravity's PreToolUse JSON payload on stdin, parses the tool name
// and conversationId, records the invocation into ~/.harnez/tool_catalog.sqlite,
// and returns `{"decision":"allow"}` on stdout so tool execution continues unimpeded.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/readcard"
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

type agyPostToolUseInput struct {
	ConversationID string `json:"conversationId"`
	TranscriptPath string `json:"transcriptPath"`
	Output         string `json:"output"`
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
	postCmd := &cobra.Command{
		Use:          "post-tool",
		Aliases:      []string{"agy-post"},
		Short:        "Antigravity PostToolUse telemetry hook",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgyPostToolHook(cmd.InOrStdin(), cmd.OutOrStdout(), agyPostHookOptions{})
		},
	}
	cmd.AddCommand(postCmd)

	readCmd := newReadHookCmd()
	cmd.AddCommand(readCmd)

	return cmd
}

type agyPostHookOptions struct {
	DBPath string
	Update func(dbPath, sessionID string, outputBytes, durationMs int64) error
}

func runAgyPostToolHook(in io.Reader, out io.Writer, opts agyPostHookOptions) error {
	defer fmt.Fprintln(out, `{}`)

	var payload agyPostToolUseInput
	if err := json.NewDecoder(in).Decode(&payload); err != nil {
		fmt.Fprintf(os.Stderr, "harnez post-tool hook: decode payload: %v\n", err)
		return nil
	}

	outputStr := payload.Output
	if outputStr == "" && payload.TranscriptPath != "" {
		outputStr = extractTranscriptOutput(payload.TranscriptPath)
	}
	outputBytes := int64(len([]byte(outputStr)))
	actualTok := int64(readcard.ComputeTextTokens(outputStr).TextTokens)
	var actualTokens *int64 = &actualTok

	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath, _ = telemetry.DefaultDBPath()
	}

	if opts.Update != nil {
		if err := opts.Update(dbPath, payload.ConversationID, outputBytes, 0); err != nil {
			fmt.Fprintf(os.Stderr, "harnez post-tool hook: update: %v\n", err)
		}
		return nil
	}

	db, err := telemetry.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnez post-tool hook: open db: %v\n", err)
		return nil
	}
	defer db.Close()

	var durationMs int64
	var savingsTokens, savingsBytes *int64
	calls, queryErr := db.Query(telemetry.Filter{SessionID: payload.ConversationID})
	if queryErr == nil && len(calls) > 0 {
		if !calls[0].CreatedAt.IsZero() {
			d := time.Since(calls[0].CreatedAt).Milliseconds()
			if d > 0 {
				durationMs = d
			}
		}
		if isNativeReadTool(calls[0].ToolName) && outputStr != "" {
			provider := readcard.ParseProvider(calls[0].AgentID)
			estimate := readcard.EstimateSavings(outputStr, provider)
			savingsTokens = &estimate.SavingsTokens
			savingsBytes = &estimate.SavingsBytes
		}
	}

	if err := db.UpdateLatestToolCallMetrics(payload.ConversationID, outputBytes, durationMs, actualTokens, savingsTokens, savingsBytes); err != nil {
		fmt.Fprintf(os.Stderr, "harnez post-tool hook: update metrics: %v\n", err)
	}
	return nil
}

func extractTranscriptOutput(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var step struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(line), &step); err == nil && step.Content != "" {
			return step.Content
		}
	}
	return string(data)
}

func isNativeReadTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "view_file", "readmultiplefiles", "read_multiple_files", "cat":
		return true
	default:
		return false
	}
}

type agyHookOptions struct {
	BaseDir     string
	DBPath      string
	StateDir    string
	EnforceRead *bool
	Insert      func(dbPath string, call telemetry.ToolCall) error
}

// readEnforcementEnabled reports whether native large-read interception is active.
// Enforcement remains enabled by default so existing installations keep their
// current behavior; HARNEZ_READ_ENFORCE=0 (or false/off/no) selects autonomous
// read mode without removing the observation hooks.
func readEnforcementEnabled() bool {
	return readEnforcementEnabledFor(nil)
}

func readEnforcementEnabledFor(explicit *bool) bool {
	if explicit != nil {
		return *explicit
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HARNEZ_READ_ENFORCE"))) {
	case "0", "false", "off", "no":
		return false
	case "1", "true", "on", "yes":
		return true
	}
	if home, err := os.UserHomeDir(); err == nil {
		data, err := os.ReadFile(filepath.Join(home, ".harnez", "config.yaml"))
		if err == nil {
			var cfg struct {
				ReadingDiscipline struct {
					Enforce *bool `yaml:"enforce"`
				} `yaml:"reading_discipline"`
			}
			if yaml.Unmarshal(data, &cfg) == nil && cfg.ReadingDiscipline.Enforce != nil {
				return *cfg.ReadingDiscipline.Enforce
			}
		}
	}
	return true
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

	wd := opts.BaseDir
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			wd = ""
		}
	}

	deny, reason := evaluateReadToolDisciplineWithEnforcement(toolName, payload.ToolCall.Args, wd, opts.EnforceRead)
	if deny {
		callType = "hook:deny"
		note = "reading_discipline:intercepted"
	}

	scoreVal := 5
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

	if deny {
		outObj := map[string]string{
			"decision": "deny",
			"reason":   reason,
		}
		data, _ := json.Marshal(outObj)
		fmt.Fprintln(out, string(data))
		return nil
	}

	fmt.Fprintln(out, `{"decision":"allow"}`)
	return nil
}

// readingDisciplineDenyReason is the structured guidance message returned when an agent
// attempts native IDE file viewing on large files without using harnez read (issue 405).
const readingDisciplineDenyReason = "harnez guard: native view_file on large files (>100 lines) violates Reading & Context Discipline. Execute 'harnez read -I <file>' for visual cards or 'harnez read -L <range> -n <file>' for line-bounded editing anchors."

// isReadTool reports whether toolName is a client-native file reading tool that
// should be intercepted under Reading & Context Discipline.
func isReadTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "read", "view_file", "view", "read_file", "readfile", "readmultiplefiles", "read_multiple_files":
		return true
	default:
		return false
	}
}

// extractFilePaths extracts target file path(s) from tool args/input across AGY, Claude Code, and generic tools.
func extractFilePaths(args map[string]any) []string {
	if args == nil {
		return nil
	}
	var paths []string
	for _, key := range []string{"AbsolutePath", "file_path", "path", "FilePath", "Path", "target_file", "TargetFile", "file", "File"} {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				paths = append(paths, strings.TrimSpace(s))
				break
			}
		}
	}
	for _, key := range []string{"paths", "files", "Paths", "Files"} {
		if v, ok := args[key]; ok {
			switch list := v.(type) {
			case []string:
				for _, s := range list {
					if strings.TrimSpace(s) != "" {
						paths = append(paths, strings.TrimSpace(s))
					}
				}
			case []any:
				for _, item := range list {
					if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
						paths = append(paths, strings.TrimSpace(s))
					}
				}
			}
		}
	}
	return paths
}

func parseLineNumber(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return n, true
		}
	case json.Number:
		if n, err := val.Int64(); err == nil {
			return int(n), true
		}
	}
	return 0, false
}

type readRange struct {
	hasRange  bool
	startLine int
	endLine   int
}

// extractReadRange extracts line range boundaries from tool arguments.
func extractReadRange(args map[string]any) readRange {
	if args == nil {
		return readRange{}
	}
	var r readRange
	if offset, ok := parseLineNumber(args["offset"]); ok && offset > 0 {
		r = readRange{hasRange: true, startLine: offset}
		if limit, ok := parseLineNumber(args["limit"]); ok && limit > 0 && limit <= int(^uint(0)>>1)-offset {
			r.endLine = offset + limit - 1
		}
		return r
	}
	if limit, ok := parseLineNumber(args["limit"]); ok && limit > 0 {
		return readRange{hasRange: true, startLine: 1, endLine: limit}
	}
	for _, k := range []string{"view_range", "viewRange"} {
		if v, ok := args[k]; ok {
			switch arr := v.(type) {
			case []any:
				if len(arr) == 2 {
					s, sOk := parseLineNumber(arr[0])
					e, eOk := parseLineNumber(arr[1])
					if sOk && eOk {
						r.hasRange = true
						r.startLine = s
						r.endLine = e
						return r
					}
				}
			case []int:
				if len(arr) == 2 {
					r.hasRange = true
					r.startLine = arr[0]
					r.endLine = arr[1]
					return r
				}
			case []float64:
				if len(arr) == 2 {
					r.hasRange = true
					r.startLine = int(arr[0])
					r.endLine = int(arr[1])
					return r
				}
			}
		}
	}

	for _, k := range []string{"StartLine", "start_line", "startLine"} {
		if v, ok := args[k]; ok {
			if n, ok := parseLineNumber(v); ok && n > 0 {
				r.startLine = n
				r.hasRange = true
				break
			}
		}
	}
	for _, k := range []string{"EndLine", "end_line", "endLine"} {
		if v, ok := args[k]; ok {
			if n, ok := parseLineNumber(v); ok && n > 0 {
				r.endLine = n
				r.hasRange = true
				break
			}
		}
	}
	return r
}

func countFileLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	lines := bytes.Count(data, []byte("\n"))
	if !bytes.HasSuffix(data, []byte("\n")) {
		lines++
	}
	return lines, nil
}

// isBinaryMedia reports whether path is a binary media file (image/video/audio/pdf)
// that is intended to be inspected by visual multimodal tools without line counts.
func isBinaryMedia(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".ico", ".svg", ".pdf", ".mp4", ".webm", ".mp3", ".wav":
		return true
	default:
		return false
	}
}

// evaluateReadToolDiscipline checks if a tool invocation on a target file violates
// Reading & Context Discipline (file >= 100 lines or range >= 100 lines or unconstrained whole-file read of a >=100 line file).
func evaluateReadToolDiscipline(toolName string, args map[string]any, baseDir string) (bool, string) {
	return evaluateReadToolDisciplineWithEnforcement(toolName, args, baseDir, nil)
}

func evaluateReadToolDisciplineWithEnforcement(toolName string, args map[string]any, baseDir string, enforce *bool) (bool, string) {
	if !readEnforcementEnabledFor(enforce) {
		return false, ""
	}
	if !isReadTool(toolName) {
		return false, ""
	}

	rng := extractReadRange(args)
	if rng.hasRange && rng.startLine > 0 && rng.endLine >= rng.startLine {
		if rng.endLine-rng.startLine+1 >= 100 {
			return true, readingDisciplineDenyReason
		}
	}

	paths := extractFilePaths(args)
	for _, p := range paths {
		target := p
		if !filepath.IsAbs(target) && baseDir != "" {
			target = filepath.Join(baseDir, target)
		}
		if isBinaryMedia(target) {
			// Binary media (e.g. PNG context cards) are visual inputs, not text files.
			continue
		}
		totalLines, err := countFileLines(target)
		if err != nil {
			// If file doesn't exist on disk, we can't count lines; range check above already checked explicit >= 100.
			continue
		}
		if totalLines < 100 {
			// Allowed: file is smaller than 100 lines.
			continue
		}
		// totalLines >= 100
		if !rng.hasRange {
			// Unconstrained whole-file read of a >= 100 line file!
			return true, readingDisciplineDenyReason
		}
		if rng.startLine > 0 && rng.endLine >= rng.startLine {
			if rng.endLine-rng.startLine+1 >= 100 {
				return true, readingDisciplineDenyReason
			}
		} else if rng.startLine > 0 && rng.endLine == 0 {
			if totalLines-rng.startLine+1 >= 100 {
				return true, readingDisciplineDenyReason
			}
		} else if rng.endLine > 0 && rng.startLine == 0 {
			if rng.endLine >= 100 {
				return true, readingDisciplineDenyReason
			}
		}
	}

	return false, ""
}

// claudePreToolUseInput represents the JSON payload received from Claude Code on stdin
// during a PreToolUse lifecycle event.
type claudePreToolUseInput struct {
	HookEventName string         `json:"hookEventName"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	ToolUseID     string         `json:"tool_use_id"`
	SessionID     string         `json:"session_id"`
	Cwd           string         `json:"cwd"`
}

type claudeHookOutput struct {
	HookSpecificOutput claudeHookSpecificOutput `json:"hookSpecificOutput"`
	SystemMessage      string                   `json:"systemMessage,omitempty"`
}

type claudeHookSpecificOutput struct {
	HookEventName            string `json:"hookEventName,omitempty"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

type readHookOptions struct {
	BaseDir     string
	DBPath      string
	StateDir    string
	EnforceRead *bool
	Insert      func(dbPath string, call telemetry.ToolCall) error
}

func newReadHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "read",
		Short:  "Claude Code PreToolUse file-read interception hook",
		Hidden: true,
		Long: `read accepts Claude Code's PreToolUse JSON payload on stdin for View and ReadMultipleFiles,
intercepts unconstrained or large (>=100 lines) file reads, and returns a permissionDecision of deny
with guidance to use 'harnez read'. Small files (<100 lines) and bounded slices (<100 lines) are allowed.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClaudeReadHook(cmd.InOrStdin(), cmd.OutOrStdout(), readHookOptions{})
		},
	}
	return cmd
}

func runClaudeReadHook(in io.Reader, out io.Writer, opts readHookOptions) error {
	raw, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintln(out, `{}`)
		return fmt.Errorf("read claude read hook payload: %w", err)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		fmt.Fprintln(out, `{}`)
		return nil
	}

	var payload claudePreToolUseInput
	if err := json.Unmarshal(raw, &payload); err != nil {
		fmt.Fprintln(out, `{}`)
		return fmt.Errorf("decode claude read hook payload: %w", err)
	}
	if !readEnforcementEnabledFor(opts.EnforceRead) {
		fmt.Fprintln(out, `{}`)
		return nil
	}

	baseDir := opts.BaseDir
	if baseDir == "" {
		if payload.Cwd != "" {
			baseDir = payload.Cwd
		} else {
			baseDir, _ = os.Getwd()
		}
	}

	deny, reason := evaluateReadToolDisciplineWithEnforcement(payload.ToolName, payload.ToolInput, baseDir, opts.EnforceRead)
	if isReadTool(payload.ToolName) {
		stateDir := opts.StateDir
		if stateDir == "" {
			stateDir = filepath.Join(resolve.DefaultStateDir(), "read-hooks")
		}
		seen := map[string]bool{}
		for _, path := range extractFilePaths(payload.ToolInput) {
			if !filepath.IsAbs(path) {
				path = filepath.Join(baseDir, path)
			}
			if canonical, err := filepath.EvalSymlinks(path); err == nil {
				path = canonical
			}
			if isBinaryMedia(path) || seen[path] {
				continue
			}
			seen[path] = true
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if strings.EqualFold(payload.ToolName, "read") {
				if lines, err := countFileLines(path); err == nil && lines > 100 {
					deny = true
				}
			}
			if repeatedRead(stateDir, payload.SessionID, path) {
				deny = true
			}
		}
		if deny {
			reason = readingDisciplineDenyReason + "\n" + readRedirect(payload.ToolInput, baseDir)
		}
	}
	if deny {
		resp := claudeHookOutput{
			HookSpecificOutput: claudeHookSpecificOutput{
				HookEventName:            "PreToolUse",
				PermissionDecision:       "deny",
				PermissionDecisionReason: reason,
			},
			SystemMessage: reason,
		}
		data, _ := json.Marshal(resp)
		fmt.Fprintln(out, string(data))
		return nil
	}

	fmt.Fprintln(out, `{}`)
	return nil
}

// A create-exclusive marker makes repeated reads deterministic across hook
// processes without a lost-update race. Session hashes cannot traverse paths.
func repeatedRead(stateDir, session, path string) bool {
	if session == "" || stateDir == "" {
		return false
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return false
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(session+"\x00"+path)))
	f, err := os.OpenFile(filepath.Join(stateDir, key), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		f.Close()
		return false
	}
	return os.IsExist(err)
}

func readRedirect(args map[string]any, baseDir string) string {
	command := "harnez read --auto"
	rng := extractReadRange(args)
	if rng.hasRange {
		start := max(1, rng.startLine)
		end := ""
		if rng.endLine >= start {
			end = strconv.Itoa(rng.endLine)
		}
		command = "harnez read -n -L " + strconv.Itoa(start) + ":" + end
	}
	command += " --"
	for _, path := range extractFilePaths(args) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		command += " '" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	}
	return command
}
