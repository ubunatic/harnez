package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/hookdispatch"
)

// The PostCompact hook reads the transcript once and never waits. Claude writes the
// compact boundary after PostCompact returns, so the collector fills the row later
// from a re-read (issue 1303 M1e).

// singleClaudeCompactRead is one read with no freshness check. The collector
// decides whether the boundary follows the PreCompact row. The read is bounded by
// claude_compact.max_transcript_bytes (issue 1303 M3).
func singleClaudeCompactRead(path, sessionID string) (*claudeCompact, string) {
	c, err := scanClaudeCompactSpec(path, sessionID)
	switch {
	case err != nil:
		return nil, reasonReadError
	case c == nil:
		return nil, reasonNoBoundary
	default:
		return c, ""
	}
}

// scanClaudeCompactSpec is scanClaudeCompact with the bound from the hooks spec.
func scanClaudeCompactSpec(path, sessionID string) (*claudeCompact, error) {
	maxBytes, err := hookdispatch.LoadClaudeCompactMaxTranscriptBytes()
	if err != nil {
		return nil, err
	}
	return scanClaudeCompact(path, sessionID, maxBytes)
}

// claudeCompact is the newest complete compact boundary in a Claude transcript
// (issue 1303 M1). Model is the last main-chain model seen before the boundary.
// Only token counts, trigger and model are kept: the compact summary and custom
// instructions are never read.
type claudeCompact struct {
	Timestamp  time.Time `json:"timestamp"`
	Trigger    string    `json:"trigger"`
	PreTokens  int64     `json:"pre_tokens"`
	PostTokens int64     `json:"post_tokens"`
	Model      string    `json:"model"`
}

// claudeTranscriptLine is the subset of one transcript line the compact reader needs.
type claudeTranscriptLine struct {
	Type        string `json:"type"`
	Subtype     string `json:"subtype"`
	Timestamp   string `json:"timestamp"`
	SessionID   string `json:"sessionId"`
	IsSidechain bool   `json:"isSidechain"`
	Message     struct {
		Model string `json:"model"`
	} `json:"message"`
	CompactMetadata *struct {
		Trigger    string `json:"trigger"`
		PreTokens  *int64 `json:"preTokens"`
		PostTokens *int64 `json:"postTokens"`
	} `json:"compactMetadata"`
}

// readClaudeCompact returns sessionID's newest compact boundary within the last
// maxBytes of the transcript, or nil when the file is missing, unreadable or has
// none. It never fails the hook: a read error is the same as no boundary.
func readClaudeCompact(path, sessionID string, maxBytes int64) *claudeCompact {
	c, _ := scanClaudeCompact(path, sessionID, maxBytes)
	return c
}

// scanClaudeCompact returns sessionID's newest compact boundary in the last maxBytes of
// the transcript at path, or nil when there is none (issue 1303 M1c, M3). A sidechain
// boundary is never a compaction of the session. A boundary that names another
// session is skipped too; a boundary with no sessionId is kept, since the field's
// presence on boundary lines is not verified on this host. A file larger than maxBytes
// is read from its tail only, and the tail's first, possibly cut, line is dropped.
// maxBytes must be positive: there is no unbounded read.
func scanClaudeCompact(path, sessionID string, maxBytes int64) (*claudeCompact, error) {
	if maxBytes <= 0 {
		return nil, errors.New("claude compact: read bound must be positive")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	cut := info.Size() > maxBytes
	if cut {
		// Start one byte early. If the byte before the tail is a newline, the tail starts
		// a whole line, and the skip below consumes only that newline.
		if _, err := file.Seek(info.Size()-maxBytes-1, io.SeekStart); err != nil {
			return nil, err
		}
	}
	reader := bufio.NewReader(file)
	if cut {
		if _, err := reader.ReadBytes('\n'); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	var latest *claudeCompact
	var mainModel string
	for {
		line, readErr := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			var entry claudeTranscriptLine
			if json.Unmarshal(trimmed, &entry) == nil {
				if model := entry.Message.Model; model != "" && !entry.IsSidechain && !strings.HasPrefix(model, "<") {
					mainModel = model
				}
				ownBoundary := !entry.IsSidechain && (entry.SessionID == "" || entry.SessionID == sessionID)
				if ownBoundary && entry.Type == "system" && entry.Subtype == "compact_boundary" && entry.CompactMetadata != nil && entry.CompactMetadata.PreTokens != nil && entry.CompactMetadata.PostTokens != nil {
					ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
					latest = &claudeCompact{Timestamp: ts, Trigger: entry.CompactMetadata.Trigger, PreTokens: *entry.CompactMetadata.PreTokens, PostTokens: *entry.CompactMetadata.PostTokens, Model: mainModel}
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return latest, nil
			}
			return nil, readErr
		}
	}
}
