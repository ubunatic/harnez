// Usage demonstrates the public usage library. It shows controller ownership,
// persisted snapshots, refresh requests, and subscription events.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/usage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cmd := newRootCmd()
	cmd.SetContext(ctx)
	if err := cmd.Execute(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:           "usage",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), refresh)
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "request a demonstration refresh after connecting")
	return cmd
}

func run(ctx context.Context, refreshOnly bool) error {
	opts, err := demoOptions()
	if err != nil {
		return err
	}
	client, err := usage.Open(opts)
	if err != nil {
		return err
	}
	defer client.Close()
	printController(client.ControllerInfo())
	fmt.Printf("\x1b[2mDemo state: %s\x1b[0m\n", opts.StateDir)

	if refreshOnly {
		return refreshAndPrint(ctx, client)
	}
	if err := seedDemoSnapshots(opts.StateDir); err != nil {
		return err
	}
	if err := printSnapshots(ctx, client); err != nil {
		return err
	}
	if err := refreshAndPrint(ctx, client); err != nil {
		return err
	}
	return subscribe(ctx, client)
}

// demoOptions shares only a dedicated per-user directory under the OS temp
// directory. This lets concurrent demo processes attach to each other while
// keeping their snapshots and socket separate from Harnez's real state.
func demoOptions() (usage.Options, error) {
	current, err := user.Current()
	if err != nil {
		return usage.Options{}, fmt.Errorf("find current user for demo temp directory: %w", err)
	}
	base := filepath.Join(os.TempDir(), "harnez-usage-demo-"+current.Uid)
	stateDir := filepath.Join(base, "state")
	runtimeDir := filepath.Join(base, "runtime")
	for _, dir := range []string{base, stateDir, runtimeDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return usage.Options{}, fmt.Errorf("create demo temp directory: %w", err)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return usage.Options{}, fmt.Errorf("secure demo temp directory: %w", err)
		}
	}
	return usage.Options{
		StateDir:      stateDir,
		RuntimeDir:    runtimeDir,
		StartIfAbsent: true,
		Collector:     demoCollector{},
		OnIdleShutdown: func(usage.ControllerInfo) {
			fmt.Printf("\x1b[36m●\x1b[0m controller  \x1b[2midle 60s, shut down\x1b[0m\n")
		},
	}, nil
}

// demoCollector supplies deterministic sample values and never calls a
// provider. Its source remains "demo" through controller persistence.
type demoCollector struct{}

func (demoCollector) CollectUsage(_ context.Context, provider usage.ProviderID) (usage.Snapshot, error) {
	if provider == usage.ProviderAGY {
		return usage.Snapshot{Status: usage.StatusSkipped, Source: usage.SourceDemo, Usage: usage.UsageData{Installed: false}}, nil
	}
	session, weekly := 43.0, 18.0
	if provider == usage.ProviderCodex {
		session, weekly = 72, 33
	}
	return usage.Snapshot{Status: usage.StatusDemo, Source: usage.SourceDemo, ObservedAt: time.Now().UTC(), Usage: usage.UsageData{
		Installed: true,
		Session:   &usage.QuotaWindow{UsedPercent: session},
		Weekly:    &usage.QuotaWindow{UsedPercent: weekly},
	}}, nil
}

func seedDemoSnapshots(stateDir string) error {
	now := time.Now().UTC()
	for _, item := range []struct {
		provider usage.ProviderID
		status   usage.SnapshotStatus
		age      time.Duration
		session  float64
		weekly   float64
	}{
		{usage.ProviderClaude, usage.StatusCached, 4 * time.Minute, 41, 18},
		{usage.ProviderCodex, usage.StatusStale, 2 * time.Hour, 72, 33},
	} {
		snapshot := usage.Snapshot{
			SchemaVersion: usage.SnapshotSchemaVersion,
			ProviderID:    item.provider,
			FetchedAt:     now.Add(-item.age),
			ObservedAt:    now.Add(-item.age),
			Status:        item.status,
			Source:        usage.SourceDemo,
			Usage: usage.UsageData{
				Installed: true,
				Session:   &usage.QuotaWindow{UsedPercent: item.session},
				Weekly:    &usage.QuotaWindow{UsedPercent: item.weekly},
			},
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("encode demo snapshot: %w", err)
		}
		if err := os.WriteFile(filepath.Join(stateDir, string(item.provider)+".json"), data, 0600); err != nil {
			return err
		}
	}
	return nil
}

func printController(info usage.ControllerInfo) {
	if info.State == usage.ControllerStarted {
		fmt.Printf("\x1b[36m●\x1b[0m controller  \x1b[1mstarted\x1b[0m in-process  \x1b[2m(socket %s, protocol v%d, idle 60s)\x1b[0m\n", info.SocketPath, info.ProtocolVersion)
		return
	}
	fmt.Printf("\x1b[36m●\x1b[0m controller  \x1b[1mjoined\x1b[0m pid %d  \x1b[2m(protocol v%d)\x1b[0m\n", info.PID, info.ProtocolVersion)
}

