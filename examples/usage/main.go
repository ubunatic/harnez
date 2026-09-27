// Usage demonstrates the public usage library. It is intentionally separate
// from `harnez usage --compact`: this program shows controller ownership,
// snapshots, refreshes, and subscription events for application authors.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ubunatic.com/harnez/usage"
)

var refresh = flag.Bool("refresh", false, "request a demonstration refresh after connecting")

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := usage.Open(usage.Options{
		StartIfAbsent: true,
		Collector:     demoCollector{}, // Demo only: no provider credentials or network calls.
		OnIdleShutdown: func(info usage.ControllerInfo) {
			fmt.Printf("\x1b[36m●\x1b[0m controller  \x1b[2midle 60s, shut down\x1b[0m\n")
		},
	})
	if err != nil {
		fatal(err)
	}
	defer client.Close()
	printController(client.ControllerInfo())

	if *refresh {
		refreshAndPrint(ctx, client)
		return
	}
	printSnapshots(ctx, client, "snapshot    read from state")
	refreshAndPrint(ctx, client)
	subscribe(ctx, client)
}

// demoCollector is deliberately a stub. It lets this standalone example show
// refresh and subscription without using credentials or a live provider.
type demoCollector struct{}

func (demoCollector) CollectUsage(_ context.Context, provider usage.ProviderID) (usage.Snapshot, error) {
	if provider == usage.ProviderAGY {
		return usage.Snapshot{Usage: usage.UsageData{Installed: false}}, nil
	}
	return usage.Snapshot{Usage: usage.UsageData{Installed: true, Session: &usage.QuotaWindow{UsedPercent: 43}, Weekly: &usage.QuotaWindow{UsedPercent: 18}}}, nil
}

func printController(info usage.ControllerInfo) {
	if info.State == usage.ControllerStarted {
		fmt.Printf("\x1b[36m●\x1b[0m controller  \x1b[1mstarted\x1b[0m in-process  \x1b[2m(socket %s, protocol v%d, idle 60s)\x1b[0m\n", info.SocketPath, info.ProtocolVersion)
		return
	}
	fmt.Printf("\x1b[36m●\x1b[0m controller  \x1b[1mjoined\x1b[0m pid %d  \x1b[2m(protocol v%d)\x1b[0m\n", info.PID, info.ProtocolVersion)
}

func printSnapshots(ctx context.Context, client usage.Client, heading string) {
	fmt.Printf("\x1b[36m●\x1b[0m %s   \x1b[2m(%s)\x1b[0m\n\n", heading, usage.StateDir(""))
	fmt.Println("  \x1b[1mPROVIDER  STATUS      SOURCE       SESSION           WEEKLY            OBSERVED\x1b[0m")
	for _, provider := range providers() {
		snapshot, err := client.Snapshot(ctx, provider)
		if err != nil {
			fatal(err)
		}
		if snapshot == nil {
			snapshot = &usage.Snapshot{ProviderID: provider, Status: usage.StatusSkipped, Usage: usage.UsageData{Installed: false}}
		}
		printSnapshot(*snapshot)
	}
}

func refreshAndPrint(ctx context.Context, client usage.Client) {
	fmt.Println("\n\x1b[36m●\x1b[0m refresh     claude, codex …")
	snapshots, err := client.Refresh(ctx, usage.ProviderClaude, usage.ProviderCodex)
	if err != nil {
		fatal(err)
	}
	for _, snapshot := range snapshots {
		printSnapshot(snapshot)
	}
}

func subscribe(ctx context.Context, client usage.Client) {
	fmt.Println("\n\x1b[36m●\x1b[0m subscribe   waiting for updates  \x1b[2m(ctrl+c to quit)\x1b[0m")
	events, err := client.Subscribe(ctx, usage.ProviderClaude)
	if err != nil {
		fatal(err)
	}
	for event := range events {
		fmt.Printf("\x1b[2m%s\x1b[0m  %-9s \x1b[32m%-6s\x1b[0m", time.Now().Format("15:04:05"), event.Snapshot.ProviderID, event.Snapshot.Status)
		if event.RefreshingPID != 0 {
			fmt.Printf("  \x1b[2mpushed by refresh from pid %d\x1b[0m", event.RefreshingPID)
		}
		fmt.Println()
	}
}

func providers() []usage.ProviderID {
	return []usage.ProviderID{usage.ProviderClaude, usage.ProviderCodex, usage.ProviderAGY}
}

func printSnapshot(snapshot usage.Snapshot) {
	status := string(snapshot.Status)
	source := string(snapshot.Source)
	if source == "" {
		source = "-"
	}
	fmt.Printf("  %-9s %-11s %-12s %-17s %-17s %s\n", snapshot.ProviderID, status, source, quota(snapshot.Usage.Session), quota(snapshot.Usage.Weekly), observed(snapshot.ObservedAt))
}

func quota(window *usage.QuotaWindow) string {
	if window == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", window.UsedPercent)
}

func observed(at time.Time) string {
	if at.IsZero() {
		return "not installed"
	}
	return at.Local().Format("15:04:05")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
