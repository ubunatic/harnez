package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// NeusIndexer is a server service that serially indexes registered repositories.
// Ticker and RunCommand may be injected to exercise scheduling without sleeps.
type NeusIndexer struct {
	Config     NeusIndexConfig
	Output     io.Writer
	Ticker     func(time.Duration) (<-chan time.Time, func())
	RunCommand func(context.Context, string, ...string) error
	mu         sync.Mutex
}

// Run waits for ticks and joins each child before accepting another cycle.
func (s *NeusIndexer) Run(ctx context.Context) error {
	if err := s.Config.Validate(); err != nil {
		return err
	}
	if !s.Config.Enabled {
		<-ctx.Done()
		return nil
	}
	interval, _ := time.ParseDuration(s.Config.Interval)
	newTicker := s.Ticker
	if newTicker == nil {
		newTicker = func(d time.Duration) (<-chan time.Time, func()) { t := time.NewTicker(d); return t.C, t.Stop }
	}
	ticks, stop := newTicker(interval)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ticks:
			if !ok {
				return nil
			}
			if err := s.cycle(ctx); err != nil && ctx.Err() == nil {
				out := s.Output
				if out == nil {
					out = os.Stderr
				}
				fmt.Fprintf(out, "harnez server: neus indexing: %v\n", err)
			}
		}
	}
}

func (s *NeusIndexer) cycle(ctx context.Context) error {
	if !s.mu.TryLock() {
		return nil
	}
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	run := s.RunCommand
	if run == nil {
		run = func(ctx context.Context, name string, args ...string) error {
			cmd := exec.CommandContext(ctx, name, args...)
			// No unbounded output buffering and no inherited terminal input.
			cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
			return cmd.Run()
		}
	}
	timeout, _ := time.ParseDuration(s.Config.Timeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	active, err := neusServiceActive(ctx, run, s.Config.ServiceName)
	if err != nil {
		return err
	}
	if active {
		return nil
	}
	roots, err := canonicalIndexRoots(s.Config.Repos)
	if err != nil {
		return err
	}
	if len(roots) == 0 {
		return nil
	}
	argv := append(append([]string{}, s.Config.Command[1:]...), roots...)
	if err := run(ctx, s.Config.Command[0], argv...); err != nil {
		return fmt.Errorf("run %s: %w", s.Config.Command[0], err)
	}
	return nil
}

func neusServiceActive(ctx context.Context, run func(context.Context, string, ...string) error, unit string) (bool, error) {
	err := run(ctx, "systemctl", "--user", "is-active", "--quiet", unit)
	if err == nil {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	// Without systemctl there cannot be a systemd user indexer. Exit 3 means
	// inactive; exit 4 means unknown unit (neus service packaging is pending).
	if errors.Is(err, exec.ErrNotFound) {
		return false, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && (exit.ExitCode() == 3 || exit.ExitCode() == 4) {
		return false, nil
	}
	return false, fmt.Errorf("check %s: %w", unit, err)
}

func canonicalIndexRoots(paths []string) ([]string, error) {
	var roots []string
	seen := make(map[string]bool)
	for _, path := range paths {
		if path == "" {
			return nil, fmt.Errorf("empty repository path")
		}
		root, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return nil, fmt.Errorf("repository %q: %w", path, err)
		}
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("repository %q is not a directory", path)
		}
		if !seen[root] {
			seen[root] = true
			roots = append(roots, root)
		}
	}
	return roots, nil
}
