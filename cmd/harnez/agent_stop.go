package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
)

func sessionRoots(x *subagent.Session) []procs.TreeRoot {
	roots := []procs.TreeRoot{{PID: x.ProcessPID, Starttime: x.ProcessStarttime}}
	needsCallerRoot := x.Status == "running" || (x.HarnessType == "interactive" && x.Status == "active")
	if x.ProcessPID <= 0 && needsCallerRoot && x.CallerPID > 0 {
		roots[0].PID = x.CallerPID
		roots[0].Starttime = x.CallerStarttime
	}
	if x.ProviderPID > 0 {
		roots = append(roots, procs.TreeRoot{PID: x.ProviderPID, Starttime: x.ProviderStarttime, Group: true})
	}
	// A detached worker stays a writer until it exits, even after it cleared
	// ProcessPID and recorded its terminal status. Keep roots with unknown
	// starttimes so stop validation rejects the whole session before signalling.
	if x.WorkerPID > 0 && x.WorkerPID != x.ProcessPID {
		roots = append(roots, procs.TreeRoot{PID: x.WorkerPID, Starttime: x.WorkerStarttime})
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

// observeSessionProcess returns ctx carrying a process observer that records the
// provider PID on session id. It derives a new context instead of calling
// cmd.SetContext, which would race goroutines reading cmd.Context().
func observeSessionProcess(ctx context.Context, store *subagent.FileSessionStore, id string, local *subagent.Session) context.Context {
	return subagent.WithProcessObserver(ctx, func(pid int, start uint64) error {
		current, err := store.Get(id)
		if err != nil {
			return err
		}
		current.ProviderPID, current.ProviderStarttime = pid, start
		if local != nil {
			local.ProviderPID, local.ProviderStarttime = pid, start
		}
		return store.Save(current)
	})
}

func stopSession(cmd *cobra.Command, store *subagent.FileSessionStore, x *subagent.Session) error {
	return stopSessionWithReader(cmd, store, x, nil)
}

func stopSessionWithReader(cmd *cobra.Command, store *subagent.FileSessionStore, x *subagent.Session, reader procs.ProcReader) error {
	grace, killWait, poll, err := subagent.StopBounds()
	if err != nil {
		return err
	}
	roots := sessionRoots(x)
	if x.HarnessType == "interactive" && x.Status == "active" && roots[0].PID <= 0 {
		return fmt.Errorf("session %q cannot be stopped safely: active interactive session has no recorded process identity", x.Name)
	}
	if err := procs.ValidateTreeRoots(roots); err != nil {
		return fmt.Errorf("session %q cannot be stopped safely: %w", x.Name, err)
	}
	if x.HarnessType == "interactive" {
		if x.Status != "active" {
			return fmt.Errorf("session %q is not active", x.Name)
		}
		if err := procs.ValidateTreeRootIdentities(roots[:1], reader); err != nil {
			return fmt.Errorf("session %q cannot be stopped safely: %w", x.Name, err)
		}
		if err := subagent.SendControl(cmd.Context(), x.ControlSocket, "stop", ""); err != nil {
			return reportSessionStop(cmd, store, x, nil, err)
		}
	}
	if x.Status == "running" && roots[0].PID <= 0 && x.ProviderPID <= 0 {
		return reportSessionStop(cmd, store, x, nil, fmt.Errorf("running session has no recorded process identity; cannot confirm exit"))
	}
	// PID-less legacy/test drivers still own their provider-side lifecycle.
	if x.ProcessPID <= 0 && x.ProviderPID <= 0 && x.HarnessType != "interactive" {
		err = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}, x.WorkingDir).Stop(cmd.Context(), x.ProviderID())
		if err != nil {
			return err
		}
	}
	live, err := procs.StopTree(agentCommandContext(cmd), procs.TreeOptions{Roots: roots, Grace: grace, KillWait: killWait, Poll: poll})
	return reportSessionStop(cmd, store, x, live, err)
}

func reportSessionStop(cmd *cobra.Command, store *subagent.FileSessionStore, x *subagent.Session, live []procs.ProcInfo, stopErr error) error {
	if len(live) > 0 || stopErr != nil {
		pid := x.ProcessPID
		if len(live) > 0 {
			pid = live[0].PID
		}
		if stopErr != nil {
			return fmt.Errorf("stop requested: %d still exiting; run `harnez agent wait %s`: %w", pid, x.Name, stopErr)
		}
		return fmt.Errorf("stop requested: %d still exiting; run `harnez agent wait %s`", pid, x.Name)
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
