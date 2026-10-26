package main

import (
	"os"
	"strconv"

	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
)

// interactiveCodexHost identifies a provider-confirmed interactive thread,
// excluding workers belonging to a live Harnez launcher. Environment alone is
// insufficient: long-lived app servers can inherit an unrelated worker's role.
func interactiveCodexHost() string {
	id := os.Getenv("CODEX_THREAD_ID")
	if id == "" || !subagent.IsInteractiveCodexThread(id) {
		return ""
	}
	if launcher := os.Getenv(agentLauncherEnv); launcher != "" {
		pid, err := strconv.Atoi(launcher)
		if err != nil || pid <= 1 || launcherInAncestry(pid) {
			return ""
		}
	}
	// Preserve restrictions for interactive workers launched by older binaries
	// which did not export launcher provenance.
	if name := os.Getenv(agentSessionEnv); name != "" {
		store, err := subagent.NewSessionStore(subagent.DefaultStoreDir())
		if err != nil {
			return ""
		}
		workers, err := store.List("", true)
		if err != nil {
			return ""
		}
		for _, worker := range workers {
			if (worker.Name == name || worker.ID == name) && worker.ProviderID() == id {
				return ""
			}
		}
	}
	return id
}

func launcherInAncestry(launcher int) bool {
	processes, err := (procs.LinuxProcReader{}).List()
	if err != nil {
		return true // Cannot disprove worker provenance.
	}
	parents := make(map[int]int, len(processes))
	for _, process := range processes {
		parents[process.PID] = process.PPID
	}
	seen := make(map[int]bool)
	for pid := os.Getpid(); pid > 1; pid = parents[pid] {
		if pid == launcher || seen[pid] {
			return true
		}
		seen[pid] = true
		if _, ok := parents[pid]; !ok {
			return true
		}
	}
	return false
}

func callerAgentRole() string {
	if interactiveCodexHost() != "" {
		return ""
	}
	return os.Getenv(agentRoleEnv)
}
