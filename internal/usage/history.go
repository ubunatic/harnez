package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// HistorySummaryData holds aggregated usage metrics across recorded history files.
type HistorySummaryData struct {
	FileCount    int           `json:"file_count"`
	TotalBytes   int64         `json:"total_bytes"`
	TotalEntries int           `json:"total_entries"`
	StartTokens  int64         `json:"start_tokens"`
	EndTokens    int64         `json:"end_tokens"`
	TotalUsed    int64         `json:"total_used"`
	Duration     time.Duration `json:"duration"`
	RatePerHour  int64         `json:"rate_per_hour"`
	RatePerDay   int64         `json:"rate_per_day"`
	Sparkline    string        `json:"sparkline,omitempty"`
}

// HistorySummaryStats inspects *.jsonl files in dir, loads and merges history entries,
// and computes overall file stats, total tokens consumed across the recorded timespan,
// time duration, consumption rates, and an overall sparkline.
func HistorySummaryStats(dir string) (HistorySummaryData, error) {
	var data HistorySummaryData

	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(matches) == 0 {
		return data, nil
	}

	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		data.FileCount++
		data.TotalBytes += info.Size()
	}

	entries, err := ReadHistory(dir)
	if err != nil {
		return data, err
	}
	data.TotalEntries = len(entries)
	if len(entries) == 0 {
		return data, nil
	}

	// Calculate per-entry total tokens across all agents
	var totalTokSeries []int64
	for _, e := range entries {
		var entryTotal int64
		for _, a := range e.Agents {
			if a.Installed && a.Authenticated && a.Tokens != nil {
				entryTotal += a.Tokens.TotalTokens
			}
		}
		totalTokSeries = append(totalTokSeries, entryTotal)
	}

	if len(totalTokSeries) > 0 {
		data.StartTokens = totalTokSeries[0]
		data.EndTokens = totalTokSeries[len(totalTokSeries)-1]
		used := data.EndTokens - data.StartTokens
		if used < 0 {
			used = 0
		}
		data.TotalUsed = used
		data.Duration = entries[len(entries)-1].Timestamp.Sub(entries[0].Timestamp)

		if data.Duration >= time.Minute && data.TotalUsed > 0 {
			hours := data.Duration.Hours()
			data.RatePerHour = int64(float64(data.TotalUsed) / hours)
			data.RatePerDay = int64(float64(data.TotalUsed) / (hours / 24.0))
		}
		data.Sparkline = RenderSparklineInt64Width(totalTokSeries, 6)
	}

	return data, nil
}

// HistoryStats inspects *.jsonl files in dir and returns the file count, total size in bytes,
// and total number of snapshot entries recorded across all files.
func HistoryStats(dir string) (fileCount int, totalBytes int64, totalEntries int) {
	stats, _ := HistorySummaryStats(dir)
	return stats.FileCount, stats.TotalBytes, stats.TotalEntries
}

// RenderHistoryStatsText formats aggregate history metrics into human-readable text.
func RenderHistoryStatsText(stats HistorySummaryData, dir string) string {
	var sb strings.Builder
	sb.WriteString("Usage History Statistics\n")
	if dir != "" {
		displayDir := dir
		if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(displayDir, home) {
			displayDir = "~" + strings.TrimPrefix(displayDir, home)
		}
		sb.WriteString(fmt.Sprintf("Directory:     %s\n", displayDir))
	}
	fileWord := "files"
	if stats.FileCount == 1 {
		fileWord = "file"
	}
	sb.WriteString(fmt.Sprintf("Files:         %d %s (%s)\n", stats.FileCount, fileWord, FormatBytes(stats.TotalBytes)))
	sb.WriteString(fmt.Sprintf("Snapshots:     %d\n", stats.TotalEntries))
	if stats.TotalEntries > 0 {
		sb.WriteString(fmt.Sprintf("Timespan:      %s\n", FormatDuration(stats.Duration)))
		sb.WriteString(fmt.Sprintf("Tokens Start:  %s\n", FormatNumber(stats.StartTokens)))
		sb.WriteString(fmt.Sprintf("Tokens End:    %s\n", FormatNumber(stats.EndTokens)))
		sb.WriteString(fmt.Sprintf("Tokens Used:   +%s\n", FormatNumber(stats.TotalUsed)))
		if stats.Duration >= time.Minute {
			sb.WriteString(fmt.Sprintf("Burn Rate:     %s /hr · %s /day\n", FormatNumber(stats.RatePerHour), FormatNumber(stats.RatePerDay)))
		}
		if stats.Sparkline != "" {
			sb.WriteString(fmt.Sprintf("Trajectory:    [%s]\n", stats.Sparkline))
		}
	}
	return sb.String()
}

