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

// claudeCompactNow and claudeCompactSleep are the wait clock. Tests replace them
// so the boundary poll runs without real sleeps (issue 1303 M1b).
var (
	claudeCompactNow   = time.Now
	claudeCompactSleep = time.Sleep
)

// waitClaudeCompact polls the transcript for a complete compact boundary at most
// boundary_window old. Claude may append its boundary after the PostCompact hook
// has started, so one read can miss it (issue 1303 M1b). The poll stops after
// boundary_wait, which keeps the hook under handler_timeout. A boundary older
// than the window belongs to an earlier compaction and is skipped.
func waitClaudeCompact(path string, wait hookdispatch.ClaudeCompactWait) *claudeCompact {
	cutoff := claudeCompactNow().Add(-wait.Window)
	polls := int(wait.Wait / wait.Poll)
	for slept := 0; ; slept++ {
		if c := readClaudeCompact(path); c != nil && !c.Timestamp.Before(cutoff) {
			return c
		}
		if slept >= polls {
			return nil
		}
		claudeCompactSleep(wait.Poll)
	}
}

// readClaudePostCompact is the PostCompact read. Only Claude's payload (it carries
// compact_summary) waits for the boundary; Codex reads once. An unreadable spec
// falls back to one read, which the spec tests guard against at build time.
func readClaudePostCompact(path string, claude bool) *claudeCompact {
	if !claude {
		return readClaudeCompact(path)
	}
	wait, err := hookdispatch.LoadClaudeCompactWait()
	if err != nil {
		return readClaudeCompact(path)
	}
	return waitClaudeCompact(path, wait)
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

// readClaudeCompact returns the newest compact boundary that carries both token
// counts, or nil when the file is missing, unreadable or has none. It never
// fails the hook: a read error is the same as no boundary.
func readClaudeCompact(path string) *claudeCompact {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	reader := bufio.NewReader(file)
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
				if entry.Type == "system" && entry.Subtype == "compact_boundary" && entry.CompactMetadata != nil && entry.CompactMetadata.PreTokens != nil && entry.CompactMetadata.PostTokens != nil {
					ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
					latest = &claudeCompact{Timestamp: ts, Trigger: entry.CompactMetadata.Trigger, PreTokens: *entry.CompactMetadata.PreTokens, PostTokens: *entry.CompactMetadata.PostTokens, Model: mainModel}
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return latest
			}
			return nil
		}
	}
}
