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

// processGroupStoppedAt reports whether the process-group leader is stopped.
// exec creates a new process group whose leader is its direct child, so reading
// that one stat file detects the stopped-child failure mode without repeatedly
// enumerating every process on the host while the child is simply waiting.
func processGroupStoppedAt(procRoot string, pid int) (bool, error) {
	stat, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if err != nil {
		return false, err
	}
	end := strings.LastIndex(string(stat), ")")
	if end < 0 || end+2 >= len(stat) {
		return false, nil
	}
	state := stat[end+2]
	return state == 'T' || state == 't', nil
}

func signalProcessGroup(pgid int, signal syscall.Signal) error {
	return syscall.Kill(-pgid, signal)
}
