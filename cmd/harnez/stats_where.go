package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/xdgpath"
)

type telemetryStore struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	Newest    time.Time `json:"newest_record,omitempty"`
	Owner     string    `json:"owner"`
	Answers   string    `json:"answers"`
	Status    string    `json:"status"`
}

type storeSpec struct {
	path, owner, answers string
}

func runStatsWhere(w io.Writer, jsonOutput bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("stats --where: resolve home: %w", err)
	}
	dataRoot := filepath.Join(xdgpath.DataHome(), "harnez")
	cacheRoot := filepath.Join(xdgpath.CacheHome(), "harnez")
	stateRoot := filepath.Join(home, ".harnez")
	stateData := filepath.Join(home, ".local", "state", "harnez")
	if value := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(value) {
		stateData = filepath.Join(value, "harnez")
	}
	specs := []storeSpec{
		{filepath.Join(dataRoot, "telemetry.sqlite"), "telemetry", "Tool calls, CLI invocations, agent requests, and compaction events"},
		{filepath.Join(dataRoot, "usage-history"), "usage collector", "Per-host usage snapshots and quota window history"},
		{filepath.Join(dataRoot, "voice-input"), "voice input", "Voice recording and transcription history"},
		{filepath.Join(cacheRoot, "quota-cache-*.json"), "usage collector", "Latest cached provider quota readings"},
		{filepath.Join(stateData, "agents", "usage"), "usage collector", "Cached agent usage snapshots"},
		{filepath.Join(stateRoot, "sessions"), "sessionstate", "Harnez session command counts and reminders"},
		{filepath.Join(stateRoot, "agents"), "agent", "Harnez-managed agent session records"},
		{filepath.Join(stateRoot, "agymeter"), "AGY meter", "Antigravity quota and usage measurements"},
		{filepath.Join(stateRoot, "bench"), "benchmark", "Benchmark runs and results"},
		{filepath.Join(stateRoot, "tool_catalog.sqlite"), "telemetry migration", "Legacy tool-call database; retained only as migration source or backup"},
		{filepath.Join(stateRoot, "tool_catalog.sqlite.bak-*"), "telemetry migration", "Preserved legacy tool-call database backup"},
		{filepath.Join(dataRoot, "telemetry.db"), "legacy decoy", "Obsolete empty telemetry database; should be removed"},
	}
	var stores []telemetryStore
	for _, spec := range specs {
		paths, err := filepath.Glob(spec.path)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			stores = append(stores, telemetryStore{Path: spec.path, Owner: spec.owner, Answers: spec.answers, Status: "missing"})
			continue
		}
		for _, path := range paths {
			store := telemetryStore{Path: path, Owner: spec.owner, Answers: spec.answers}
			store.SizeBytes, store.Newest, err = inspectStore(path)
			if err != nil {
				store.Status = "unreadable: " + err.Error()
			} else if store.SizeBytes == 0 {
				store.Status = "EMPTY"
			} else {
				store.Status = "present"
			}
			stores = append(stores, store)
		}
	}
	if jsonOutput {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(stores)
	}
	fmt.Fprintln(w, "Harnez data stores (size, newest record, owner, purpose)")
	for _, store := range stores {
		newest := "—"
		if !store.Newest.IsZero() {
			newest = store.Newest.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%-9s %10s  %s\n          newest: %s | owner: %s | answers: %s\n", store.Status, formatStoreSize(store.SizeBytes), store.Path, newest, store.Owner, store.Answers)
	}
	return nil
}

func inspectStore(path string) (int64, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, time.Time{}, err
	}
	if !info.IsDir() {
		newest := info.ModTime()
		if strings.HasSuffix(path, ".jsonl") {
			if recordTime := newestJSONLTimestamp(path); !recordTime.IsZero() {
				newest = recordTime
			}
		} else if strings.HasSuffix(path, ".sqlite") {
			if recordTime := newestTelemetryTimestamp(path); !recordTime.IsZero() {
				newest = recordTime
			}
		}
		return info.Size(), newest, nil
	}
	var size int64
	newest := time.Time{}
	err = filepath.WalkDir(path, func(file string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		size += fileInfo.Size()
		stamp := fileInfo.ModTime()
		if strings.HasSuffix(file, ".jsonl") {
			if recordTime := newestJSONLTimestamp(file); !recordTime.IsZero() {
				stamp = recordTime
			}
		}
		if stamp.After(newest) {
			newest = stamp
		}
		return nil
	})
	return size, newest, err
}

func newestJSONLTimestamp(path string) time.Time {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}
	}
	defer f.Close()
	var newest time.Time
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 4*1024*1024)
	for scanner.Scan() {
		var row struct {
			Timestamp time.Time `json:"timestamp"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) == nil && row.Timestamp.After(newest) {
			newest = row.Timestamp
		}
	}
	return newest
}

func newestTelemetryTimestamp(path string) time.Time {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return time.Time{}
	}
	defer db.Close()
	var stamp string
	if err := db.QueryRow("SELECT MAX(created_at) FROM tool_calls").Scan(&stamp); err != nil {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, stamp); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func formatStoreSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f KiB", float64(size)/1024)
}
