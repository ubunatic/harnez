package telemetry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ActivityCategory is the canonical closed enum taxonomy for tool executions
// (issue 212). It classifies tool calls into 9 safe categories suitable for
// public visual analytics and dashboards without leaking free-form note text.
type ActivityCategory string

const (
	CategoryTest       ActivityCategory = "test"       // go test, pytest, jest, assertion checks
	CategoryBuild      ActivityCategory = "build"      // go build, cargo, gcc, syntax/compiler runs
	CategoryEdit       ActivityCategory = "edit"       // file edits, patches, refactors
	CategoryInspection ActivityCategory = "inspection" // read, grep, find, view_file
	CategoryGit        ActivityCategory = "git"        // status, diff, commit, branch, log
	CategoryDebug      ActivityCategory = "debug"      // reproducing crashes, inspecting panics, traces
	CategoryWorkflow   ActivityCategory = "workflow"   // agent handoff, issue tracking, protocol injection
	CategoryConfig     ActivityCategory = "config"     // settings, hooks, dotfiles, environment setup
	CategoryOther      ActivityCategory = "other"      // unclassified fallback
)

// IsValid reports whether cat is one of the valid canonical activity categories.
func (c ActivityCategory) IsValid() bool {
	switch c {
	case CategoryTest, CategoryBuild, CategoryEdit, CategoryInspection,
		CategoryGit, CategoryDebug, CategoryWorkflow, CategoryConfig, CategoryOther:
		return true
	default:
		return false
	}
}

var (
	// Category-specific regexes used by ClassifyTier1 (sub-microsecond deterministic matching)
	reWorkflowNote = regexp.MustCompile(`(?i)(?:committed\s+(?:ticket|issue|study|synthesis|changes|updates|worktree)|updated\s+(?:issues/)?readme|filed\s+(?:ticket|issue)|created\s+issue|reserving\s+issue|loaded\s+tool\s+feedback|loaded\s+evergreen|agent\s+handoff|inline-reviewed\s+fresh-sprint|subagent|handing\s+work\s+to\s+subagent)`)
	reGitNote      = regexp.MustCompile(`(?i)(?:^git\s+|git\s+status|git\s+diff|git\s+commit|git\s+log|git\s+checkout|git\s+branch|working\s+tree\s+is\s+clean|clean\s+tree|commits?\s+ahead)`)
	reConfigNote   = regexp.MustCompile(`(?i)(?:settings\.json|dotfiles|env\s+setup|configure|configuration|environment\s+setup|agy-hooks|\.gitconfig)`)
	reTestNote     = regexp.MustCompile(`(?i)(?:go\s+test|pytest|jest|test\s+failure|tests?\s+pass|test\s+passed|unit\s+tests|all\s+tests|assertions?)`)
	reBuildNote    = regexp.MustCompile(`(?i)(?:go\s+build|cargo\s+build|make\s+install|make\s+build|compile\s+error|build\s+error|syntax\s+error|type\s+error)`)
	reDebugNote    = regexp.MustCompile(`(?i)(?:panic|segfault|segmentation\s+fault|traceback|uncaught\s+exception|core\s+dumped|diagnos|debug|profil|stack\s+trace)`)
	reInspectNote  = regexp.MustCompile(`(?i)(?:read\s+issue|inspected\s+issue|view_file|read_file|grep_search|find_by_name|list_dir)`)
)

