package procs

import (
	"os"
	"strconv"
	"syscall"
)

// IsAlive reports whether pid still runs. Signal 0 alone also succeeds for a zombie that no
// init reaped (issue 783: containers whose PID 1 is not a reaping init), so on Linux a process
// in state Z (zombie) or X (dead) counts as gone. Without a readable /proc/<pid>/stat (other
// systems, or a race with the exit) the signal-0 answer stands.
func IsAlive(pid int) bool {
	if pid <= 0 || syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	info, ok := parseStat(pid, string(stat))
	return !ok || (info.State != "Z" && info.State != "X")
}
