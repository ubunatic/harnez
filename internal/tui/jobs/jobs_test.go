package jobs

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// uiLoop stands in for loom's event loop: post never blocks, run drains.
type uiLoop struct {
	mu sync.Mutex
	q  []func()
}

func (l *uiLoop) post(fn func()) {
	l.mu.Lock()
	l.q = append(l.q, fn)
	l.mu.Unlock()
}

func (l *uiLoop) runOnce() int {
	l.mu.Lock()
	q := l.q
	l.q = nil
	l.mu.Unlock()
	for _, fn := range q {
		fn()
	}
	return len(q)
}

// until drains callbacks on the test goroutine until cond holds.
func (l *uiLoop) until(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if l.runOnce() == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
	}
}

func newTestManager(t *testing.T) (*Manager, *uiLoop) {
	t.Helper()
	l := &uiLoop{}
	m := NewManager(Options{Shell: "sh", Dir: t.TempDir(), LogBytes: 4096, CancelGrace: 500 * time.Millisecond}, l.post)
	t.Cleanup(m.Shutdown)
	return m, l
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// groupMembers lists pids whose process group is pgid (zombies excluded).
func groupMembers(t *testing.T, pgid int) []int {
	t.Helper()
	ents, err := os.ReadDir("/proc")
	if err != nil {
		t.Skip("no /proc")
	}
	var out []int
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(b)
		f := strings.Fields(s[strings.LastIndexByte(s, ')')+2:])
		if len(f) > 2 && f[0] != "Z" && f[2] == strconv.Itoa(pgid) {
			out = append(out, pid)
		}
	}
	return out
}

func TestJobStateTransitions(t *testing.T) {
	m, l := newTestManager(t)
	var done []int
	m.OnDone = func(j *Job) { done = append(done, j.ID) }
	ok, err := m.Start("echo hello; echo oops >&2")
	if err != nil {
		t.Fatal(err)
	}
	bad, err := m.Start("exit 3")
	if err != nil {
		t.Fatal(err)
	}
	if ok.ID != 1 || bad.ID != 2 || ok.Status != Running || m.Running() != 2 || ok.PID <= 1 {
		t.Fatalf("after start: %+v %+v running=%d", ok, bad, m.Running())
	}
	l.until(t, func() bool { return m.Running() == 0 })
	if ok.Status != Succeeded || ok.ExitCode != 0 || ok.End.Before(ok.Start) {
		t.Fatalf("ok job: %+v", ok)
	}
	if bad.Status != Failed || bad.ExitCode != 3 {
		t.Fatalf("bad job: %+v", bad)
	}
	l.runOnce()
	if got := strings.Join(ok.Lines(), "|"); got != "hello|oops" {
		t.Fatalf("output %q", got)
	}
	if len(done) != 2 {
		t.Fatalf("OnDone ran for %v", done)
	}
	if m.Get(2) != bad || m.Get(9) != nil {
		t.Fatal("Get lookup")
	}
}

func TestRingBound(t *testing.T) {
	r := NewRing(10)
	for i := 0; i < 7; i++ {
		_, _ = r.Write([]byte("abc"))
	}
	if r.Len() != 10 || !r.Truncated() || string(r.Bytes()) != "cabcabcabc" {
		t.Fatalf("ring %q len %d", r.Bytes(), r.Len())
	}
	_, _ = r.Write(bytes.Repeat([]byte("x"), 25))
	if string(r.Bytes()) != strings.Repeat("x", 10) {
		t.Fatalf("big write %q", r.Bytes())
	}
	r = NewRing(64)
	_, _ = r.Write([]byte("a\tb\x1b[31mred\x1b[0m\r\nprog 1\rprog 2\n"))
	if got := strings.Join(r.Lines(), "|"); got != "a    bred|prog 2" {
		t.Fatalf("lines %q", got)
	}
}

