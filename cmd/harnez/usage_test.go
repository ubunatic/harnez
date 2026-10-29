package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagecli"
)

func TestUsageStartModes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "normal"},
		{[]string{"--watch"}, "minimal"},
		{[]string{"--watch", "--minimal"}, "minimal"},
		{[]string{"--watch", "--compact"}, "compact"},
		{[]string{"--watch", "--table"}, "normal"},
		{[]string{"--watch", "--normal"}, "normal"},
		{[]string{"--table"}, "normal"},
		{[]string{"--normal"}, "normal"},
	} {
		t.Run(fmt.Sprint(tc.args), func(t *testing.T) {
			cmd := newUsageCmd()
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatal(err)
			}
			flag := func(name string) bool {
				value, err := cmd.Flags().GetBool(name)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			mode, _, err := usagecli.UsageStartMode(flag("watch"), flag("table"), flag("compact"), flag("minimal"))
			if err != nil || mode != tc.want {
				t.Fatalf("start mode = %q, %v; want %q", mode, err, tc.want)
			}
			if !cmd.Flags().Lookup("normal").Hidden || cmd.Flags().Lookup("table").Hidden {
				t.Fatal("only the legacy alias should be hidden")
			}
		})
	}
}

func TestUsageOneShotDefaultsToTable(t *testing.T) {
	seedUsageStore(t)
	cmd := newUsageCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--offline"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Claude")) || bytes.Contains(buf.Bytes(), []byte("╭")) {
		t.Fatalf("expected plain usage table, got %s", buf.String())
	}
}

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
	return seedUsageStoreAgents(t, usage.AgentUsage{AgentID: "claude", Name: "Claude Code", Installed: true})
}

func seedUsageStoreAgents(t *testing.T, agents ...usage.AgentUsage) string {
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
	if err := usage.PersistUsageSummary(ctx, home, "", usage.UsageSummary{Timestamp: time.Now(), Agents: agents}); err != nil {
		t.Fatal(err)
	}
	return usage.UsageStorePath()
}

func TestUsageAgentFilterAppliesToTableViews(t *testing.T) {
	reset := time.Now().Add(48 * time.Hour)
	week := func() *usage.QuotaWindow {
		return &usage.QuotaWindow{Name: "Weekly", UsedPercent: 10, RemainingPercent: 90, ResetAt: &reset}
	}
	seedUsageStoreAgents(t,
		usage.AgentUsage{AgentID: "claude", Name: "Claude Code", Installed: true, Authenticated: true, Weekly: week()},
		usage.AgentUsage{AgentID: "codex", Name: "Codex CLI", Installed: true, Authenticated: true, Weekly: week()},
	)
	for _, view := range [][]string{{}, {"--compact"}, {"--minimal"}} {
		args := append([]string{"usage", "--offline", "--agent", "codex"}, view...)
		t.Run(fmt.Sprint(view), func(t *testing.T) {
			root := newRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if !strings.Contains(out, "Codex") {
				t.Errorf("output missing Codex:\n%s", out)
			}
			if strings.Contains(out, "Claude Code") {
				t.Errorf("--agent codex output still shows Claude:\n%s", out)
			}
		})
	}
}

func TestUsageSubcommandsSilenceUsageOnRuntimeErrors(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Runnable() && !c.SilenceUsage {
			t.Errorf("%q does not set SilenceUsage", c.CommandPath())
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(newUsageCmd())
}

func TestUsageEmptyStoreFailsWithActionableError(t *testing.T) {
	old := usagecli.EnsureLocalServerFunc
	usagecli.EnsureLocalServerFunc = func(context.Context, server.Config) usagecli.ServerCheckResult {
		return usagecli.ServerCheckResult{}
	}
	t.Cleanup(func() { usagecli.EnsureLocalServerFunc = old })
	for _, store := range []string{"initialized", "missing"} {
		for _, args := range [][]string{
			{"usage"},
			{"usage", "--compact"},
			{"usage", "--agent", "codex"},
			{"usage", "--json"},
			{"usage", "history", "record"},
		} {
			t.Run(store+"/"+fmt.Sprint(args), func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				t.Setenv("XDG_DATA_HOME", home+"/data")
				t.Setenv("XDG_STATE_HOME", home+"/state")
				t.Setenv("XDG_CACHE_HOME", home+"/cache")
				if store == "initialized" {
					if err := usage.InitializeUsageStore(context.Background(), home, ""); err != nil {
						t.Fatal(err)
					}
				}
				root := &cobra.Command{Use: "harnez"}
				root.AddCommand(newUsageCmd())
				out := new(bytes.Buffer)
				root.SetOut(out)
				root.SetErr(out)
				root.SetArgs(args)
				err := root.Execute()
				if err == nil {
					t.Fatalf("expected an error; output:\n%s", out)
				}
				if got := exitCodeFromRunError(err); got != 1 {
					t.Fatalf("exit code = %d, want 1", got)
				}
				if !errors.Is(err, usage.ErrNoCollectorSnapshots) {
					t.Fatalf("error %v does not wrap ErrNoCollectorSnapshots", err)
				}
				text := out.String() + err.Error()
				for _, want := range []string{"usage store has no collector snapshots yet", "harnez server install"} {
					if !strings.Contains(text, want) {
						t.Fatalf("output missing %q:\n%s", want, text)
					}
				}
				for _, bad := range []string{"Usage:", "Flags:"} {
					if strings.Contains(text, bad) {
						t.Fatalf("output contains %q:\n%s", bad, text)
					}
				}
			})
		}
	}
}
