package procs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"syscall"
	"time"
)

// TreeRoot identifies an owned process or an isolated provider group. Starttime
// prevents a recycled PID from being mistaken for the original writer.
type TreeRoot struct {
	PID       int
	Starttime uint64
	Group     bool
}

// TreeOptions bounds both graceful shutdown and confirmation after SIGKILL.
type TreeOptions struct {
	Roots                 []TreeRoot
	Grace, KillWait, Poll time.Duration
	Reader                ProcReader
	Signal                func(int, syscall.Signal) error
}

// ValidateTreeRoots rejects signal targets whose process identity is unknown.
// PID values at or below 1 are placeholders and are ignored.
func ValidateTreeRoots(roots []TreeRoot) error {
	for _, root := range roots {
		if root.PID > 1 && root.Starttime == 0 {
			return fmt.Errorf("cannot stop process %d: start time is unknown; cannot verify the target safely", root.PID)
		}
	}
	return nil
}

// ValidateTreeRootIdentities confirms that every recorded root is currently
// visible with its recorded start time. This check is a snapshot only; callers
// that signal later still have a PID reuse window.
func ValidateTreeRootIdentities(roots []TreeRoot, reader ProcReader) error {
	if reader == nil {
		reader = LinuxProcReader{}
	}
	processes, err := reader.List()
	if err != nil {
		return fmt.Errorf("cannot inspect process identities: %w", err)
	}
	for _, root := range roots {
		if root.PID <= 1 {
			continue
		}
		found := false
		for _, process := range processes {
			if process.PID != root.PID {
				continue
			}
			found = true
			if process.Starttime != root.Starttime {
				return fmt.Errorf("cannot stop process %d: recorded start time does not match the live process", root.PID)
			}
			break
		}
		if !found {
			return fmt.Errorf("cannot stop process %d: recorded process is not visible", root.PID)
		}
	}
	return nil
}

func liveProcess(p ProcInfo) bool { return p.State != "Z" && p.State != "X" }

// ProcessStarttime reads the kernel identity of an existing process, or 0
// when the process does not exist or /proc is unavailable.
func ProcessStarttime(pid int) uint64 {
	return LinuxProcReader{}.Starttime(pid)
}

// TreeMembers finds live descendants and members of explicitly owned groups.
// Previously discovered identities remain tracked even after reparenting.
func TreeMembers(roots []TreeRoot, known map[int]ProcInfo, reader ProcReader) ([]ProcInfo, error) {
	if len(known) == 0 && !slices.ContainsFunc(roots, func(r TreeRoot) bool { return r.PID > 1 }) {
		return nil, nil // nothing to track: skip the full process scan
	}
	realReader := reader == nil
	if reader == nil {
		reader = LinuxProcReader{}
	}
	all, err := reader.List()
	if err != nil {
		return nil, err
	}
	groups := make(map[int]uint64)
	identities := make(map[int]uint64)
	for _, p := range all {
		identities[p.PID] = p.Starttime
	}
	for _, p := range known {
		identity, exists := identities[p.PID]
		if p.PGID == p.PID && (!exists || identity == p.Starttime) {
			groups[p.PGID] = p.Starttime
		}
	}
	for _, r := range roots {
		if r.PID <= 1 {
			continue
		}
		valid := true
		for _, p := range all {
			if p.PID == r.PID && r.Starttime != 0 && p.Starttime != r.Starttime {
				valid = false
			}
		}
		if !valid {
			continue
		}
		if _, visible := identities[r.PID]; !visible && realReader {
			err := syscall.Kill(r.PID, 0)
			if err == nil || errors.Is(err, syscall.EPERM) {
				return nil, fmt.Errorf("cannot inspect live process %d to confirm exit", r.PID)
			}
		}
		if r.Group {
			groups[r.PID] = r.Starttime
		}
		for _, p := range all {
			if p.PID == r.PID {
				if !p.UIDKnown || p.UID != uint32(os.Getuid()) {
					return nil, fmt.Errorf("cannot establish ownership of process %d", p.PID)
				}
				known[p.PID] = p
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, p := range all {
			if !p.UIDKnown || p.UID != uint32(os.Getuid()) {
				continue
			}
			if previous, ok := known[p.PID]; ok && previous.Starttime == p.Starttime {
				continue
			}
			parent, child := known[p.PPID]
			child = child && identities[parent.PID] == parent.Starttime
			start, grouped := groups[p.PGID]
			if child || (grouped && p.Starttime >= start) {
				known[p.PID] = p
				if p.PGID == p.PID {
					groups[p.PGID] = p.Starttime
				}
				changed = true
			}
		}
	}
	var live []ProcInfo
	for _, p := range all {
		if previous, ok := known[p.PID]; ok && previous.Starttime == p.Starttime && liveProcess(p) {
			live = append(live, p)
		}
	}
	return live, nil
}

// StopTree sends TERM, escalates to KILL, and returns surviving PIDs. Zombies
// have exited and cannot hold a writer; their parent's reap notification may lag.
func StopTree(ctx context.Context, opts TreeOptions) ([]ProcInfo, error) {
	if err := ValidateTreeRoots(opts.Roots); err != nil {
		return nil, err
	}
	if opts.Grace <= 0 || opts.KillWait <= 0 || opts.Poll <= 0 {
		return nil, fmt.Errorf("stop tree: positive shutdown bounds required")
	}
	if opts.Signal == nil {
		opts.Signal = syscall.Kill
	}
	known := make(map[int]ProcInfo)
	var live []ProcInfo
	for _, phase := range []struct {
		signal   syscall.Signal
		duration time.Duration
	}{{syscall.SIGTERM, opts.Grace}, {syscall.SIGKILL, opts.KillWait}} {
		deadline := time.Now().Add(phase.duration)
		sent := make(map[int]uint64)
		for {
			var err error
			live, err = TreeMembers(opts.Roots, known, opts.Reader)
			if err != nil {
				return live, err
			}
			if len(live) == 0 {
				return nil, nil
			}
			for _, p := range live {
				if p.PID <= 1 || p.PID == os.Getpid() {
					return live, fmt.Errorf("refusing to signal own process or init (%d)", p.PID)
				}
			}
			for _, p := range live {
				if sent[p.PID] == p.Starttime {
					continue
				}
				if err := opts.Signal(p.PID, phase.signal); err != nil && !errors.Is(err, syscall.ESRCH) {
					return live, err
				}
				sent[p.PID] = p.Starttime
			}
			if time.Now().After(deadline) {
				break
			}
			timer := time.NewTimer(opts.Poll)
			select {
			case <-ctx.Done():
				timer.Stop()
				return live, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return TreeMembers(opts.Roots, known, opts.Reader)
}