func printSnapshots(ctx context.Context, client usage.Client) error {
	fmt.Printf("\x1b[36m●\x1b[0m snapshot    read from state   \x1b[2m(dedicated temp demo state)\x1b[0m\n\n")
	fmt.Println("  \x1b[1mPROVIDER  STATUS      SOURCE       SESSION           WEEKLY            OBSERVED\x1b[0m")
	for _, provider := range []usage.ProviderID{usage.ProviderClaude, usage.ProviderCodex, usage.ProviderAGY} {
		snapshot, err := client.Snapshot(ctx, provider)
		if err != nil {
			return err
		}
		if snapshot == nil {
			printSkipped(provider)
			continue
		}
		printSnapshot(*snapshot)
	}
	return nil
}

func refreshAndPrint(ctx context.Context, client usage.Client) error {
	fmt.Println("\n\x1b[36m●\x1b[0m refresh     claude, codex …")
	snapshots, err := client.Refresh(ctx, usage.ProviderClaude, usage.ProviderCodex)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		printSnapshot(snapshot)
	}
	return nil
}

func subscribe(ctx context.Context, client usage.Client) error {
	fmt.Println("\n\x1b[36m●\x1b[0m subscribe   waiting for updates  \x1b[2m(ctrl+c to quit)\x1b[0m")
	events, err := client.Subscribe(ctx, usage.ProviderClaude)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			fmt.Printf("\x1b[2m%s\x1b[0m  %-9s \x1b[32m%-6s\x1b[0m  session %s", time.Now().Format("15:04:05"), event.Snapshot.ProviderID, event.Snapshot.Status, quotaBar(event.Snapshot.Usage.Session))
			if event.RefreshingPID != 0 {
				fmt.Printf("  \x1b[2mpushed by refresh from pid %d\x1b[0m", event.RefreshingPID)
			}
			fmt.Println()
		}
	}
}

func printSkipped(provider usage.ProviderID) {
	fmt.Printf("  %-9s \x1b[2mskipped  -            -                 -                 not installed\x1b[0m\n", provider)
}

func printSnapshot(snapshot usage.Snapshot) {
	status := coloredStatus(snapshot.Status)
	source := string(snapshot.Source)
	if source == "" {
		source = "-"
	}
	if !snapshot.Usage.Installed {
		printSkipped(snapshot.ProviderID)
		return
	}
	fmt.Printf("  %-9s %s %-12s %s  %3.0f%%  %s  %3.0f%%  %s", snapshot.ProviderID, status, source, quotaBar(snapshot.Usage.Session), used(snapshot.Usage.Session), quotaBar(snapshot.Usage.Weekly), used(snapshot.Usage.Weekly), relativeAge(snapshot.ObservedAt))
	if snapshot.Error != nil {
		fmt.Printf("  \x1b[31m%s\x1b[0m", snapshot.Error.Category)
		if snapshot.Status == usage.StatusStale {
			fmt.Print("  \x1b[2m(kept last good data)\x1b[0m")
		}
	}
	if snapshot.Status == usage.StatusThrottled && snapshot.Error != nil && snapshot.Error.RetryAfter != nil {
		remaining := time.Until(*snapshot.Error.RetryAfter).Round(time.Second)
		fmt.Printf("  \x1b[2mretry after %s (min interval 30s)\x1b[0m", remaining)
	}
	fmt.Println()
}

func coloredStatus(status usage.SnapshotStatus) string {
	color := "\x1b[2m"
	switch status {
	case usage.StatusLive:
		color = "\x1b[32m"
	case usage.StatusCached, usage.StatusStale:
		color = "\x1b[33m"
	case usage.StatusThrottled:
		color = "\x1b[35m"
	case usage.StatusError:
		color = "\x1b[31m"
	case usage.StatusDemo:
		color = "\x1b[35m"
	}
	return color + fmt.Sprintf("%-11s", status) + "\x1b[0m"
}

func quotaBar(window *usage.QuotaWindow) string {
	if window == nil {
		return "-"
	}
	filled := int(window.UsedPercent / 10)
	if filled > 10 {
		filled = 10
	}
	return "\x1b[32m" + strings.Repeat("█", filled) + "\x1b[0m\x1b[2m" + strings.Repeat("░", 10-filled) + "\x1b[0m"
}

func used(window *usage.QuotaWindow) float64 {
	if window == nil {
		return 0
	}
	return window.UsedPercent
}

func relativeAge(at time.Time) string {
	if at.IsZero() {
		return "not installed"
	}
	if time.Since(at) < time.Minute {
		return "now"
	}
	age := time.Since(at).Round(time.Minute)
	if age < time.Hour {
		return strconv.Itoa(int(age/time.Minute)) + "m ago"
	}
	return strconv.Itoa(int(age/time.Hour)) + "h ago"
}