// ClassifyTier1 performs fast (< 1ms, typically < 1µs) deterministic rule-based
// classification of a tool call based on its tool name, note text, exit code,
// and command patterns. Returns the matched ActivityCategory, or CategoryOther
// if no deterministic rule matches.
func ClassifyTier1(toolName, note string, exitCode *int) ActivityCategory {
	lowerTool := strings.ToLower(strings.TrimSpace(toolName))
	trimmedNote := strings.TrimSpace(note)

	// 1. Tool name direct heuristics
	switch lowerTool {
	case "edit", "apply_patch", "write", "write_to_file", "replace_file_content":
		return CategoryEdit
	case "read", "grep", "grep_search", "find", "find_by_name", "list_dir", "view_file", "read_url_content", "web", "web__run":
		return CategoryInspection
	case "test", "testtool":
		return CategoryTest
	case "agent", "agentlifecycle", "list_agents", "plan":
		return CategoryWorkflow
	}

	// 2. Output / Note signatures from score.go or error patterns
	if reGoPanic.MatchString(trimmedNote) || reSegfault.MatchString(trimmedNote) ||
		rePyCrash.MatchString(trimmedNote) || reNodeCrash.MatchString(trimmedNote) {
		return CategoryDebug
	}

	if reGoTestFail.MatchString(trimmedNote) || reTestFail.MatchString(trimmedNote) {
		return CategoryTest
	}

	if reGoBuild.MatchString(trimmedNote) || rePySyntax.MatchString(trimmedNote) ||
		reNodeSyntax.MatchString(trimmedNote) || reTypeSyntax.MatchString(trimmedNote) ||
		reRustBuild.MatchString(trimmedNote) || reCBuild.MatchString(trimmedNote) {
		return CategoryBuild
	}

	// 3. Regex matches on note content
	if trimmedNote != "" {
		if reWorkflowNote.MatchString(trimmedNote) {
			return CategoryWorkflow
		}
		if reGitNote.MatchString(trimmedNote) {
			return CategoryGit
		}
		if reTestNote.MatchString(trimmedNote) {
			return CategoryTest
		}
		if reBuildNote.MatchString(trimmedNote) {
			return CategoryBuild
		}
		if reDebugNote.MatchString(trimmedNote) {
			return CategoryDebug
		}
		if reInspectNote.MatchString(trimmedNote) {
			return CategoryInspection
		}
		if reConfigNote.MatchString(trimmedNote) {
			return CategoryConfig
		}
	}

	// 4. ScoreShell notes evaluation
	switch trimmedNote {
	case "clean success":
		// Clean success with a generic shell tool could be anything, but let's check tool
		if lowerTool == "bash" || lowerTool == "exec_command" || lowerTool == "functions.exec" {
			return CategoryOther
		}
	case "go runtime panic / fatal error", "segmentation fault / fatal signal",
		"python traceback / uncaught exception", "unhandled promise rejection / fatal error":
		return CategoryDebug
	case "go build / syntax error", "python syntax / type error",
		"javascript syntax / type error", "syntax / type error",
		"rust compile error", "c/c++ build error":
		return CategoryBuild
	case "go test failure", "test failure", "linter violation":
		return CategoryTest
	case "command not found / permission denied":
		return CategoryOther
	}

	if exitCode != nil && *exitCode > 128 {
		// Signal termination
		return CategoryDebug
	}

	return CategoryOther
}

// GetCachedCategories looks up hashes in note_category_cache, returning a
// hash -> ActivityCategory map for hits.
func (d *DB) GetCachedCategories(ctx context.Context, hashes []string) (map[string]ActivityCategory, error) {
	out := make(map[string]ActivityCategory, len(hashes))
	if len(hashes) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(hashes))
	args := make([]any, len(hashes))
	for i, h := range hashes {
		placeholders[i] = "?"
		args[i] = h
	}
	query := fmt.Sprintf(
		"SELECT raw_hash, category FROM note_category_cache WHERE raw_hash IN (%s)",
		strings.Join(placeholders, ","),
	)
	rows, err := d.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query note_category_cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var hash, catStr string
		if err := rows.Scan(&hash, &catStr); err != nil {
			return nil, fmt.Errorf("telemetry: scan note_category_cache row: %w", err)
		}
		cat := ActivityCategory(catStr)
		if cat.IsValid() {
			out[hash] = cat
		} else {
			out[hash] = CategoryOther
		}
	}
	return out, rows.Err()
}

