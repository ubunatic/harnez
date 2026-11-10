package procs

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// IsAlive checks if a process is still alive. A zombie process is considered dead.
func IsAlive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return !os.IsNotExist(err)
	}
	statStr := string(statData)
	idx := strings.LastIndexByte(statStr, ')')
	if idx != -1 && idx+2 < len(statStr) {
		state := statStr[idx+2 : idx+3]
		return state != "Z" && state != "X"
	}
	return true
}
