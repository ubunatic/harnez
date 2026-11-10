package procs

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestIsAliveSelf(t *testing.T) {
	if !IsAlive(os.Getpid()) {
		t.Fatal("IsAlive(self) = false; want true")
	}
}

// Issue 783: a killed child that nobody has waited for yet is a zombie. Signal 0 still succeeds
// for it, which is what made the process-group kill tests report it alive in containers.
func TestIsAliveTreatsUnreapedZombieAsDead(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("zombie state is read from /proc")
	}
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	defer cmd.Wait() //nolint:errcheck // reaps the zombie at the end
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for state(t, pid) != "Z" {
		if time.Now().After(deadline) {
			t.Fatalf("child %d never became a zombie", pid)
		}
		runtime.Gosched()
	}
	if IsAlive(pid) {
		t.Fatalf("IsAlive(%d) = true for an unreaped zombie; want false", pid)
	}
}

func state(t *testing.T, pid int) string {
	t.Helper()
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		t.Fatal(err)
	}
	info, ok := parseStat(pid, string(stat))
	if !ok {
		t.Fatalf("parse /proc/%d/stat: %q", pid, stat)
	}
	return info.State
}
