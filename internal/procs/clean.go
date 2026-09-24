package procs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultGracePeriod = time.Second

// ProcInfo contains the fields needed to validate and classify a process.
type ProcInfo struct {
	PID       int
	PGID      int
	UID       uint32
	UIDKnown  bool
	State     string
	Starttime uint64
}

// ProcReader lists processes. It is injectable for safe, deterministic tests.
type ProcReader interface {
	List() ([]ProcInfo, error)
}

// LinuxProcReader reads process identities from /proc.
type LinuxProcReader struct{ Root string }

func (r LinuxProcReader) List() ([]ProcInfo, error) {
	root := r.Root
	if root == "" {
		root = "/proc"
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var processes []ProcInfo
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		stat, err := os.ReadFile(filepath.Join(root, entry.Name(), "stat"))
		if err != nil {
			continue
		}
		info, ok := parseStat(pid, string(stat))
		if !ok {
			continue
		}
		status, err := os.ReadFile(filepath.Join(root, entry.Name(), "status"))
		if err != nil {
			processes = append(processes, info)
			continue
		}
		uid, ok := parseUID(string(status))
		if !ok {
			processes = append(processes, info)
			continue
		}
		info.UID = uid
		info.UIDKnown = true
		processes = append(processes, info)
	}
	return processes, nil
}

func parseStat(pid int, stat string) (ProcInfo, bool) {
	end := strings.LastIndex(stat, ")")
	if end < 0 || end+1 >= len(stat) {
		return ProcInfo{}, false
	}
	fields := strings.Fields(stat[end+1:])
	if len(fields) <= 19 {
		return ProcInfo{}, false
	}
	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return ProcInfo{}, false
	}
	starttime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return ProcInfo{}, false
	}
	return ProcInfo{PID: pid, PGID: pgid, State: fields[0], Starttime: starttime}, true
}

func parseUID(status string) (uint32, bool) {
	for _, line := range strings.Split(status, "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "Uid:"))
		if len(fields) == 0 {
			return 0, false
		}
		uid, err := strconv.ParseUint(fields[0], 10, 32)
		return uint32(uid), err == nil
	}
	return 0, false
}

// CleanOptions controls stale process cleanup. SignalGroup, GroupExists, and
// Sleep are injectable so unit tests never signal real processes.
type CleanOptions struct {
	RecordDir   string
	ProcReader  ProcReader
	UID         uint32
	Kill        bool
	GracePeriod time.Duration
	SignalGroup func(pgid int, signal syscall.Signal) error
	GroupExists func(pgid int) bool
	Sleep       func(time.Duration)
}