func TestJobOutputBounded(t *testing.T) {
	m, l := newTestManager(t)
	j, err := m.Start("i=0; while test $i -lt 2000; do echo line-$i; i=$((i+1)); done")
	if err != nil {
		t.Fatal(err)
	}
	l.until(t, func() bool { return j.Status != Running })
	l.runOnce()
	if j.out.Len() > 4096 || !j.out.Truncated() {
		t.Fatalf("ring len %d truncated %v", j.out.Len(), j.out.Truncated())
	}
	lines := j.Lines()
	if lines[len(lines)-1] != "line-1999" || strings.HasPrefix(lines[0], "line-0") {
		t.Fatalf("kept %q..%q", lines[0], lines[len(lines)-1])
	}
}

func TestCancelKillsWholeTree(t *testing.T) {
	m, l := newTestManager(t)
	j, err := m.Start("sleep 30 & sleep 30")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(groupMembers(t, j.PID)) < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := len(groupMembers(t, j.PID)); n < 3 {
		t.Fatalf("group has %d members, want sh + 2 sleeps", n)
	}
	if !m.Cancel(j.ID) || m.Cancel(j.ID) {
		t.Fatal("Cancel should succeed once")
	}
	l.until(t, func() bool { return j.Status != Running })
	if j.Status != Cancelled {
		t.Fatalf("status %v", j.Status)
	}
	if left := groupMembers(t, j.PID); len(left) > 0 {
		t.Fatalf("orphans left in group %d: %v", j.PID, left)
	}
	if m.Cancel(j.ID) {
		t.Fatal("cancel of an ended job")
	}
}

func TestCancelEscalatesToKill(t *testing.T) {
	m, l := newTestManager(t)
	j, err := m.Start("trap '' TERM; echo ready; while :; do sleep 0.05; done")
	if err != nil {
		t.Fatal(err)
	}
	l.until(t, func() bool { return len(j.Lines()) > 0 })
	start := time.Now()
	m.Cancel(j.ID)
	l.until(t, func() bool { return j.Status != Running })
	if j.Status != Cancelled || time.Since(start) < 400*time.Millisecond {
		t.Fatalf("status %v after %v", j.Status, time.Since(start))
	}
	if left := groupMembers(t, j.PID); len(left) > 0 {
		t.Fatalf("orphans: %v", left)
	}
}

func TestLeaderExitReapsBackgroundChildren(t *testing.T) {
	m, l := newTestManager(t)
	j, err := m.Start("sleep 30 & echo bye")
	if err != nil {
		t.Fatal(err)
	}
	l.until(t, func() bool { return j.Status != Running })
	if j.Status != Succeeded {
		t.Fatalf("status %v", j.Status)
	}
	if left := groupMembers(t, j.PID); len(left) > 0 {
		t.Fatalf("orphans: %v", left)
	}
}

func TestShutdownLeavesNoProcesses(t *testing.T) {
	l := &uiLoop{}
	m := NewManager(Options{Shell: "sh", Dir: t.TempDir(), LogBytes: 1024, CancelGrace: 300 * time.Millisecond}, l.post)
	var pids []int
	for _, c := range []string{"sleep 30 & sleep 30", "trap '' TERM; sleep 30", "echo quick"} {
		j, err := m.Start(c)
		if err != nil {
			t.Fatal(err)
		}
		pids = append(pids, j.PID)
	}
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	m.Shutdown() // the UI loop never runs again: posted callbacks are dropped
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("shutdown took %v", d)
	}
	for _, pid := range pids {
		if left := groupMembers(t, pid); len(left) > 0 {
			t.Fatalf("group %d survived shutdown: %v", pid, left)
		}
		if alive(pid) {
			t.Fatalf("leader %d still alive", pid)
		}
	}
}

func TestStartErrorKeepsRegistryEmpty(t *testing.T) {
	m := NewManager(Options{Shell: "/nonexistent/sh", LogBytes: 16, CancelGrace: time.Second}, func(func()) {})
	if _, err := m.Start("true"); err == nil {
		t.Fatal("expected start error")
	}
	if len(m.Jobs()) != 0 {
		t.Fatal("failed start registered a job")
	}
}