// RenderHistoryStatsJSON formats aggregate history metrics as JSON.
func RenderHistoryStatsJSON(stats HistorySummaryData) (string, error) {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal history stats: %w", err)
	}
	return string(data), nil
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

// latestHistoryQuotaWindow scans dir's recorded history entries (see
// ReadHistory) for the most recent entry for agentID that carries actual
// quota-window data (Session/Weekly/ModelGroups — see hasQuotaWindowSignal;
// Tokens alone doesn't count, matching PersistAgentSnapshot's offline
// guard). ok is false when there is no history, or no entry for this agent
// ever had quota-window data. ReadHistory returns entries sorted oldest
// first, so this walks backwards to find the most recent qualifying one.
func latestHistoryQuotaWindow(dir, agentID string) (usage AgentUsage, at time.Time, ok bool) {
	entries, err := ReadHistory(dir)
	if err != nil {
		return AgentUsage{}, time.Time{}, false
	}
	for i := len(entries) - 1; i >= 0; i-- {
		for _, a := range entries[i].Agents {
			if a.AgentID == agentID && a.hasQuotaWindowSignal() {
				return a, entries[i].Timestamp, true
			}
		}
	}
	return AgentUsage{}, time.Time{}, false
}

// fillFromHistoryIfNoQuotaWindows returns u unchanged if it already carries
// quota-window data, or if no historical fallback is available. Otherwise
// it fills in Session/Weekly/ModelGroups from the most recent usage-history
// entry that had them (marked stale), and stamps LastRefreshed to that
// entry's own recorded time so the "last updated" display and staleness
// reporting stay honest about how old that quota data actually is (issue 103).
//
// This closes a gap found live on this machine (issue 086/103): the persisted
// collector-daemon snapshot for an intermittently-running agent (AGY) can go
// stale/empty for its quota windows specifically — every collector tick while
// AGY wasn't running wrote a snapshot with no Session/Weekly, and once the
// original snapshot was already empty, there was nothing richer left on disk
// for PersistAgentSnapshot's or cacheOrLive's guards to protect.
// usage-history/*.jsonl is a separate store from the collector-daemon cache
// and keeps real quota data from the last time AGY actually answered.
func fillFromHistoryIfNoQuotaWindows(historyDir string, u AgentUsage) AgentUsage {
	if u.hasQuotaWindowSignal() {
		return u
	}
	hist, at, ok := latestHistoryQuotaWindow(historyDir, u.AgentID)
	if !ok {
		return u
	}
	if hist.Session != nil {
		u.Session = staleQuotaWindow(hist.Session)
	}
	if hist.Weekly != nil {
		u.Weekly = staleQuotaWindow(hist.Weekly)
	}
	if len(hist.ModelGroups) > 0 {
		u.ModelGroups = staleModelGroups(hist.ModelGroups)
	}
	u.LastRefreshed = at
	u.Sources = append(u.Sources, fmt.Sprintf("%s (usage-history, stale)", historyDir))
	return u
}

