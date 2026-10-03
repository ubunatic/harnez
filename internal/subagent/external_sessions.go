package subagent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DiscoverExternalSessions returns local sessions described by the embedded
// provider discovery spec. These records are selectable metadata, not proof a
// provider process is currently running.
func DiscoverExternalSessions(home string) ([]*Session, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	var sessions []*Session
	seen := make(map[string]struct{})
	providers := make([]string, 0, len(spec.ExternalSessions))
	for provider := range spec.ExternalSessions {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		discovery := spec.ExternalSessions[provider]
		if discovery.Format != "jsonl" {
			return nil, fmt.Errorf("external session spec: unsupported format %q for %s", discovery.Format, provider)
		}
		if discovery.MaxRecordBytes <= 0 {
			return nil, fmt.Errorf("external session spec: positive max_record_bytes required for %s", provider)
		}
		root := strings.ReplaceAll(discovery.Root, "{home}", home)
		matches, err := filepath.Glob(filepath.Join(root, discovery.Pattern))
		if err != nil {
			return nil, fmt.Errorf("external session spec: glob %s: %w", provider, err)
		}
		for _, path := range matches {
			session, err := readExternalSession(provider, discovery, path)
			if err != nil {
				// Provider files can be truncated while the CLI is writing them.
				// Skip malformed or oversized metadata and keep usable sessions visible.
				continue
			}
			key := provider + "\x00" + session.ProviderID()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// DiscoverActiveCodexSessions returns Codex rollouts tied to a currently
// running interactive Codex CLI process. Codex does not expose a thread ID in
// the process environment, so new sessions are matched by working directory
// and process/session start time. Resumed sessions are matched by their ID in
// the command line.
func DiscoverActiveCodexSessions(home string) ([]*Session, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	window := time.Duration(spec.ExternalSessions["codex"].ActiveMatchWindowSeconds) * time.Second
	all, err := DiscoverExternalSessions(home)
	if err != nil {
		return nil, err
	}
	processes, err := activeCodexProcesses()
	if err != nil {
		return nil, err
	}
	used := make(map[int]struct{})
	var active []*Session
	currentIDs := map[string]struct{}{}
	for _, key := range []string{"CODEX_THREAD_ID", "CODEX_SESSION_ID"} {
		if id := os.Getenv(key); id != "" {
			currentIDs[id] = struct{}{}
		}
	}
	for i, session := range all {
		if _, ok := currentIDs[session.ProviderID()]; ok {
			used[i] = struct{}{}
			session.Status = "active"
			populateCodexContextMetrics(session)
			active = append(active, session)
		}
	}
	for _, process := range processes {
		best := -1
		for i, session := range all {
			if _, ok := used[i]; ok || session.Provider != "codex" {
				continue
			}
			if process.sessionID != "" && session.ProviderID() == process.sessionID {
				best = i
				break
			}
			if filepath.Clean(session.WorkingDir) != filepath.Clean(process.workingDir) {
				continue
			}
			delta := process.startedAt.Sub(session.CreatedAt)
			// A long-lived Codex process can switch to a thread created later.
			// Keep candidates created near process start or at any later time,
			// then select the one whose rollout was updated most recently.
			if delta > window {
				continue
			}
			if best < 0 || session.LastActiveAt.After(all[best].LastActiveAt) {
				best = i
			}
		}
		if best >= 0 {
			used[best] = struct{}{}
			all[best].Status = "active"
			populateCodexContextMetrics(all[best])
			active = append(active, all[best])
		}
	}
	return active, nil
}

func populateCodexContextMetrics(session *Session) {
	if session == nil || session.externalRolloutPath == "" {
		return
	}
	usage, _, err := codexRolloutUsageAtPath(session.externalRolloutPath, session.ProviderID())
	if usage.ContextTokens > 0 {
		session.ContextTokens = usage.ContextTokens
	}
	if usage.ContextWindowSize > 0 {
		session.ContextWindowSize = usage.ContextWindowSize
	}
	if usage.SessionUsedKnown {
		session.SessionUsedTokens = usage.SessionUsedTokens
		session.SessionUsedKnown = true
	}
	if err != nil && usage.ContextTokens <= 0 {
		session.ContextTokens = -1
	}
}

type codexProcess struct {
	workingDir string
	startedAt  time.Time
	sessionID  string
}

func activeCodexProcesses() ([]codexProcess, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read process table for active Codex sessions: %w", err)
	}
	var processes []codexProcess
	for _, entry := range entries {
		if !entry.IsDir() || !isDigits(entry.Name()) {
			continue
		}
		proc := filepath.Join("/proc", entry.Name())
		cmdline, err := os.ReadFile(filepath.Join(proc, "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(string(cmdline), "\x00")
		if len(args) < 1 || !strings.Contains(filepath.Base(args[0]), "codex") || strings.Contains(filepath.Base(args[0]), "code-mode") {
			continue
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "app-server") || strings.Contains(joined, "pid-update-loop") {
			continue
		}
		workingDir, err := os.Readlink(filepath.Join(proc, "cwd"))
		if err != nil {
			continue
		}
		startedAt, err := processStartTime(entry.Name())
		if err != nil {
			continue
		}
		process := codexProcess{workingDir: workingDir, startedAt: startedAt}
		for i, arg := range args {
			if arg == "resume" && i+1 < len(args) {
				process.sessionID = args[i+1]
				break
			}
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func processStartTime(pid string) (time.Time, error) {
	data, err := os.ReadFile(filepath.Join("/proc", pid, "stat"))
	if err != nil {
		return time.Time{}, err
	}
	// The command name in /proc/<pid>/stat is parenthesized and may contain spaces.
	fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
	if len(fields) <= 19 {
		return time.Time{}, fmt.Errorf("incomplete process stat")
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	bootData, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}
	var bootTime int64
	for _, line := range strings.Split(string(bootData), "\n") {
		if _, err := fmt.Sscanf(line, "btime %d", &bootTime); err == nil {
			break
		}
	}
	if bootTime == 0 {
		return time.Time{}, fmt.Errorf("process boot time not found")
	}
	// Linux proc start ticks use USER_HZ, which is 100 on supported hosts.
	return time.Unix(bootTime+int64(startTicks/100), int64(startTicks%100)*int64(time.Second)/100), nil
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func readExternalSession(provider string, spec ExternalSessionSpec, path string) (*Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(io.LimitReader(file, int64(spec.MaxRecordBytes)+1))
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	if len(line) > spec.MaxRecordBytes {
		return nil, fmt.Errorf("external session record exceeds configured limit")
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		return nil, err
	}
	if externalStringField(record, spec.RecordTypePath) != spec.RecordType {
		return nil, fmt.Errorf("external session metadata record not found")
	}
	id := externalStringField(record, spec.Fields.ID)
	workingDir := externalStringField(record, spec.Fields.WorkingDir)
	if id == "" || workingDir == "" {
		return nil, fmt.Errorf("external session metadata is missing identity or working directory")
	}
	providerID := externalStringField(record, spec.Fields.ProviderSessionID)
	if providerID == "" {
		providerID = id
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, externalStringField(record, spec.Fields.CreatedAt))
	shortID := id
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return &Session{
		ID:                  id,
		ProviderSessionID:   providerID,
		Name:                spec.NamePrefix + "-" + shortID,
		Provider:            provider,
		Source:              externalStringField(record, spec.Fields.Source),
		ModelProvider:       externalStringField(record, spec.Fields.ModelProvider),
		WorkingDir:          workingDir,
		HarnessType:         "external",
		Status:              spec.AvailableStatus,
		CreatedAt:           createdAt,
		LastActiveAt:        info.ModTime(),
		externalRolloutPath: path,
	}, nil
}

func externalStringField(record map[string]any, path string) string {
	var value any = record
	for _, part := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		value = object[part]
	}
	text, _ := value.(string)
	return text
}
