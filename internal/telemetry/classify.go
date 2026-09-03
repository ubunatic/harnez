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
	"os"
	"regexp"
	"strconv"
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

	// batchSize now bounds one DefaultLocalClassifier conversation's length
	// (see its ClassifyBatch doc comment: one note per turn, not one JSON
	// array per batch), so a bad reply only costs its own note rather than
	// the whole chunk — the old failure mode this constant used to guard
	// against (small models losing track and repeating entries past a large
	// JSON array's requested length) no longer applies. This now just bounds
	// how much conversation history accumulates before starting a fresh
	// session, to stay well clear of the local model's serve context window
	// (16384 tokens for qwen3-4b-instruct-2507-q4 on this box): 60 notes at
	// a generous ~55 tokens/turn is ~3.3k tokens, comfortable headroom.
	const batchSize = 60
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
			// Graceful fallback to CategoryOther on classifier failure (issue
			// 216 AC3) — but still surface *why*, since a silently-failing
			// classifier and a working one otherwise look identical in the
			// export output (see issue 216 follow-up: swallowed errors made a
			// broken Tier 3 pass indistinguishable from --classify not being
			// used at all).
			fmt.Fprintf(os.Stderr, "harnez: warning: batch classification failed for %d note(s), falling back to %q: %v\n", len(chunk), CategoryOther, err)
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

// truncateForError trims s to at most n runes for embedding in error
// messages, so a runaway or malformed model response doesn't blow up log
// output while still giving enough context to diagnose it.
func truncateForError(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "...(truncated)"
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// DefaultLocalClassifier implements NoteBatchClassifier by holding one
// growing conversation per ClassifyBatch call against a local
// OpenAI-compatible chat-completions endpoint (e.g. `lmcoder serve`, which
// exposes llama-server's standard /v1/chat/completions route): a system
// preamble once, then one note per turn ("<n>: <note>" -> "<n>: <category>"),
// each turn appended to the message history sent on the next call. This
// replaced an earlier design that asked for a whole batch's categories as one
// JSON array in a single request — that failed unpredictably as batches grew
// (the small local model would lose track and repeat entries instead of
// closing the array). Per-turn requests are self-contained: a bad reply only
// costs that one note (falls back to CategoryOther), never the whole batch.
// Because each call resends the full growing history and llama-server caches
// matching prompt prefixes (confirmed via its response `cached_tokens`
// field), only the newly appended note/reply needs reprocessing each turn —
// so this isn't N independent cold requests despite looking like one.
//
// Note text is only ever sent to BaseURL, which defaults to localhost — never
// to a cloud API. If the local server is unreachable on the very first turn,
// ClassifyBatch returns an error and the caller (ClassifyNotes) falls back to
// CategoryOther for the whole batch, same as before; a failure after the
// session is already underway instead falls back only the remaining
// unclassified notes in that batch, logging its own warning.
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
	// defaultClassifierModel: qwen3-4b-instruct-2507-q4 (Qwen3, July 2025) —
	// the best already-cached lmcoder model for this box (see
	// ~/projects/lmcoder/spec/models.yaml). Chosen over qwen2.5-3b-instruct-q4
	// (mid-2024, the original default), which is a full generation older,
	// similar size/speed, and has a much smaller native context (32k vs
	// 262k). Larger cached-on-demand options (mistral-nemo-12b,
	// qwen3.8-27b-instruct-q4) trade latency/download size for quality; not
	// used as the default here.
	defaultClassifierModel = "qwen3-4b-instruct-2507-q4"
)

type chatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []chatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature"`
	MaxTokens   int                     `json:"max_tokens"`
}

// turnMaxTokens bounds a single turn's reply to "<n>: <category>" — the
// longest category name is "inspection" (~2-3 tokens), plus the number and
// punctuation, so this leaves comfortable headroom while still failing fast
// if a turn goes off the rails instead of rambling for a long time.
const turnMaxTokens = 16

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatCompletionMessage `json:"message"`
	} `json:"choices"`
}

const classifySystemPrompt = `You are a fast local classification tool for developer tool-call notes. There are exactly 9 canonical activity categories:
- test (go test, pytest, jest, assertion checks)
- build (go build, cargo, gcc, syntax/compiler runs)
- edit (file edits, patches, refactors)
- inspection (read, grep, find, view_file)
- git (status, diff, commit, branch, log)
- debug (reproducing crashes, inspecting panics, traces)
- workflow (agent handoff, issue tracking, protocol injection)
- config (settings, hooks, dotfiles, environment setup)
- other (unclassified, generic, or ambiguous fallback)

I will send you one note per turn, in the form "<number>: <note text>". For
each one, reply with EXACTLY "<number>: <category>" — the same number I sent,
a colon, a space, and exactly one of the 9 lowercase category names above.
Do not include markdown, punctuation beyond that single colon, explanations,
or any other text. Wait for each line before replying; never anticipate or
batch ahead.`

