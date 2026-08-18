package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/tools"
)

func main() {
	stateFile := flag.String("state-file", tools.DefaultModifierStatePath, "path to world-readable modifier state file")
	interval := flag.Duration("interval", 10*time.Millisecond, "polling interval for modifier checks")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	deps := tools.DefaultDependencies(os.Stdin, os.Stdout)
	opts := tools.DaemonOptions{
		StateFile: *stateFile,
		Interval:  *interval,
	}

	if err := tools.RunModifierDaemon(ctx, deps, opts); err != nil {
		fmt.Fprintf(os.Stderr, "harnez-modifierd error: %v\n", err)
		os.Exit(1)
	}
}
