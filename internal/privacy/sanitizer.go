package privacy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// NoteSanitizer rewrites a batch of raw free-text notes into sanitized,
// high-level summaries, in the same order as the input, one output per
// input. It is a real interface (not a stub) so the LLM backend behind
// LevelAgentSanitized is swappable later (e.g. a direct api.anthropic.com
// call, or a different local model) without touching the caching/batching
// orchestration in internal/telemetry/sanitize_cache.go, which depends
// only on this interface.
type NoteSanitizer interface {
	SanitizeBatch(ctx context.Context, notes []string) ([]string, error)
}

// DefaultSanitizeBatchSize bounds how many notes go into one NoteSanitizer
// call. Issue 204 explicitly requires minimizing LLM invocations: batch
// aggressively rather than one call per note. 50 is chosen as a
// conservative middle ground — small enough that the prompt and the
// model's JSON-array response both stay well within normal context/output
// limits and a single malformed response only costs one batch's worth of
// notes to retry, but large enough that a typical export run (which sees,
// at most, a few dozen new/changed notes since the last run thanks to the
// content-hash cache) finishes in one call. Chunking at this size rather
// than sending everything in a single call also caps how much a single
// slow/huge prompt can affect an export's runtime.
const DefaultSanitizeBatchSize = 50

// sanitizePromptTemplate is the batch prompt sent to the `claude` CLI.
// Notes are embedded as a JSON array built via encoding/json.Marshal (see
// ClaudeCLISanitizer.SanitizeBatch) rather than string-concatenated, so
// note content can never break out of its slot in the prompt regardless
// of what characters/quotes/newlines it contains.
const sanitizePromptTemplate = `You are sanitizing short free-text developer tool-call notes before they are published in a public data-visualization export.

For each of the %[1]d note(s) in the JSON array below, rewrite it as a short, generic, high-level summary of the kind of action taken. Remove and never repeat: client/company/customer names, project code names, secrets/API keys/tokens, absolute file paths, usernames, email addresses, hostnames, or any other identifying or confidential detail. When in doubt, generalize further rather than less (example: "Fixed internal auth bug in client Acme Corp repo" becomes "Fixed an authentication bug").

Respond with ONLY a JSON array of exactly %[1]d string(s), one sanitized summary per input note, in the same order as the input notes. Do not include markdown code fences, explanations, or any text other than the JSON array itself.

Input notes (JSON array of %[1]d string(s)):
%[2]s`

// ClaudeCLISanitizer implements NoteSanitizer by shelling out to the
// `claude` CLI non-interactively (issue 204's explicit choice: reuse
// whatever auth the user's Claude Code install already has, rather than
// having harnez hold its own api.anthropic.com credential).
//
// Invocation shape (`claude -p --output-format json`, prompt on stdin) was
// determined by running `claude --help` and a live smoke probe
// (`echo '...' | claude -p --output-format json`) rather than guessed:
//   - `-p`/`--print` runs one non-interactive turn and exits.
//   - Omitting the positional `prompt` argument and instead writing the
//     prompt to stdin avoids ever putting note text on the process argv
//     (long argv, and a note that happens to start with "-" being parsed
//     as a flag, are both avoided) — this is also why exec.CommandContext
//     is used with a fixed argument slice, never a shell string.
//   - `--output-format json` wraps the reply in a JSON envelope whose
//     `result` field holds the model's text output and whose `is_error`
//     field flags a failed turn; the prompt instructs the model to make
//     that text itself be a JSON array, which is parsed out of `result`.
type ClaudeCLISanitizer struct {
	// Bin is the executable to run. Defaults to "claude" (resolved via
	// PATH) when empty.
	Bin string
	// Timeout bounds one SanitizeBatch call. Defaults to 90s when zero.
	Timeout time.Duration
}

// NewClaudeCLISanitizer returns a ClaudeCLISanitizer with default
// Bin/Timeout.
func NewClaudeCLISanitizer() *ClaudeCLISanitizer {
	return &ClaudeCLISanitizer{}
}

// claudeResultEnvelope is the subset of `claude -p --output-format json`'s
// output shape this package reads. The real envelope carries many more
// fields (token usage, cost, session id, ...) that are irrelevant here.
type claudeResultEnvelope struct {
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
}

// SanitizeBatch sends all of notes as a single `claude -p` call. Callers
// (see internal/telemetry.SanitizeNotes) are responsible for chunking a
// larger set of notes into DefaultSanitizeBatchSize-sized pieces before
// calling this — SanitizeBatch itself does not sub-chunk.
func (c *ClaudeCLISanitizer) SanitizeBatch(ctx context.Context, notes []string) ([]string, error) {
	if len(notes) == 0 {
		return nil, nil
	}

	bin := c.Bin
	if bin == "" {
		bin = "claude"
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	notesJSON, err := json.Marshal(notes)
	if err != nil {
		return nil, fmt.Errorf("privacy: marshal notes for sanitize prompt: %w", err)
	}
	prompt := fmt.Sprintf(sanitizePromptTemplate, len(notes), string(notesJSON))

	// exec.CommandContext with a fixed argument slice — never a shell
	// string — and the prompt delivered over stdin rather than argv, per
	// issue 204's security note on shelling out with agent-generated
	// free text as input.
	cmd := exec.CommandContext(cctx, bin, "-p", "--output-format", "json")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("privacy: claude CLI invocation failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	var envelope claudeResultEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		return nil, fmt.Errorf("privacy: parse claude CLI json envelope: %w (stdout: %s)", err, truncate(stdout.String(), 500))
	}
	if envelope.IsError {
		return nil, fmt.Errorf("privacy: claude CLI reported an error result: %s", truncate(envelope.Result, 500))
	}

	sanitized, err := parseSanitizedArray(envelope.Result)
	if err != nil {
		return nil, err
	}
	if len(sanitized) != len(notes) {
		return nil, fmt.Errorf("privacy: claude returned %d sanitized note(s), expected %d", len(sanitized), len(notes))
	}
	return sanitized, nil
}

// parseSanitizedArray extracts the JSON string array the sanitize prompt
// asked for out of the model's raw text reply, tolerating the model
// wrapping it in a markdown ```json fence despite being told not to
// (models do this often enough in practice that stripping it defensively
// is cheaper than a guaranteed-to-eventually-fire retry loop).
func parseSanitizedArray(raw string) ([]string, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("privacy: parse sanitized notes JSON array from claude output: %w (raw: %s)", err, truncate(raw, 500))
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
