package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/quota1"
)

type quotaCleanResult struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type cleanReport struct {
	Procs []procs.Action    `json:"procs,omitempty"`
	Q1    *quotaCleanResult `json:"q1,omitempty"`
}

func newCleanCmd() *cobra.Command {
	var dir string
	var kill bool
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "clean [procs] [q1]",
		Short: "Remove stale process records and release verified quota-1 state",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(cmd.OutOrStdout(), dir, args, kill, jsonOutput)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "directory used to locate Quota-1 state")
	cmd.Flags().BoolVar(&kill, "kill", false, "act on eligible process groups and remove verified state")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "write cleanup results as JSON")
	return cmd
}

func runClean(out io.Writer, dir string, targets []string, kill, jsonOutput bool) error {
	selected := map[string]bool{}
	if len(targets) == 0 {
		selected["procs"] = true
		selected["q1"] = true
	} else {
		for _, target := range targets {
			if target != "procs" && target != "q1" {
				return fmt.Errorf("clean: unknown target %q (supported: procs, q1)", target)
			}
			selected[target] = true
		}
	}
	var report cleanReport
	if selected["procs"] {
		recordDir, err := procs.RecordDir()
		if err != nil {
			return err
		}
		actions, err := procs.CleanProcs(procs.CleanOptions{RecordDir: recordDir, Kill: kill})
		if err != nil {
			return err
		}
		report.Procs = actions
	}
	if selected["q1"] {
		result, err := cleanQuota1(dir, kill)
		if err != nil {
			return err
		}
		report.Q1 = &result
	}
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	for _, action := range report.Procs {
		fmt.Fprintf(out, "procs %d: %s", action.PGID, action.Status)
		if action.Reason != "" {
			fmt.Fprintf(out, " (%s)", action.Reason)
		}
		fmt.Fprintln(out)
	}
	if selected["procs"] && len(report.Procs) == 0 {
		fmt.Fprintln(out, "procs: no recorded process groups")
	}
	if report.Q1 != nil {
		fmt.Fprintf(out, "q1: %s", report.Q1.Status)
		if report.Q1.Reason != "" {
			fmt.Fprintf(out, " (%s)", report.Q1.Reason)
		}
		fmt.Fprintln(out)
	}
	return nil
}

func cleanQuota1(dir string, kill bool) (quotaCleanResult, error) {
	return cleanQuota1WithGroupCheck(dir, kill, func(pgid int) (bool, error) {
		return procs.GroupGone(pgid, procs.LinuxProcReader{}, nil)
	})
}

func cleanQuota1WithGroupCheck(dir string, kill bool, groupGone func(int) (bool, error)) (quotaCleanResult, error) {
	stateFile, _, err := quota1.ResolveStateFile(dir)
	if err != nil {
		return quotaCleanResult{}, err
	}
	record, err := quota1.ReadRunRecord(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return quotaCleanResult{Status: "unchanged", Reason: "no quota-1 state"}, nil
		}
		return quotaCleanResult{}, fmt.Errorf("read quota-1 state: %w", err)
	}
	if record.Exit != nil {
		return quotaCleanResult{Status: "kept", Reason: "run finished normally; modify source to rerun"}, nil
	}
	if record.PGID <= 0 {
		return quotaCleanResult{Status: "kept", Reason: "cannot verify process group; no recorded pgid"}, nil
	}
	gone, err := groupGone(record.PGID)
	if err != nil {
		return quotaCleanResult{}, fmt.Errorf("verify quota-1 process group: %w", err)
	}
	if !gone {
		return quotaCleanResult{Status: "kept", Reason: fmt.Sprintf("process group %d is still alive", record.PGID)}, nil
	}
	if !kill {
		return quotaCleanResult{Status: "would-release", Reason: "incomplete run has no live process group"}, nil
	}
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		return quotaCleanResult{}, fmt.Errorf("remove quota-1 state: %w", err)
	}
	return quotaCleanResult{Status: "released", Reason: "incomplete run has no live process group"}, nil
}
