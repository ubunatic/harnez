package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// QuotaHistoryFilename is the append-only JSONL log where quota window states
// are recorded upon cache updates (issue 375).
const QuotaHistoryFilename = "quota-history.jsonl"

// DefaultQuotaHistoryThrottle is the default minimum interval before appending
// an unchanged quota window reading to quota-history.jsonl.
const DefaultQuotaHistoryThrottle = 15 * time.Minute

// QuotaHistoryEntry represents one persisted quota window snapshot.
type QuotaHistoryEntry struct {
	Timestamp        time.Time  `json:"timestamp"`
	Agent            string     `json:"agent"`
	Group            string     `json:"group,omitempty"`
	Window           string     `json:"window"`
	UsedPercent      int        `json:"used_percent"`
	RemainingPercent int        `json:"remaining_percent"`
	ResetAt          *time.Time `json:"reset_at,omitempty"`
	DurationLeftMS   int64      `json:"duration_left_ms,omitempty"`
}

// QuotaHistoryPath returns the absolute path to quota-history.jsonl inside historyDir.
func QuotaHistoryPath(historyDir string) string {
	if historyDir == "" {
		historyDir = HistoryDir("")
	}
	return filepath.Join(historyDir, QuotaHistoryFilename)
}

// resolveQuotaHistoryDir determines the usage-history directory given an agent's
// config/state directory (e.g. ~/.claude, ~/.codex, ~/.gemini/antigravity-cli).
func resolveQuotaHistoryDir(agentDir string) string {
	if agentDir == "" {
		return HistoryDir("")
	}
	base := filepath.Base(agentDir)
	if base == ".claude" {
		return filepath.Join(agentDir, "harnez", historyDirName)
	}
	if base == ".codex" {
		homeDir := filepath.Dir(agentDir)
		return HistoryDir(homeDir)
	}
	if base == "antigravity-cli" && filepath.Base(filepath.Dir(agentDir)) == ".gemini" {
		homeDir := filepath.Dir(filepath.Dir(agentDir))
		return HistoryDir(homeDir)
	}
	if _, err := os.Stat(filepath.Join(agentDir, ".claude")); err == nil {
		return filepath.Join(agentDir, ".claude", "harnez", historyDirName)
	}
	return filepath.Join(agentDir, "harnez", historyDirName)
}

// quotaWindowToHistoryEntry converts a QuotaWindow into a QuotaHistoryEntry.
func quotaWindowToHistoryEntry(agent, group string, qw QuotaWindow, now time.Time) QuotaHistoryEntry {
	windowName := qw.Name
	if windowName == "" {
		windowName = "quota"
	}
	used := int(math.Round(qw.UsedPercent))
	remaining := int(math.Round(qw.RemainingPercent))
	if remaining == 0 && qw.RemainingPercent == 0 && qw.UsedPercent > 0 {
		remaining = 100 - used
		if remaining < 0 {
			remaining = 0
		}
	}
	if used == 0 && qw.UsedPercent == 0 && qw.RemainingPercent > 0 {
		used = 100 - remaining
		if used < 0 {
			used = 0
		}
	}

	var durLeftMS int64
	if qw.ResetAt != nil {
		if qw.ResetAt.After(now) {
			durLeftMS = qw.ResetAt.Sub(now).Milliseconds()
		}
	} else if qw.DurationLeft > 0 {
		durLeftMS = qw.DurationLeft.Milliseconds()
	}

	return QuotaHistoryEntry{
		Timestamp:        now.UTC().Truncate(time.Second),
		Agent:            agent,
		Group:            group,
		Window:           windowName,
		UsedPercent:      used,
		RemainingPercent: remaining,
		ResetAt:          qw.ResetAt,
		DurationLeftMS:   durLeftMS,
	}
}

