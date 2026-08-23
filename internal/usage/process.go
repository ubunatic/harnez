package usage

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// AgentProcessCount records the number of running processes detected for each agent.
type AgentProcessCount struct {
	Claude int
	AGY    int
	Codex  int
}

// Total returns the sum of all running agent processes.
func (c AgentProcessCount) Total() int {
	return c.Claude + c.AGY + c.Codex
}

// CountRunningAgentProcesses counts currently active processes for claude, agy, and codex.
// It inspects /proc on Linux, falling back to ps output if /proc is inaccessible.
func CountRunningAgentProcesses() AgentProcessCount {
	return countRunningAgentProcessesInProc("/proc")
}

func countRunningAgentProcessesInProc(procPath string) AgentProcessCount {
	counts, err := countProcessesFromProc(procPath)
	if err == nil {
		return counts
	}
	return countProcessesFromPS()
}

// isAgentName matches a process name/command to one of the agent identifiers ("claude", "agy", "codex").
func isAgentName(comm string) string {
	comm = strings.TrimSpace(comm)
	base := filepath.Base(comm)
	switch base {
	case "claude":
		return "claude"
	case "agy":
		return "agy"
	case "codex":
		return "codex"
	default:
		return ""
	}
}

// countProcessesFromProc iterates numeric directories in procPath to read comm and cmdline.
func countProcessesFromProc(procPath string) (AgentProcessCount, error) {
	entries, err := os.ReadDir(procPath)
	if err != nil {
		return AgentProcessCount{}, err
	}

	var counts AgentProcessCount
	foundAny := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Process directories in /proc are numeric PIDs
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		foundAny = true

		pidDir := filepath.Join(procPath, entry.Name())
		commBytes, err := os.ReadFile(filepath.Join(pidDir, "comm"))
		if err == nil {
			comm := strings.TrimSpace(string(commBytes))
			switch isAgentName(comm) {
			case "claude":
				counts.Claude++
				continue
			case "agy":
				counts.AGY++
				continue
			case "codex":
				counts.Codex++
				continue
			}
		}

		// If comm didn't match directly (e.g. node / python wrapper or script runner), inspect cmdline
		cmdlineBytes, err := os.ReadFile(filepath.Join(pidDir, "cmdline"))
		if err == nil && len(cmdlineBytes) > 0 {
			// cmdline arguments are null-byte separated
			args := bytes.Split(cmdlineBytes, []byte{0})
			if len(args) > 0 && len(args[0]) > 0 {
				arg0 := string(args[0])
				switch isAgentName(arg0) {
				case "claude":
					counts.Claude++
				case "agy":
					counts.AGY++
				case "codex":
					counts.Codex++
				}
			}
		}
	}

	if !foundAny && procPath == "/proc" {
		return counts, fmt.Errorf("no processes found in /proc")
	}

	return counts, nil
}

// countProcessesFromPS executes `ps -eo comm=` as a fallback when /proc is unavailable.
func countProcessesFromPS() AgentProcessCount {
	out, err := exec.Command("ps", "-eo", "comm=").Output()
	if err != nil {
		return AgentProcessCount{}
	}

	var counts AgentProcessCount
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		comm := strings.TrimSpace(line)
		switch isAgentName(comm) {
		case "claude":
			counts.Claude++
		case "agy":
			counts.AGY++
		case "codex":
			counts.Codex++
		}
	}
	return counts
}
