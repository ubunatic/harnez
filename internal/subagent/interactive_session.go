package subagent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type codexSessionMeta struct {
	Type    string `json:"type"`
	Payload struct {
		ID        string `json:"id"`
		CWD       string `json:"cwd"`
		Timestamp string `json:"timestamp"`
	} `json:"payload"`
}

func findCodexInteractiveSessionID(home string, pid int, dir string, started time.Time) string {
	root := filepath.Join(home, ".codex")
	if configured := os.Getenv("CODEX_HOME"); configured != "" {
		root = configured
	}
	paths, err := filepath.Glob(filepath.Join(root, "sessions", "*", "*", "*", "rollout-*.jsonl"))
	if err != nil {
		return ""
	}
	type candidate struct {
		id   string
		path string
		time time.Time
	}
	var matches []candidate
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		var meta codexSessionMeta
		err = json.NewDecoder(bufio.NewReader(file)).Decode(&meta)
		_ = file.Close()
		if err != nil || meta.Type != "session_meta" || meta.Payload.ID == "" || filepath.Clean(meta.Payload.CWD) != filepath.Clean(dir) {
			continue
		}
		if !strings.HasSuffix(strings.TrimSuffix(filepath.Base(path), ".jsonl"), "-"+meta.Payload.ID) {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, meta.Payload.Timestamp)
		if err != nil || at.Before(started.Add(-2*time.Second)) {
			continue
		}
		matches = append(matches, candidate{id: meta.Payload.ID, path: path, time: at})
	}
	if pid > 0 {
		var owned []candidate
		fdDir := filepath.Join("/proc", stringPID(pid), "fd")
		fds, _ := os.ReadDir(fdDir)
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			target = strings.TrimSuffix(target, " (deleted)")
			for _, match := range matches {
				if filepath.Clean(target) == filepath.Clean(match.path) {
					owned = append(owned, match)
				}
			}
		}
		if len(owned) == 1 {
			return owned[0].id
		}
		if len(owned) > 1 {
			return ""
		}
	}
	if len(matches) == 1 {
		return matches[0].id
	}
	return ""
}

func findAgyInteractiveSessionID(home, procRoot string, pid int) string {
	if pid <= 0 {
		return ""
	}
	fdDir := filepath.Join(procRoot, stringPID(pid), "fd")
	fds, err := os.ReadDir(fdDir)
	if err != nil {
		return ""
	}
	root := filepath.Join(home, ".gemini", "antigravity-cli")
	var found string
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
		if err != nil {
			continue
		}
		target = strings.TrimSuffix(target, " (deleted)")
		if filepath.Dir(target) != filepath.Join(root, "presence") || !strings.HasSuffix(target, ".lock") {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(target), ".lock")
		if found != "" && found != id {
			return ""
		}
		found = id
	}
	if found == "" {
		return ""
	}
	for _, db := range []string{filepath.Join(root, "conversations", found+".db"), filepath.Join(root, found+".db")} {
		if info, err := os.Stat(db); err == nil && !info.IsDir() {
			return found
		}
	}
	return ""
}

func stringPID(pid int) string {
	return strconv.Itoa(pid)
}
