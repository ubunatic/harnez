package main

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

func TestUsageProjectFlag_CobraRegistered(t *testing.T) {
	root := newRootCmd()
	if usageCmd := root.Commands(); usageCmd != nil {
		for _, c := range usageCmd {
			if c.Name() == "usage" {
				if c.Flags().Lookup("project") == nil {
					t.Errorf("missing --project flag on usage command")
				}
				if c.Flags().Lookup("cwd") == nil {
					t.Errorf("missing --cwd flag on usage command")
				}
				if c.Flags().Lookup("loom") == nil {
					t.Errorf("missing --loom flag on usage command")
				}
				if c.Flags().Lookup("splash") == nil {
					t.Errorf("missing --splash flag on usage command")
				}
				if c.Flags().Lookup("minimal") == nil {
					t.Errorf("missing --minimal flag on usage command")
				}
				if c.Flags().Lookup("normal") == nil {
					t.Errorf("missing --normal flag on usage command")
				}
				if c.Flags().Lookup("compact") == nil {
					t.Errorf("missing --compact flag on usage command")
				}
			}
			if c.Name() == "assess" {
				if c.Flags().Lookup("tokens") == nil {
					t.Errorf("missing --tokens flag on assess command")
				}
				if c.Flags().Lookup("history") == nil {
					t.Errorf("missing --history flag on assess command")
				}
				if c.Flags().Lookup("ramp") == nil {
					t.Errorf("missing --ramp flag on assess command")
				}
			}
		}
	}
}

func TestUsageCmd_LoomFlagExecution(t *testing.T) {
	skipShort(t, "first usage render burst-samples CPU load for ~2s")
	seedUsageStore(t)
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"usage", "--loom", "--offline"})

	if err := root.Execute(); err != nil {
		t.Fatalf("harnez usage --loom failed: %v", err)
	}

	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Agentic usage")) {
		t.Errorf("expected output to contain 'Agentic usage', got:\n%s", out)
	}
}

func TestUsageCmd_MinimalFlagExecution(t *testing.T) {
	skipShort(t, "first usage render burst-samples CPU load for ~2s")
	seedUsageStore(t)
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"usage", "--minimal", "--offline"})

	if err := root.Execute(); err != nil {
		t.Fatalf("harnez usage --minimal failed: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("Agentic usage")) || bytes.Contains(buf.Bytes(), []byte("hidden")) {
		t.Fatalf("minimal output should omit the title and hidden-panel summary:\n%s", buf.String())
	}
}

func TestUsageCmdJSONReadsChmodReadOnlyStore(t *testing.T) {
	path := seedUsageStore(t)
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	root := newRootCmd()
	root.SetArgs([]string{"usage", "--json", "--offline"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func seedUsageStore(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", home+"/data")
	t.Setenv("XDG_STATE_HOME", home+"/state")
	t.Setenv("XDG_CACHE_HOME", home+"/cache")
	ctx := context.Background()
	if err := usage.InitializeUsageStore(ctx, home, ""); err != nil {
		t.Fatal(err)
	}
	if err := usage.PersistUsageSummary(ctx, home, "", usage.UsageSummary{Timestamp: time.Now(), Agents: []usage.AgentUsage{{AgentID: "claude", Name: "Claude Code", Installed: true}}}); err != nil {
		t.Fatal(err)
	}
	return usage.UsageStorePath()
}
