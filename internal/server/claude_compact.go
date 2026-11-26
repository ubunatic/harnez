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
)

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
