package tools

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HistoryEntry is one recorded transcription. History is local-only and
// sensitive (it may contain anything the user has ever dictated); it is
// never synced, and ClearHistory exists specifically so it can be wiped.
type HistoryEntry struct {
	ID   string    `json:"id"`
	Time time.Time `json:"time"`
	Text string    `json:"text"`
}

// DefaultHistoryLimit caps how many entries the history file keeps. This is
// a convenience buffer for "what did I just dictate", not a searchable
// archive.
const DefaultHistoryLimit = 20

// HistoryPath returns the local history file path under the user's XDG data
// dir. Callers create the directory (0700) and file (0600) themselves via
// AppendHistory; nothing here is more persistent or widely readable than a
// single user-owned file needs to be.
func HistoryPath(home string) string {
	return filepath.Join(home, ".local", "share", "harnez", "voice-input", "history.jsonl")
}

func historyID(text string, t time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s", t.UnixNano(), text)))
	return hex.EncodeToString(sum[:])[:8]
}

// AppendHistory appends one entry and trims the file to the most recent
// `limit` entries (limit <= 0 means unlimited). Empty text is a no-op (e.g.
// a cancelled dictation) and returns a zero-ID entry.
func AppendHistory(path, text string, limit int, now time.Time) (HistoryEntry, error) {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return HistoryEntry{}, nil
	}
	entry := HistoryEntry{ID: historyID(text, now), Time: now, Text: text}
	entries, err := readHistory(path)
	if err != nil {
		return entry, err
	}
	entries = append(entries, entry)
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	if err := writeHistory(path, entries); err != nil {
		return entry, err
	}
	return entry, nil
}

// ListHistory returns recorded entries most-recent-first.
func ListHistory(path string) ([]HistoryEntry, error) {
	entries, err := readHistory(path)
	if err != nil {
		return nil, err
	}
	reversed := make([]HistoryEntry, len(entries))
	for i, e := range entries {
		reversed[len(entries)-1-i] = e
	}
	return reversed, nil
}

// FindHistoryEntry resolves a short entry ID (as printed by ListHistory) to
// its full entry.
func FindHistoryEntry(path, id string) (HistoryEntry, error) {
	entries, err := readHistory(path)
	if err != nil {
		return HistoryEntry{}, err
	}
	for _, e := range entries {
		if e.ID == id {
			return e, nil
		}
	}
	return HistoryEntry{}, fmt.Errorf("no history entry with id %q", id)
}

// ClearHistory removes all recorded entries.
func ClearHistory(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear history: %w", err)
	}
	return nil
}

func readHistory(path string) ([]HistoryEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read history: %w", err)
	}
	var entries []HistoryEntry
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip a corrupt line rather than fail the whole read
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func writeHistory(path string, entries []HistoryEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}
	var buf strings.Builder
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("encode history entry: %w", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".history-*")
	if err != nil {
		return fmt.Errorf("create temp history file: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	writeErr := func() error {
		if _, err := tmp.WriteString(buf.String()); err != nil {
			return err
		}
		return tmp.Chmod(0600)
	}()
	closeErr := tmp.Close()
	if writeErr != nil {
		return fmt.Errorf("write history: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write history: %w", closeErr)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("install history: %w", err)
	}
	return nil
}
