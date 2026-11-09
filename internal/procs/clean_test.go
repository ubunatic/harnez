package procs

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fakeProcReader struct{ processes []ProcInfo }

func (r *fakeProcReader) List() ([]ProcInfo, error) {
	return append([]ProcInfo(nil), r.processes...), nil
}

func writeTestRecord(t *testing.T, dir string, record Record) string {
	t.Helper()
	path, err := WriteRecord(dir, record)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLinuxProcReaderParsesStatAndUID(t *testing.T) {
	root := t.TempDir()
	pidDir := filepath.Join(root, "321")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fields := []string{"T", "1", "321"}
	for field := 6; field <= 21; field++ {
		fields = append(fields, "2")
	}
	fields = append(fields, "987654")
	stat := "321 (command with ) parens) " + strings.Join(fields, " ")
	if err := os.WriteFile(filepath.Join(pidDir, "stat"), []byte(stat), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "status"), []byte("Name:\ttest\nUid:\t1001\t1001\t1001\t1001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processes, err := (LinuxProcReader{Root: root}).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 1 || processes[0] != (ProcInfo{PID: 321, PPID: 1, PGID: 321, UID: 1001, UIDKnown: true, State: "T", Starttime: 987654}) {
		t.Fatalf("processes = %+v; want parsed stopped process", processes)
	}
	reader := LinuxProcReader{Root: root}
	if got := reader.Starttime(321); got != 987654 {
		t.Fatalf("Starttime(321) = %d, want 987654", got)
	}
	if got := reader.Starttime(999); got != 0 {
		t.Fatalf("Starttime of a missing pid = %d, want 0", got)
	}
}

func TestProcessStarttimeMatchesFullScan(t *testing.T) {
	pid := os.Getpid()
	all, err := (LinuxProcReader{}).List()
	if err != nil {
		t.Skipf("no /proc: %v", err)
	}
	var want uint64
	for _, p := range all {
		if p.PID == pid {
			want = p.Starttime
		}
	}
	if got := ProcessStarttime(pid); got == 0 || got != want {
		t.Fatalf("ProcessStarttime(self) = %d, full scan = %d", got, want)
	}
}

func TestCleanProcsDryRunAndInjectedSignals(t *testing.T) {
	dir := t.TempDir()
	recordDir := filepath.Join(dir, "records")
	reader := &fakeProcReader{processes: []ProcInfo{
		{PID: 42, PGID: 42, UID: 1001, UIDKnown: true, State: "T", Starttime: 900},
		{PID: 43, PGID: 42, UID: 1001, UIDKnown: true, State: "S", Starttime: 901},
	}}
	path := writeTestRecord(t, recordDir, Record{PGID: 42, PIDStarttime: 900, OwnerPID: 1000})
	var signals []syscall.Signal
	opts := CleanOptions{
		RecordDir:  recordDir,
		ProcReader: reader,
		UID:        1001,
		GroupExists: func(int) bool {
			return len(reader.processes) != 0
		},
		SignalGroup: func(_ int, signal syscall.Signal) error {
			signals = append(signals, signal)
			reader.processes = nil
			return nil
		},
		Sleep: func(time.Duration) {},
	}
	actions, err := CleanProcs(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Status != "would-kill" || len(signals) != 0 {
		t.Fatalf("dry-run = %+v, signals=%v; want would-kill and no signals", actions, signals)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dry-run removed record: %v", err)
	}

	opts.Kill = true
	actions, err = CleanProcs(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Status != "killed" || len(signals) != 1 || signals[0] != syscall.SIGTERM {
		t.Fatalf("kill result = %+v, signals=%v; want killed after SIGTERM", actions, signals)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("killed record still exists, stat err = %v", err)
	}
}

// Characterization: matching kernel metadata does not authenticate a record.
func TestCleanProcsForgedRecordCanSignal(t *testing.T) {
	dir := t.TempDir()
	writeTestRecord(t, dir, Record{PGID: 42, PIDStarttime: 900, OwnerPID: 999, OwnerStarttime: 100})
	reader := &fakeProcReader{processes: []ProcInfo{{PID: 42, PGID: 42, UID: 1001, UIDKnown: true, State: "S", Starttime: 900}}}
	var signals []syscall.Signal
	actions, err := CleanProcs(CleanOptions{RecordDir: dir, ProcReader: reader, UID: 1001, Kill: true,
		GroupExists: func(int) bool { return len(reader.processes) != 0 },
		SignalGroup: func(pgid int, signal syscall.Signal) error {
			if pgid != 42 {
				t.Fatalf("unexpected group %d", pgid)
			}
			signals = append(signals, signal)
			reader.processes = nil
			return nil
		}, Sleep: func(time.Duration) {}})
	if err != nil || len(actions) != 1 || actions[0].Status != "killed" || len(signals) != 1 || signals[0] != syscall.SIGTERM {
		t.Fatalf("forged record cleanup = %+v, %v, signals=%v; want fake SIGTERM", actions, err, signals)
	}
}

func TestCleanProcsRefusesMixedUIDAndRemovesReusedPIDRecord(t *testing.T) {
	dir := t.TempDir()
	recordDir := filepath.Join(dir, "records")
	reader := &fakeProcReader{processes: []ProcInfo{
		{PID: 52, PGID: 52, UID: 1001, UIDKnown: true, State: "T", Starttime: 920},
		{PID: 53, PGID: 52, UID: 1002, UIDKnown: true, State: "S", Starttime: 921},
	}}
	writeTestRecord(t, recordDir, Record{PGID: 52, PIDStarttime: 920, OwnerPID: 999})
	var signals int
	opts := CleanOptions{RecordDir: recordDir, ProcReader: reader, UID: 1001, Kill: true,
		SignalGroup: func(int, syscall.Signal) error { signals++; return nil },
		GroupExists: func(int) bool { return true }}
	actions, err := CleanProcs(opts)
	if err != nil || len(actions) != 1 || actions[0].Status != "skipped" || signals != 0 {
		t.Fatalf("mixed uid cleanup = %+v, %v, signals=%d; want safe skip", actions, err, signals)
	}

	reader.processes = []ProcInfo{{PID: 52, PGID: 52, UID: 1001, UIDKnown: true, State: "T", Starttime: 999}}
	actions, err = CleanProcs(opts)
	if err != nil || len(actions) != 1 || actions[0].Status != "removed" || signals != 0 {
		t.Fatalf("reused pid cleanup = %+v, %v, signals=%d; want record removal without signal", actions, err, signals)
	}
}

func TestCleanProcsRemovesGoneRecord(t *testing.T) {
	recordDir := t.TempDir()
	path := writeTestRecord(t, recordDir, Record{PGID: 61, PIDStarttime: 930})
	reader := &fakeProcReader{}
	actions, err := CleanProcs(CleanOptions{RecordDir: recordDir, ProcReader: reader, Kill: true,
		GroupExists: func(int) bool { return false }})
	if err != nil || len(actions) != 1 || actions[0].Status != "removed" {
		t.Fatalf("gone group cleanup = %+v, %v; want removed", actions, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("gone record remains, stat err = %v", err)
	}
}

func TestCleanProcsDoesNotTrustReusedOwnerPID(t *testing.T) {
	recordDir := t.TempDir()
	record := Record{PGID: 82, PIDStarttime: 940, OwnerPID: 90, OwnerStarttime: 1200}
	path := writeTestRecord(t, recordDir, record)
	reader := &fakeProcReader{processes: []ProcInfo{
		{PID: 82, PGID: 82, UID: 1001, UIDKnown: true, State: "S", Starttime: 940},
		{PID: 90, PGID: 90, UID: 1001, UIDKnown: true, State: "S", Starttime: 2200},
	}}
	opts := CleanOptions{RecordDir: recordDir, ProcReader: reader, UID: 1001, GroupExists: func(int) bool { return true }}
	actions, err := CleanProcs(opts)
	if err != nil || len(actions) != 1 || actions[0].Status != "would-kill" || actions[0].Reason != "owner process is gone" {
		t.Fatalf("reused owner cleanup = %+v, %v; want stale owner", actions, err)
	}
	record.OwnerStarttime = 2200
	if _, err := WriteRecord(recordDir, record); err != nil {
		t.Fatal(err)
	}
	actions, err = CleanProcs(opts)
	if err != nil || len(actions) != 1 || actions[0].Status != "kept" {
		t.Fatalf("matching owner cleanup = %+v, %v; want kept", actions, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("matching owner record removed: %v", err)
	}
}

func TestCleanProcsEscalatesToSIGKILL(t *testing.T) {
	recordDir := t.TempDir()
	path := writeTestRecord(t, recordDir, Record{PGID: 62, PIDStarttime: 931, OwnerPID: 999})
	reader := &fakeProcReader{processes: []ProcInfo{{PID: 62, PGID: 62, UID: 1001, UIDKnown: true, State: "T", Starttime: 931}}}
	var signals []syscall.Signal
	opts := CleanOptions{RecordDir: recordDir, ProcReader: reader, UID: 1001, Kill: true,
		GroupExists: func(int) bool { return len(reader.processes) > 0 },
		SignalGroup: func(_ int, signal syscall.Signal) error {
			signals = append(signals, signal)
			if signal == syscall.SIGKILL {
				reader.processes = nil
			}
			return nil
		},
		Sleep: func(time.Duration) {}, GracePeriod: time.Millisecond}
	actions, err := CleanProcs(opts)
	if err != nil || len(actions) != 1 || actions[0].Status != "killed" || len(signals) != 2 || signals[0] != syscall.SIGTERM || signals[1] != syscall.SIGKILL {
		t.Fatalf("escalation = %+v, %v, signals=%v; want TERM then KILL", actions, err, signals)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("escalated process record remains, stat err = %v", err)
	}
}

func TestCleanProcsKillsOnlyChildItSpawned(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exec sleep 30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	waited := false
	defer func() {
		if !waited {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-wait //nolint:errcheck // wait for goroutine or process exit; the value is not needed
		}
	}()

	reader := LinuxProcReader{}
	var child ProcInfo
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		processes, err := reader.List()
		if err != nil {
			t.Fatal(err)
		}
		for _, process := range processes {
			if process.PID == cmd.Process.Pid {
				child = process
				break
			}
		}
		if child.PID != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if child.PID == 0 || child.Starttime == 0 {
		t.Fatal("could not read spawned child identity")
	}
	recordDir := t.TempDir()
	path := writeTestRecord(t, recordDir, Record{
		PGID: cmd.Process.Pid, PIDStarttime: child.Starttime, OwnerPID: cmd.Process.Pid + 1_000_000,
		Argv: cmd.Args, CWD: dirCurrent(), Started: time.Now(),
	})
	actions, err := CleanProcs(CleanOptions{RecordDir: recordDir, Kill: true, UID: uint32(os.Getuid()),
		GracePeriod: 250 * time.Millisecond})
	if err != nil || len(actions) != 1 || actions[0].Status != "killed" {
		t.Fatalf("child cleanup = %+v, %v; want killed", actions, err)
	}
	waitErr := <-wait
	waited = true
	if waitErr == nil {
		t.Fatal("killed child exited normally")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("child record remains, stat err = %v", err)
	}
}

func dirCurrent() string {
	dir, _ := os.Getwd()
	return dir
}

func TestWriteRecordIsJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeTestRecord(t, dir, Record{PGID: 72, PIDStarttime: 940, Quota1: true})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Record
	if err := json.Unmarshal(data, &got); err != nil || got.PGID != 72 || !got.Quota1 {
		t.Fatalf("record = %+v, err=%v", got, err)
	}
	if filepath.Base(path) != strconv.Itoa(got.PGID)+".json" {
		t.Fatalf("record path = %q, pgid=%d", path, got.PGID)
	}
}

func TestRecordDirUsesRunnerRuntimeForWriterAndCleanup(t *testing.T) {
	runner := t.TempDir()
	if err := os.Chmod(runner, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runner)
	t.Setenv("HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	dir, err := RecordDir()
	if err != nil {
		t.Fatal(err)
	}
	path, err := WriteRecord(dir, Record{PGID: 73})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir || dir != filepath.Join(runner, "runtime", "procs") {
		t.Fatalf("record path = %q, directory = %q", path, dir)
	}
	entries, err := os.ReadDir(filepath.Join(runner, "runtime", "procs"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "73.json" {
		t.Fatalf("cleanup-visible entries = %v, %v", entries, err)
	}
	actions, err := CleanProcs(CleanOptions{RecordDir: dir, ProcReader: &fakeProcReader{}, Kill: true, GroupExists: func(int) bool { return false }})
	if err != nil || len(actions) != 1 || actions[0].Status != "removed" {
		t.Fatalf("cleanup using writer directory = %+v, %v; want record removed", actions, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleanup left runner process record, stat err = %v", err)
	}
}

func TestRecordDirKeepsLegacyEnvironmentPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TAIA_SESSION_DIR", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	dir, err := RecordDir()
	if err != nil || dir != filepath.Join(home, ".harnez", "run", "procs") {
		t.Fatalf("legacy home process record dir = %q, %v", dir, err)
	}
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	dir, err = RecordDir()
	if err != nil || dir != filepath.Join(runtimeDir, "harnez", "procs") {
		t.Fatalf("legacy XDG process record dir = %q, %v", dir, err)
	}
}

func TestTreeMembersWithoutRootsSkipsScan(t *testing.T) {
	reader := &countingProcReader{}
	members, err := TreeMembers([]TreeRoot{{PID: 0}, {PID: 1}}, map[int]ProcInfo{}, reader)
	if err != nil || members != nil || reader.calls != 0 {
		t.Fatalf("TreeMembers without roots = (%v, %v), want (nil, nil) without scanning", members, err)
	}
}

type countingProcReader struct{ calls int }

func (r *countingProcReader) List() ([]ProcInfo, error) {
	r.calls++
	return nil, nil
}
