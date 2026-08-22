package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// historyDirName is where per-host usage snapshots are appended. Copying
// another machine's file into this directory folds that machine's history
// into the local timeline — the mechanism is "cp", not sync.
const historyDirName = "usage-history"

// HistoryEntry is one persisted usage snapshot, tagged with the hostname it
// was collected on so entries copied in from other machines stay
// distinguishable once merged into one directory.
type HistoryEntry struct {
	Hostname string `json:"hostname"`
	UsageSummary
}

// HistoryDir resolves the directory that per-host history files live in:
// ~/.claude/harnez/usage-history/. Copy another machine's *.jsonl file(s)
// into this directory to fold that machine's history into the local timeline.
func HistoryDir(homeDir string) string {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return filepath.Join(homeDir, ".claude", "harnez", historyDirName)
}

// sanitizeHostname keeps the on-disk filename portable across the systems a
// history file might get copied between (no "/", spaces, or other characters
// a foreign filesystem might reject).
func sanitizeHostname(h string) string {
	h = strings.ToLower(h)
	var sb strings.Builder
	for _, r := range h {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			sb.WriteRune(r)
		default:
			sb.WriteRune('-')
		}
	}
	if sb.Len() == 0 {
		return "unknown-host"
	}
	return sb.String()
}

func historyFilePath(dir string) string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	return filepath.Join(dir, sanitizeHostname(host)+".jsonl")
}

const historyLockRetries = 5
const historyLockDelay = 50 * time.Millisecond

// lockHistoryFile takes a non-blocking, bounded-retry exclusive flock on a
// sidecar `.lock` file, the same pattern used for the quota cache (issue
// 033), so two local `harnez usage` processes (e.g. a --watch loop and a
// one-shot --history call) never interleave appends to the same file.
func lockHistoryFile(path string) (f *os.File, ok bool) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false
	}
	for attempt := 0; attempt <= historyLockRetries; attempt++ {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return f, true
		}
		if attempt < historyLockRetries {
			time.Sleep(historyLockDelay)
		}
	}
	f.Close()
	return nil, false
}

func unlockHistoryFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// AppendHistory appends one snapshot as a JSON line to this machine's history
// file under dir (see HistoryDir), tagging it with the local hostname.
func AppendHistory(dir string, summary UsageSummary) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown-host"
	}
	line, err := json.Marshal(HistoryEntry{Hostname: host, UsageSummary: summary})
	if err != nil {
		return fmt.Errorf("marshal history entry: %w", err)
	}

	path := historyFilePath(dir)
	lockFile, ok := lockHistoryFile(path)
	if !ok {
		return fmt.Errorf("could not lock history file %s", path)
	}
	defer unlockHistoryFile(lockFile)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write history entry: %w", err)
	}
	return nil
}

// ReadHistory loads and merges every *.jsonl file directly under dir
// (typically one per machine — see HistoryDir), sorted by timestamp.
// Malformed lines are skipped rather than failing the whole read, since a
// history file may have been copied in mid-write from another machine.
func ReadHistory(dir string) ([]HistoryEntry, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob history dir: %w", err)
	}

	var entries []HistoryEntry
	for _, path := range matches {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var e HistoryEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				continue
			}
			entries = append(entries, e)
		}
		f.Close()
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp.Before(entries[j].Timestamp) })
	return entries, nil
}

// RenderTimelineText formats merged history entries as a flat, chronological
// table — one row per (snapshot, agent) pair — so a timeline spanning several
// machines reads as a single ordered log.
func RenderTimelineText(entries []HistoryEntry) string {
	if len(entries) == 0 {
		return "No usage history recorded yet. Run `harnez usage --history` (or `--watch --history`) to start recording.\n"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-20s %-16s %-8s %9s %9s %14s\n", "TIME", "HOST", "AGENT", "SESSION%", "WEEKLY%", "TOKENS"))
	for _, e := range entries {
		ts := e.Timestamp.Local().Format("2006-01-02 15:04:05")
		for _, a := range e.Agents {
			if !a.Installed || !a.Authenticated {
				continue
			}
			sess, week, tok := "-", "-", "-"
			if a.Session != nil {
				sess = fmt.Sprintf("%.1f%%", a.Session.UsedPercent)
			}
			if a.Weekly != nil {
				week = fmt.Sprintf("%.1f%%", a.Weekly.UsedPercent)
			}
			if a.Tokens != nil {
				tok = FormatNumber(a.Tokens.TotalTokens)
			}
			sb.WriteString(fmt.Sprintf("%-20s %-16s %-8s %9s %9s %14s\n", ts, e.Hostname, a.AgentID, sess, week, tok))
		}
	}
	return sb.String()
}

// RenderTimelineJSON serializes merged history entries for scripting/plotting.
func RenderTimelineJSON(entries []HistoryEntry) (string, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal history entries: %w", err)
	}
	return string(data), nil
}