// Action is one process record's cleanup outcome.
type Action struct {
	PGID   int    `json:"pgid"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func groupExists(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func signalGroup(pgid int, signal syscall.Signal) error {
	return syscall.Kill(-pgid, signal)
}

// GroupGone reports whether a process group has no visible members and the
// kernel confirms that the group no longer exists.
func GroupGone(pgid int, reader ProcReader, exists func(int) bool) (bool, error) {
	if pgid <= 0 {
		return false, fmt.Errorf("invalid process group id %d", pgid)
	}
	if reader == nil {
		reader = LinuxProcReader{}
	}
	if exists == nil {
		exists = groupExists
	}
	processes, err := reader.List()
	if err != nil {
		return false, fmt.Errorf("list processes: %w", err)
	}
	for _, process := range processes {
		if process.PGID == pgid {
			return false, nil
		}
	}
	return !exists(pgid), nil
}

// CleanProcs reports stale process groups by default. With Kill set, it
// terminates stopped groups or groups whose owner process has exited.
func CleanProcs(opts CleanOptions) ([]Action, error) {
	if opts.RecordDir == "" {
		return nil, fmt.Errorf("process record directory is required")
	}
	if opts.ProcReader == nil {
		opts.ProcReader = LinuxProcReader{}
	}
	if opts.UID == 0 {
		opts.UID = uint32(os.Getuid())
	}
	if opts.SignalGroup == nil {
		opts.SignalGroup = signalGroup
	}
	if opts.GroupExists == nil {
		opts.GroupExists = groupExists
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}
	if opts.GracePeriod <= 0 {
		opts.GracePeriod = defaultGracePeriod
	}
	entries, err := os.ReadDir(opts.RecordDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read process records: %w", err)
	}
	actions := make([]Action, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(opts.RecordDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return actions, fmt.Errorf("read %s: %w", path, err)
		}
		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			return actions, fmt.Errorf("decode %s: %w", path, err)
		}
		namePGID, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil || namePGID != record.PGID || record.PGID <= 0 {
			return actions, fmt.Errorf("invalid process record filename %s", entry.Name())
		}
		action, err := cleanRecord(opts, path, record)
		actions = append(actions, action)
		if err != nil {
			return actions, err
		}
	}
	return actions, nil
}

func cleanRecord(opts CleanOptions, path string, record Record) (Action, error) {
	processes, err := opts.ProcReader.List()
	if err != nil {
		return Action{PGID: record.PGID, Status: "error", Reason: err.Error()}, fmt.Errorf("list processes: %w", err)
	}
	group := filterGroup(processes, record.PGID)
	if len(group) == 0 {
		if opts.GroupExists(record.PGID) {
			return Action{PGID: record.PGID, Status: "unknown", Reason: "group exists but has no readable /proc members"}, nil
		}
		if !opts.Kill {
			return Action{PGID: record.PGID, Status: "would-remove", Reason: "process group is gone"}, nil
		}
		if err := RemoveRecord(path); err != nil {
			return Action{PGID: record.PGID, Status: "error", Reason: err.Error()}, err
		}
		return Action{PGID: record.PGID, Status: "removed", Reason: "process group is gone"}, nil
	}
	stopped := false
	for _, process := range group {
		if !process.UIDKnown || process.UID != opts.UID {
			return Action{PGID: record.PGID, Status: "skipped", Reason: "cannot verify that every group member belongs to the current uid"}, nil
		}
		if process.PID == record.PGID {
			if record.PIDStarttime == 0 || process.Starttime != record.PIDStarttime {
				return removeReusedRecord(opts, path, record.PGID)
			}
		}
		if process.State == "T" || process.State == "t" {
			stopped = true
		}
	}
	ownerAlive := false
	for _, process := range processes {
		if process.PID == record.OwnerPID && process.UIDKnown && process.UID == opts.UID {
			ownerAlive = true
			break
		}
	}
	if !stopped && ownerAlive {
		return Action{PGID: record.PGID, Status: "kept", Reason: "owner is alive and group is not stopped"}, nil
	}
	reason := "owner process is gone"
	if stopped {
		reason = "process group is stopped"
	}
	if !opts.Kill {
		return Action{PGID: record.PGID, Status: "would-kill", Reason: reason}, nil
	}
	if record.PIDStarttime == 0 {
		return Action{PGID: record.PGID, Status: "skipped", Reason: "cannot verify process start time"}, nil
	}
	if err := signalIfSameGroup(opts, record, syscall.SIGTERM); err != nil {
		return Action{PGID: record.PGID, Status: "error", Reason: err.Error()}, err
	}
	if waitForGroupGone(opts, record.PGID, opts.GracePeriod) {
		return removeKilledRecord(opts, path, record.PGID)
	}
	if err := signalIfSameGroup(opts, record, syscall.SIGKILL); err != nil {
		return Action{PGID: record.PGID, Status: "error", Reason: err.Error()}, err
	}
	if waitForGroupGone(opts, record.PGID, opts.GracePeriod) {
		return removeKilledRecord(opts, path, record.PGID)
	}
	return Action{PGID: record.PGID, Status: "alive", Reason: "group still exists after SIGKILL"}, nil
}

func groupMembers(reader ProcReader, pgid int) ([]ProcInfo, error) {
	processes, err := reader.List()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	return filterGroup(processes, pgid), nil
}

func filterGroup(processes []ProcInfo, pgid int) []ProcInfo {
	var group []ProcInfo
	for _, process := range processes {
		if process.PGID == pgid {
			group = append(group, process)
		}
	}
	return group
}

func removeReusedRecord(opts CleanOptions, path string, pgid int) (Action, error) {
	if !opts.Kill {
		return Action{PGID: pgid, Status: "stale", Reason: "process start time does not match"}, nil
	}
	if err := RemoveRecord(path); err != nil {
		return Action{PGID: pgid, Status: "error", Reason: err.Error()}, err
	}
	return Action{PGID: pgid, Status: "removed", Reason: "process start time does not match"}, nil
}

func signalIfSameGroup(opts CleanOptions, record Record, signal syscall.Signal) error {
	group, err := groupMembers(opts.ProcReader, record.PGID)
	if err != nil {
		return err
	}
	if len(group) == 0 {
		return nil
	}
	for _, process := range group {
		if !process.UIDKnown || process.UID != opts.UID {
			return fmt.Errorf("process group %d has an unverified uid or another uid; refusing to signal", record.PGID)
		}
		if process.PID == record.PGID && process.Starttime != record.PIDStarttime {
			return fmt.Errorf("process group %d leader start time changed; refusing to signal", record.PGID)
		}
	}
	return opts.SignalGroup(record.PGID, signal)
}

func waitForGroupGone(opts CleanOptions, pgid int, period time.Duration) bool {
	step := 25 * time.Millisecond
	for elapsed := time.Duration(0); elapsed < period; elapsed += step {
		gone, err := GroupGone(pgid, opts.ProcReader, opts.GroupExists)
		if err == nil && gone {
			return true
		}
		opts.Sleep(step)
	}
	gone, err := GroupGone(pgid, opts.ProcReader, opts.GroupExists)
	return err == nil && gone
}

func removeKilledRecord(opts CleanOptions, path string, pgid int) (Action, error) {
	if err := RemoveRecord(path); err != nil {
		return Action{PGID: pgid, Status: "error", Reason: err.Error()}, err
	}
	return Action{PGID: pgid, Status: "killed", Reason: "stale process group terminated"}, nil
}