// turnReplyPattern matches a single "<number>: <category>" reply line,
// tolerating minor whitespace/formatting drift from the model.
var turnReplyPattern = regexp.MustCompile(`^\s*(\d+)\s*:\s*([a-zA-Z]+)\s*$`)

// ClassifyBatch runs notes through one growing conversation (see the
// DefaultLocalClassifier doc comment for why): a system preamble once, then
// one request per note, each appending that note and the model's reply to
// the message history sent on the next request.
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
		// Per-turn, not per-batch: a single reply is a couple of tokens, and
		// llama-server caches the matching prompt prefix from the previous
		// turn, so this is generous headroom even under GPU contention, not
		// a budget that needs to scale with session length.
		timeout = 45 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	messages := []chatCompletionMessage{{Role: "system", Content: classifySystemPrompt}}
	out := make([]ActivityCategory, len(notes))

	for i, note := range notes {
		n := i + 1
		messages = append(messages, chatCompletionMessage{Role: "user", Content: fmt.Sprintf("%d: %s", n, note)})

		reqBody := chatCompletionRequest{
			Model:       model,
			Messages:    messages,
			Temperature: 0.0,
			MaxTokens:   turnMaxTokens,
		}
		reqJSON, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("telemetry: marshal chat completion request for note %d: %w", n, err)
		}

		cctx, cancel := context.WithTimeout(ctx, timeout)
		httpReq, err := http.NewRequestWithContext(cctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(reqJSON))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("telemetry: build classifier request for note %d: %w", n, err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			deadlineExceeded := cctx.Err() == context.DeadlineExceeded
			cancel()
			var wrapped error
			if deadlineExceeded {
				wrapped = fmt.Errorf("telemetry: local classifier at %s did not respond to note %d within %s (server reachable but too slow, or overloaded): %w", baseURL, n, timeout, err)
			} else {
				wrapped = fmt.Errorf("telemetry: local classifier endpoint at %s unreachable on note %d (is `lmcoder serve`/`lmcoder start` running?): %w", baseURL, n, err)
			}
			if i == 0 {
				// Nothing succeeded yet — let the caller (ClassifyNotes)
				// apply its own whole-batch fallback and warning.
				return nil, wrapped
			}
			// Session was already underway: keep what we classified so far,
			// fall the rest back to CategoryOther, and warn locally instead
			// of failing the whole batch.
			fmt.Fprintf(os.Stderr, "harnez: warning: classifier session request failed on note %d/%d, falling back to %q for the remaining %d note(s) in this session: %v\n", n, len(notes), CategoryOther, len(notes)-i, wrapped)
			for j := i; j < len(notes); j++ {
				out[j] = CategoryOther
			}
			return out, nil
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("telemetry: read classifier response for note %d: %w", n, err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("telemetry: classifier endpoint returned status %d for note %d: %s", resp.StatusCode, n, strings.TrimSpace(string(respBody)))
		}

		var envelope chatCompletionResponse
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			return nil, fmt.Errorf("telemetry: parse classifier json response for note %d: %w", n, err)
		}
		if len(envelope.Choices) == 0 {
			return nil, fmt.Errorf("telemetry: classifier response for note %d contained no choices", n)
		}

		reply := strings.TrimSpace(envelope.Choices[0].Message.Content)
		messages = append(messages, chatCompletionMessage{Role: "assistant", Content: reply})

		m := turnReplyPattern.FindStringSubmatch(reply)
		if m == nil || m[1] != strconv.Itoa(n) {
			fmt.Fprintf(os.Stderr, "harnez: warning: unexpected classifier reply for note %d, falling back to %q: %s\n", n, CategoryOther, truncateForError(reply, 200))
			out[i] = CategoryOther
			continue
		}
		cat := ActivityCategory(strings.ToLower(m[2]))
		if !cat.IsValid() {
			fmt.Fprintf(os.Stderr, "harnez: warning: classifier returned unknown category %q for note %d, falling back to %q\n", m[2], n, CategoryOther)
			cat = CategoryOther
		}
		out[i] = cat
	}
	return out, nil
}
