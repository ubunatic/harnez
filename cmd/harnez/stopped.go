package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	stoppedGroupGrace    = 300 * time.Millisecond
	stoppedPollInterval  = time.Second
	stoppedGroupExitCode = 125
)

type stoppedGroupOutcome struct {
	Detected bool
	Killed   bool
}

func monitorStoppedGroup(done <-chan struct{}, pgid int, grace, interval time.Duration,
	isStopped func(int) (bool, error), signal func(int, syscall.Signal) error) stoppedGroupOutcome {
	outcome := stoppedGroupOutcome{}
	if grace <= 0 {
		grace = stoppedGroupGrace
	}
	if interval <= 0 {
		interval = stoppedPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var stoppedAt time.Time
	continued := false
	for {
		select {
		case <-done:
			return outcome
		case now := <-ticker.C:
			stopped, err := isStopped(pgid)
			if err != nil {
				return outcome
			}
			if !stopped {
				if !stoppedAt.IsZero() && continued {
					return outcome
				}
				stoppedAt = time.Time{}
				continue
			}
			if stoppedAt.IsZero() {
				stoppedAt = now
				continue
			}
			if now.Sub(stoppedAt) < grace {
				continue
			}
			outcome.Detected = true
			if !continued {
				_ = signal(pgid, syscall.SIGCONT)
				continued = true
				stoppedAt = now
				continue
			}
			_ = signal(pgid, syscall.SIGKILL)
			outcome.Killed = true
			return outcome
		}
	}
}

func processGroupStopped(pgid int) (bool, error) {
	return processGroupStoppedAt("/proc", pgid)
}

// processGroupStoppedAt walks descendants through each known process's task
// children files and reads stat only for that tree. exec creates a new process
// group whose leader is pid, so stopped descendants are relevant when they
// still belong to that group.
func processGroupStoppedAt(procRoot string, pid int) (bool, error) {
	queue := []int{pid}
	seen := make(map[int]struct{})
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}

		processDir := filepath.Join(procRoot, strconv.Itoa(current))
		stat, err := os.ReadFile(filepath.Join(processDir, "stat"))
		if err != nil {
			if current == pid || !os.IsNotExist(err) {
				return false, err
			}
			continue
		}
		state, processGroup, ok := parseStoppedProcessStat(stat)
		if ok && processGroup == pid && (state == 'T' || state == 't') {
			return true, nil
		}

		tasks, err := os.ReadDir(filepath.Join(processDir, "task"))
		if err != nil {
			if !os.IsNotExist(err) {
				return false, err
			}
			continue
		}
		for _, task := range tasks {
			if _, err := strconv.Atoi(task.Name()); err != nil || !task.IsDir() {
				continue
			}
			children, err := os.ReadFile(filepath.Join(processDir, "task", task.Name(), "children"))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return false, err
			}
			for _, child := range strings.Fields(string(children)) {
				childPID, err := strconv.Atoi(child)
				if err == nil && childPID > 0 {
					queue = append(queue, childPID)
				}
			}
		}
	}
	return false, nil
}

func parseStoppedProcessStat(stat []byte) (state byte, pgid int, ok bool) {
	end := strings.LastIndex(string(stat), ")")
	if end < 0 || end+1 >= len(stat) {
		return 0, 0, false
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) < 3 || len(fields[0]) != 1 {
		return 0, 0, false
	}
	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, false
	}
	return fields[0][0], pgid, true
}

func signalProcessGroup(pgid int, signal syscall.Signal) error {
	return syscall.Kill(-pgid, signal)
}
