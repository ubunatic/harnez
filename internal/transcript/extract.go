package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/usage"
)

// Stats describes the evidence emitted, including a partial final line skipped
// because a running session had not finished writing it.
type Stats struct {
	Summaries     int    `json:"summaries"`
	UserMessages  int    `json:"user_messages"`
	TextBytes     int    `json:"text_bytes"`
	FirstRecord   string `json:"first_record,omitempty"`
	LastRecord    string `json:"last_record,omitempty"`
	TruncatedTail bool   `json:"truncated_tail"`
}

// Resolve accepts a path or unique session ID prefix. Discovery uses the same
// top-level Claude project sessions as usage attribution, never subagent files.
func Resolve(claudeDir, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", errors.New("session ID or path is required")
	}
	if info, err := os.Stat(id); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("session path is not a regular file: %s", id)
		}
		return filepath.Abs(id)
	}
	if strings.ContainsAny(id, `/\`) || strings.HasSuffix(id, ".jsonl") {
		return "", fmt.Errorf("session file not found: %s", id)
	}
	files, err := usage.ClaudeTranscriptFiles(claudeDir)
	if err != nil {
		return "", fmt.Errorf("discover Claude sessions: %w", err)
	}
	var matches []string
	for _, file := range files {
		if strings.HasPrefix(strings.TrimSuffix(filepath.Base(file), ".jsonl"), id) {
			matches = append(matches, file)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("Claude session %q not found", id)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("ambiguous Claude session %q: %d matches; use a full ID or path", id, len(matches))
	}
	return matches[0], nil
}

// Extract streams lines through a provider adapter and emits only selected
// records. Memory is bounded by the largest line, with no scanner token limit.
func Extract(r io.Reader, adapter Adapter, emit func(Record) error) (Stats, error) {
	var stats Stats
	reader := bufio.NewReader(r)
	for number := 1; ; number++ {
		line, readErr := reader.ReadBytes('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return stats, fmt.Errorf("read line %d: %w", number, readErr)
		}
		if len(bytes.TrimSpace(line)) > 0 {
			record, err := adapter.Decode(line)
			if err != nil {
				var syntax *json.SyntaxError
				if errors.Is(readErr, io.EOF) && errors.As(err, &syntax) && syntax.Error() == "unexpected end of JSON input" {
					stats.TruncatedTail = true
					return stats, nil
				}
				return stats, fmt.Errorf("decode line %d: %w", number, err)
			}
			if record != nil {
				if err := emit(*record); err != nil {
					return stats, err
				}
				if record.Kind == "summary" {
					stats.Summaries++
				} else {
					stats.UserMessages++
				}
				stats.TextBytes += len(record.Text)
				if stats.FirstRecord == "" {
					stats.FirstRecord = record.Timestamp
				}
				stats.LastRecord = record.Timestamp
			}
		}
		if errors.Is(readErr, io.EOF) {
			return stats, nil
		}
	}
}

// WriteFile streams an extract as text or a JSON object containing source,
// records and stats. A failed extraction may leave partial output; callers must
// check the returned error (the CLI's exit status) before consuming it.
func WriteFile(w io.Writer, path string, asJSON bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	if asJSON {
		source, err := json.Marshal(struct {
			Path  string `json:"path"`
			Bytes int64  `json:"bytes"`
		}{path, info.Size()})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "{\"source\":%s,\"records\":[", source)
		if err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(w, "Session: %s\nSource bytes: %d\n", path, info.Size()); err != nil {
		return err
	}
	first := true
	stats, err := Extract(f, Claude{}, func(record Record) error {
		if !asJSON {
			_, err := fmt.Fprintf(w, "\n--- %s %s ---\n%s\n", record.Kind, record.Timestamp, record.Text)
			return err
		}
		if !first {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		first = false
		return encoder.Encode(record)
	})
	if err != nil {
		return err
	}
	if asJSON {
		if _, err := io.WriteString(w, "],\"stats\":"); err != nil {
			return err
		}
		if err := encoder.Encode(stats); err != nil {
			return err
		}
		_, err = io.WriteString(w, "}\n")
	} else {
		_, err = fmt.Fprintf(w, "\nSummaries: %d\nUser messages: %d\nText bytes: %d\nFirst record: %s\nLast record: %s\nTruncated tail: %t\n", stats.Summaries, stats.UserMessages, stats.TextBytes, stats.FirstRecord, stats.LastRecord, stats.TruncatedTail)
	}
	return err
}
