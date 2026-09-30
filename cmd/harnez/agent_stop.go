package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
)

func sessionRoots(x *subagent.Session) []procs.TreeRoot {
	roots := []procs.TreeRoot{{PID: x.ProcessPID, Starttime: x.ProcessStarttime}}
	if x.ProcessPID <= 0 && x.Status == "running" && x.CallerPID > 0 {
		roots[0].PID = x.CallerPID
	}
	if x.ProviderPID > 0 {
		roots = append(roots, procs.TreeRoot{PID: x.ProviderPID, Starttime: x.ProviderStarttime, Group: true})
	}
	// A PTY provider owns a session/group too.
	if x.HarnessType == "interactive" {
		roots[0].Group = true
	}
	return roots
}

func sessionWriters(x *subagent.Session) ([]procs.ProcInfo, error) {
	return procs.TreeMembers(sessionRoots(x), make(map[int]procs.ProcInfo), nil)
}

func observeSessionProcess(cmd *cobra.Command, store *subagent.FileSessionStore, id string, local *subagent.Session) func() {
	original := agentCommandContext(cmd)
	cmd.SetContext(subagent.WithProcessObserver(original, func(pid int, start uint64) error {
		current, err := store.Get(id)
		if err != nil {
			return err
		}
		current.ProviderPID, current.ProviderStarttime = pid, start
		if local != nil {
			local.ProviderPID, local.ProviderStarttime = pid, start
		}
		return store.Save(current)
	}))
	return func() { cmd.SetContext(original) }
}

func stopSession(cmd *cobra.Command, store *subagent.FileSessionStore, x *subagent.Session) error {
	grace, killWait, poll, err := subagent.StopBounds()
	if err != nil {
		return err
	}
	if x.HarnessType == "interactive" {
		if x.Status != "active" {
			return fmt.Errorf("session %q is not active", x.Name)
		}
		if err := subagent.SendControl(cmd.Context(), x.ControlSocket, "stop", ""); err != nil {
			return reportSessionStop(cmd, store, x, nil, err)
		}
	}
	if x.Status == "running" && sessionRoots(x)[0].PID <= 0 && x.ProviderPID <= 0 {
		return reportSessionStop(cmd, store, x, nil, fmt.Errorf("running session has no recorded process identity; cannot confirm exit"))
	}
	// PID-less legacy/test drivers still own their provider-side lifecycle.
	if x.ProcessPID <= 0 && x.ProviderPID <= 0 && x.HarnessType != "interactive" {
		err = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}, x.WorkingDir).Stop(cmd.Context(), x.ProviderID())
		if err != nil {
			return err
		}
	}
	live, err := procs.StopTree(agentCommandContext(cmd), procs.TreeOptions{Roots: sessionRoots(x), Grace: grace, KillWait: killWait, Poll: poll})
	return reportSessionStop(cmd, store, x, live, err)
}

func reportSessionStop(cmd *cobra.Command, store *subagent.FileSessionStore, x *subagent.Session, live []procs.ProcInfo, stopErr error) error {
	if len(live) > 0 || stopErr != nil {
		pid := x.ProcessPID
		if len(live) > 0 {
			pid = live[0].PID
		}
		if stopErr != nil {
			return fmt.Errorf("stop requested: %d still exiting; run `harnez agent wait --name %s`: %w", pid, x.Name, stopErr)
		}
		return fmt.Errorf("stop requested: %d still exiting; run `harnez agent wait --name %s`", pid, x.Name)
	}
	x.Status = "stopped"
	x.ProcessPID, x.ProcessStarttime, x.ProviderPID, x.ProviderStarttime = 0, 0, 0, 0
	x.LastActiveAt = time.Now()
	if err := store.Save(x); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "stopped: %s, no process left, safe to resume/delete\n", x.Name)
	fmt.Fprintln(cmd.OutOrStdout(), "The host's native background task/job may take a few seconds to report exit; this is expected.")
	return nil
}