// PutCachedCategories writes entries (raw_hash -> category) into
// note_category_cache in a single transaction.
func (d *DB) PutCachedCategories(ctx context.Context, entries map[string]ActivityCategory, now time.Time) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("telemetry: begin note_category_cache write: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO note_category_cache (raw_hash, category, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(raw_hash) DO UPDATE SET
			category = excluded.category,
			created_at = excluded.created_at`)
	if err != nil {
		return fmt.Errorf("telemetry: prepare note_category_cache upsert: %w", err)
	}
	defer stmt.Close()

	createdAt := now.Format(time.RFC3339Nano)
	for hash, cat := range entries {
		if _, err := stmt.ExecContext(ctx, hash, string(cat), createdAt); err != nil {
			return fmt.Errorf("telemetry: upsert note_category_cache: %w", err)
		}
	}
	return tx.Commit()
}

// NoteBatchClassifier is the interface for Tier 3 batch note classification.
// It receives a list of unique uncached notes and returns their classified ActivityCategory.
type NoteBatchClassifier interface {
	ClassifyBatch(ctx context.Context, notes []string) ([]ActivityCategory, error)
}

// ClassifyNotes resolves activity categories for a set of tool calls using
// the multi-tier classification pipeline:
//  1. Tier 1: Fast deterministic rule matching (always tried first).
//  2. Tier 2: Persistent content-hash cache lookup for any notes not matched by Tier 1.
//  3. Tier 3: If classifier is non-nil, batched classification on cache misses,
//     persisting results to note_category_cache.
//     If classifier is nil or returns an error, misses safely fallback to CategoryOther.
func ClassifyNotes(ctx context.Context, db *DB, calls []ToolCall, classifier NoteBatchClassifier, now time.Time) ([]ActivityCategory, error) {
	out := make([]ActivityCategory, len(calls))

	type pendingMiss struct {
		index int
		note  string
		hash  string
	}
	var misses []pendingMiss

	// Step 1: Tier 1 fast match
	for i, c := range calls {
		cat := ClassifyTier1(c.ToolName, c.Note, c.ExitCode)
		if cat != CategoryOther {
			out[i] = cat
		} else if strings.TrimSpace(c.Note) != "" {
			h := sha256Hex(c.Note)
			misses = append(misses, pendingMiss{index: i, note: c.Note, hash: h})
		} else {
			out[i] = CategoryOther
		}
	}

	if len(misses) == 0 {
		return out, nil
	}

	// Step 2: Tier 2 Content-hash cache lookup (if DB provided)
	cached := make(map[string]ActivityCategory)
	if db != nil {
		distinctHashes := make([]string, 0, len(misses))
		seenH := make(map[string]bool, len(misses))
		for _, m := range misses {
			if !seenH[m.hash] {
				seenH[m.hash] = true
				distinctHashes = append(distinctHashes, m.hash)
			}
		}
		var err error
		cached, err = db.GetCachedCategories(ctx, distinctHashes)
		if err != nil {
			return nil, err
		}
	}

	var tier3Misses []pendingMiss
	for _, m := range misses {
		if cat, hit := cached[m.hash]; hit && cat.IsValid() {
			out[m.index] = cat
		} else {
			tier3Misses = append(tier3Misses, m)
		}
	}

	if len(tier3Misses) == 0 {
		return out, nil
	}

	// Step 3: Tier 3 Batch Classifier (opt-in)
	if classifier == nil {
		for _, m := range tier3Misses {
			out[m.index] = CategoryOther
		}
		return out, nil
	}

	// Dedupe notes for Tier 3 batching
	seenNote := make(map[string]bool)
	var distinctNotes []string
	var distinctHashes []string
	for _, m := range tier3Misses {
		if !seenNote[m.note] {
			seenNote[m.note] = true
			distinctNotes = append(distinctNotes, m.note)
			distinctHashes = append(distinctHashes, m.hash)
		}
	}

	const batchSize = 50
	classifiedMap := make(map[string]ActivityCategory, len(distinctNotes))
	toCache := make(map[string]ActivityCategory, len(distinctNotes))

	for start := 0; start < len(distinctNotes); start += batchSize {
		end := start + batchSize
		if end > len(distinctNotes) {
			end = len(distinctNotes)
		}
		chunk := distinctNotes[start:end]
		chunkHashes := distinctHashes[start:end]

		cats, err := classifier.ClassifyBatch(ctx, chunk)
		if err != nil {
			// Graceful fallback to CategoryOther on classifier failure
			for _, n := range chunk {
				classifiedMap[n] = CategoryOther
			}
			continue
		}
		for j, cat := range cats {
			if !cat.IsValid() {
				cat = CategoryOther
			}
			classifiedMap[chunk[j]] = cat
			toCache[chunkHashes[j]] = cat
		}
	}

	if db != nil && len(toCache) > 0 {
		_ = db.PutCachedCategories(ctx, toCache, now)
	}

	for _, m := range tier3Misses {
		if cat, ok := classifiedMap[m.note]; ok {
			out[m.index] = cat
		} else {
			out[m.index] = CategoryOther
		}
	}

	return out, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// DefaultLocalClassifier implements NoteBatchClassifier by calling a local
// OpenAI-compatible chat-completions endpoint (e.g. `lmcoder serve`, which
// exposes llama-server's standard /v1/chat/completions route). Note text is
// only ever sent to BaseURL, which defaults to localhost — never to a cloud
// API. If the local server is not running, ClassifyBatch returns an error and
// the caller (ClassifyNotes) falls back to CategoryOther for the batch.
type DefaultLocalClassifier struct {
	// BaseURL is the OpenAI-compatible API root, e.g. "http://localhost:8734/v1".
	// Defaults to defaultClassifierBaseURL when empty.
	BaseURL string
	// Model is the model name to request from the local server. Defaults to
	// defaultClassifierModel when empty.
	Model   string
	Timeout time.Duration
}

const (
	defaultClassifierBaseURL = "http://localhost:8734/v1"
	defaultClassifierModel   = "qwen2.5-3b-instruct-q4"
)

type chatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
}

const classifyPromptTemplate = `You are a strict text classification model categorizing developer tool-call notes into one of exactly 9 canonical activity categories:
- test (go test, pytest, jest, assertion checks)
- build (go build, cargo, gcc, syntax/compiler runs)
- edit (file edits, patches, refactors)
- inspection (read, grep, find, view_file)
- git (status, diff, commit, branch, log)
- debug (reproducing crashes, inspecting panics, traces)
- workflow (agent handoff, issue tracking, protocol injection)
- config (settings, hooks, dotfiles, environment setup)
- other (unclassified fallback)

For each of the %[1]d note(s) in the JSON array below, determine the most fitting category.
Respond with ONLY a JSON array of exactly %[1]d string(s) (one of the 9 lowercase category names above per note), in the same order as the input notes.
Do not include markdown code fences, explanations, or any text other than the JSON array itself.

Input notes:
%[2]s`

func (c *DefaultLocalClassifier) ClassifyBatch(ctx context.Context, notes []string) ([]ActivityCategory, error) {
	if len(notes) == 0 {
		return nil, nil
	}
	baseURL := strings.TrimSuffix(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultClassifierBaseURL
	}
	model := c.Model
	if model == "" {
		model = defaultClassifierModel
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	notesJSON, err := json.Marshal(notes)
	if err != nil {
		return nil, fmt.Errorf("telemetry: marshal notes for classify prompt: %w", err)
	}
	prompt := fmt.Sprintf(classifyPromptTemplate, len(notes), string(notesJSON))

	reqBody := chatCompletionRequest{
		Model: model,
		Messages: []chatCompletionMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.0,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("telemetry: marshal chat completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(cctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("telemetry: build classifier request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("telemetry: local classifier endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("telemetry: read classifier response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telemetry: classifier endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var envelope chatCompletionResponse
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("telemetry: parse classifier json response: %w", err)
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("telemetry: classifier response contained no choices")
	}

	raw := strings.TrimSpace(envelope.Choices[0].Message.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var catStrs []string
	if err := json.Unmarshal([]byte(raw), &catStrs); err != nil {
		return nil, fmt.Errorf("telemetry: parse categories JSON array: %w", err)
	}
	if len(catStrs) != len(notes) {
		return nil, fmt.Errorf("telemetry: classifier returned %d categories, want %d", len(catStrs), len(notes))
	}

	out := make([]ActivityCategory, len(catStrs))
	for i, s := range catStrs {
		cat := ActivityCategory(strings.ToLower(strings.TrimSpace(s)))
		if !cat.IsValid() {
			cat = CategoryOther
		}
		out[i] = cat
	}
	return out, nil
}