// AgentUsageToQuotaHistoryEntries extracts all active quota windows from an
// AgentUsage and converts them into QuotaHistoryEntry items.
func AgentUsageToQuotaHistoryEntries(u AgentUsage, now time.Time) []QuotaHistoryEntry {
	var entries []QuotaHistoryEntry
	if u.Session != nil {
		qw := *u.Session
		if qw.Name == "" {
			qw.Name = "Session"
		}
		entries = append(entries, quotaWindowToHistoryEntry(u.AgentID, "", qw, now))
	}
	if u.Weekly != nil {
		qw := *u.Weekly
		if qw.Name == "" {
			qw.Name = "Weekly"
		}
		entries = append(entries, quotaWindowToHistoryEntry(u.AgentID, "", qw, now))
	}
	for _, g := range u.ModelGroups {
		for _, w := range g.Windows {
			entries = append(entries, quotaWindowToHistoryEntry(u.AgentID, g.Name, w, now))
		}
	}
	for _, w := range u.ExtraWindows {
		entries = append(entries, quotaWindowToHistoryEntry(u.AgentID, "", w, now))
	}
	return entries
}

// ReadQuotaHistory loads all recorded entries from quota-history.jsonl in historyDir.
func ReadQuotaHistory(historyDir string) ([]QuotaHistoryEntry, error) {
	path := QuotaHistoryPath(historyDir)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []QuotaHistoryEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e QuotaHistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}

// AppendQuotaHistory appends quota window snapshots using DefaultQuotaHistoryThrottle.
func AppendQuotaHistory(historyDir string, entries []QuotaHistoryEntry) error {
	return AppendQuotaHistoryWithThrottle(historyDir, entries, DefaultQuotaHistoryThrottle, time.Now())
}

// AppendQuotaHistoryForAgent converts u's quota windows and appends them using DefaultQuotaHistoryThrottle.
func AppendQuotaHistoryForAgent(historyDir string, u AgentUsage, now time.Time) error {
	return AppendQuotaHistoryForAgentWithThrottle(historyDir, u, DefaultQuotaHistoryThrottle, now)
}

// AppendQuotaHistoryForAgentWithThrottle converts u's quota windows and appends them using the given throttle.
func AppendQuotaHistoryForAgentWithThrottle(historyDir string, u AgentUsage, throttle time.Duration, now time.Time) error {
	entries := AgentUsageToQuotaHistoryEntries(u, now)
	if len(entries) == 0 {
		return nil
	}
	return AppendQuotaHistoryWithThrottle(historyDir, entries, throttle, now)
}

// AppendQuotaHistoryWithThrottle appends quota window snapshots with deduplication and throttling.
func AppendQuotaHistoryWithThrottle(historyDir string, entries []QuotaHistoryEntry, throttle time.Duration, now time.Time) error {
	if len(entries) == 0 {
		return nil
	}
	if historyDir == "" {
		historyDir = HistoryDir("")
	}
	if err := os.MkdirAll(historyDir, 0700); err != nil {
		return fmt.Errorf("create quota history dir: %w", err)
	}

	path := QuotaHistoryPath(historyDir)
	lockFile, ok := lockHistoryFile(path)
	if !ok {
		return fmt.Errorf("could not lock quota history file %s", path)
	}
	defer unlockHistoryFile(lockFile)

	existing, _ := ReadQuotaHistory(historyDir)
	lastSeen := make(map[string]QuotaHistoryEntry, len(existing))
	for _, e := range existing {
		key := fmt.Sprintf("%s|%s|%s", e.Agent, e.Group, e.Window)
		lastSeen[key] = e
	}

	var toAppend []QuotaHistoryEntry
	for _, e := range entries {
		key := fmt.Sprintf("%s|%s|%s", e.Agent, e.Group, e.Window)
		last, exists := lastSeen[key]
		if !exists {
			toAppend = append(toAppend, e)
			lastSeen[key] = e
			continue
		}

		pctChanged := e.UsedPercent != last.UsedPercent || e.RemainingPercent != last.RemainingPercent
		resetChanged := false
		if (e.ResetAt == nil) != (last.ResetAt == nil) {
			resetChanged = true
		} else if e.ResetAt != nil && last.ResetAt != nil && !e.ResetAt.Equal(*last.ResetAt) {
			resetChanged = true
		}

		timeElapsed := throttle > 0 && e.Timestamp.Sub(last.Timestamp) >= throttle

		if pctChanged || resetChanged || timeElapsed {
			toAppend = append(toAppend, e)
			lastSeen[key] = e
		}
	}

	if len(toAppend) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open quota history file: %w", err)
	}
	defer f.Close()

	for _, e := range toAppend {
		line, err := json.Marshal(e)
		if err != nil {
			continue
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write quota history entry: %w", err)
		}
	}

	return nil
}