// RenderTimelineText formats merged history entries as a flat, chronological
// table — one row per (snapshot, agent) pair — and includes per-agent and per-model
// sparkline trends when history is available.
func RenderTimelineText(entries []HistoryEntry) string {
	if len(entries) == 0 {
		return "No usage history recorded yet. Run `harnez usage history record` (or `--watch`) to start recording.\n"
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

	sparklines := RenderTimelineSparklines(entries)
	if sparklines != "" {
		sb.WriteString("\n")
		sb.WriteString(sparklines)
	}

	return sb.String()
}

// RenderTimelineSparklines aggregates token consumption across history entries
// and renders overall agent and per-model sparkline trajectories over time,
// including start/end tokens, total tokens used, and consumption rates per hour and day.
// It detects terminal width from os.Stdout automatically.
func RenderTimelineSparklines(entries []HistoryEntry) string {
	cols, _ := terminalSize(os.Stdout)
	return RenderTimelineSparklinesWidth(entries, cols)
}

// RenderTimelineSparklinesWidth renders timeline sparklines constrained to fit within termWidth columns.
func RenderTimelineSparklinesWidth(entries []HistoryEntry, termWidth int) string {
	if len(entries) == 0 {
		return ""
	}

	// Sort entries chronologically
	sorted := make([]HistoryEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp.Before(sorted[j].Timestamp) })

	// Discover all agents and their models in history
	agentIDs := make(map[string]bool)
	agentNames := make(map[string]string)
	agentTokens := make(map[string][]int64)
	agentTimestamps := make(map[string][]time.Time)
	agentModelTokens := make(map[string]map[string][]int64)
	agentModelTimestamps := make(map[string]map[string][]time.Time)

	for _, e := range sorted {
		// Collect for each agent in this entry
		for _, a := range e.Agents {
			if !a.Installed || !a.Authenticated {
				continue
			}
			agentIDs[a.AgentID] = true
			if agentNames[a.AgentID] == "" {
				agentNames[a.AgentID] = a.Name
			}

			var totalTok int64
			if a.Tokens != nil {
				totalTok = a.Tokens.TotalTokens
			}
			agentTokens[a.AgentID] = append(agentTokens[a.AgentID], totalTok)
			agentTimestamps[a.AgentID] = append(agentTimestamps[a.AgentID], e.Timestamp)

			if len(a.ModelTokens) > 0 {
				if agentModelTokens[a.AgentID] == nil {
					agentModelTokens[a.AgentID] = make(map[string][]int64)
					agentModelTimestamps[a.AgentID] = make(map[string][]time.Time)
				}
				for model, tok := range a.ModelTokens {
					agentModelTokens[a.AgentID][model] = append(agentModelTokens[a.AgentID][model], tok)
					agentModelTimestamps[a.AgentID][model] = append(agentModelTimestamps[a.AgentID][model], e.Timestamp)
				}
			}
		}
	}

	if len(agentIDs) == 0 {
		return ""
	}

	var sortedAgents []string
	for aid := range agentIDs {
		sortedAgents = append(sortedAgents, aid)
	}
	sort.Strings(sortedAgents)

	var sb strings.Builder
	sb.WriteString("Usage Trajectory Over Time:\n")

	formatStats := func(toks []int64, times []time.Time) string {
		if len(toks) == 0 {
			return "0 → 0 (+0 used) | - /hr | - /day"
		}
		startTok := toks[0]
		endTok := toks[len(toks)-1]
		used := endTok - startTok
		if used < 0 {
			used = 0
		}

		usedStr := fmt.Sprintf("+%s used", FormatNumber(used))
		if used == 0 {
			usedStr = "+0 used"
		}

		rateStr := "- /hr | - /day"
		if len(times) >= 2 {
			duration := times[len(times)-1].Sub(times[0])
			if duration >= time.Minute && used > 0 {
				hours := duration.Hours()
				perHour := int64(float64(used) / hours)
				perDay := int64(float64(used) / (hours / 24.0))
				rateStr = fmt.Sprintf("%s /hr | %s /day", FormatNumber(perHour), FormatNumber(perDay))
			} else if duration >= time.Minute && used == 0 {
				rateStr = "0 /hr | 0 /day"
			}
		}

		return fmt.Sprintf("%10s → %10s (%s) | %s", FormatNumber(startTok), FormatNumber(endTok), usedStr, rateStr)
	}

	// Calculate maximum sparkline width available to fit lines within termWidth columns.
	// Line structure:
	//   Agent: "  %-20s [%s]   %s" -> indent(2) + name(20) + " ["(2) + spark + "]   "(4) + stats
	//   Model: "    ↳ %-16s [%s]   %s" -> indent(4) + arrow(2) + name(16) + " ["(2) + spark + "]   "(4) + stats
	// Total non-spark width is approx 28 chars (prefix) + 5 chars (brackets & space) + stats (~60 chars) = ~93 chars.
	//
	// We dynamically compute stats max width to determine available spark width:
	// sparkW = termWidth - prefixLen - bracketsPadding(5) - statsLen
	calcSparkW := func(prefixLen, statsLen int) int {
		if termWidth <= 0 {
			return 12
		}
		// prefix + "[" + spark + "]   " + stats
		// safety margin of 1 column
		avail := termWidth - prefixLen - 5 - statsLen - safetyMargin
		if avail < 5 {
			avail = 5
		}
		if avail > 40 {
			avail = 40
		}
		return avail
	}

	for _, aid := range sortedAgents {
		name := agentNames[aid]
		if name == "" {
			name = aid
		}
		toks := agentTokens[aid]
		times := agentTimestamps[aid]
		stats := formatStats(toks, times)
		// Prefix: "  %-20s" -> 22 chars
		sparkW := calcSparkW(22, visLen(stats))
		spark := RenderSparklineInt64Width(toks, sparkW)
		sb.WriteString(fmt.Sprintf("  %-20s [%s]   %s\n", name, spark, stats))

		models := agentModelTokens[aid]
		if len(models) > 0 {
			var modelList []string
			for m := range models {
				modelList = append(modelList, m)
			}
			sort.Strings(modelList)
			for _, m := range modelList {
				mToks := models[m]
				mTimes := agentModelTimestamps[aid][m]
				mStats := formatStats(mToks, mTimes)
				// Prefix: "    ↳ %-16s" -> 4 + 2 (↳ + space) + 16 = 22 chars
				mSparkW := calcSparkW(22, visLen(mStats))
				mSpark := RenderSparklineInt64Width(mToks, mSparkW)
				sb.WriteString(fmt.Sprintf("    ↳ %-16s [%s]   %s\n", m, mSpark, mStats))
			}
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

var validSSHHostRe = regexp.MustCompile(`^[a-zA-Z0-9_.\-@:]+$`)

// FetchRemoteHistory fetches remote usage history *.jsonl files from an SSH host into localDir.
// It triggers a fresh remote snapshot over SSH beforehand so the fetched files are up-to-date.
func FetchRemoteHistory(ctx context.Context, sshHost string, localDir string, out io.Writer) ([]string, error) {
	sshHost = strings.TrimSpace(sshHost)
	if sshHost == "" {
		return nil, fmt.Errorf("ssh host cannot be empty")
	}
	if !validSSHHostRe.MatchString(sshHost) || strings.HasPrefix(sshHost, "-") {
		return nil, fmt.Errorf("invalid ssh host %q", sshHost)
	}

	if err := os.MkdirAll(localDir, 0700); err != nil {
		return nil, fmt.Errorf("create local history dir: %w", err)
	}

	// 1. Proactively attempt to trigger a fresh remote snapshot over SSH.
	// We run non-blocking snapshot collection ignoring errors (remote may not have harnez in PATH or installed).
	remoteCmd := `PATH="$PATH:$HOME/go/bin:$HOME/bin" harnez usage history record 2>/dev/null || true`
	cmdSnapshot := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", sshHost, remoteCmd)
	_ = cmdSnapshot.Run()

	// 2. Copy remote *.jsonl files into localDir via scp
	remoteSrc := fmt.Sprintf("%s:~/.claude/harnez/usage-history/*.jsonl", sshHost)
	cmdScp := exec.CommandContext(ctx, "scp", "-q", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", remoteSrc, localDir+"/")
	var errBuf strings.Builder
	cmdScp.Stderr = &errBuf
	if err := cmdScp.Run(); err != nil {
		return nil, fmt.Errorf("scp failed from %s: %w (%s)", sshHost, err, strings.TrimSpace(errBuf.String()))
	}

	// Discover matching files in localDir
	matches, _ := filepath.Glob(filepath.Join(localDir, "*.jsonl"))
	var fetched []string
	for _, m := range matches {
		fetched = append(fetched, filepath.Base(m))
	}
	sort.Strings(fetched)

	if out != nil && len(fetched) > 0 {
		fmt.Fprintf(out, "fetched %d history file(s) from %s: %s\n", len(fetched), sshHost, strings.Join(fetched, ", "))
	}

	return fetched, nil
}

