package main

import (
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/procs"
)

const (
	stoppedGroupGrace    = 300 * time.Millisecond
	stoppedPollInterval  = 25 * time.Millisecond
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
	processes, err := (procs.LinuxProcReader{}).List()
	if err != nil {
		return false, err
	}
	for _, process := range processes {
		if process.PGID == pgid && (process.State == "T" || process.State == "t") {
			return true, nil
		}
	}
	return false, nil
}

func signalProcessGroup(pgid int, signal syscall.Signal) error {
	return syscall.Kill(-pgid, signal)
}
