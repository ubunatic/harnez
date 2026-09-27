//go:build unix

package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func runtimeDir(opts Options) string {
	if opts.RuntimeDir != "" {
		return opts.RuntimeDir
	}
	if base := os.Getenv("XDG_RUNTIME_DIR"); base != "" {
		return filepath.Join(base, "harnez", "usage")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".harnez", "run", "usage")
}

func socketPath(opts Options) string { return filepath.Join(runtimeDir(opts), "controller.sock") }

func startIfAbsent(opts Options) error {
	if _, err := requestController(context.Background(), opts, wireRequest{Version: ProtocolVersion, Operation: "snapshot"}); err == nil {
		return nil
	}
	dir := runtimeDir(opts)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("usage: create runtime directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("usage: secure runtime directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(dir, "controller.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("usage: open controller lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil
		}
		return fmt.Errorf("usage: lock controller: %w", err)
	}
	path := socketPath(opts)
	if _, err := requestController(context.Background(), opts, wireRequest{Version: ProtocolVersion, Operation: "snapshot"}); err == nil {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
		return nil
	}
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
		return fmt.Errorf("usage: listen on controller socket: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
		return fmt.Errorf("usage: secure controller socket: %w", err)
	}
	controller := &controller{
		opts:        opts,
		listener:    listener,
		lock:        lock,
		waiters:     make(map[uint64]chan SnapshotEvent),
		closed:      make(chan struct{}),
		lastRefresh: make(map[ProviderID]time.Time),
		inFlight:    make(map[ProviderID]*refreshFlight),
	}
	go controller.serve()
	go controller.shutdownWhenIdle()
	return nil
}

func closeController(controller *controller) {
	_ = controller.listener.Close()
	_ = os.Remove(socketPath(controller.opts))
	_ = syscall.Flock(int(controller.lock.Fd()), syscall.LOCK_UN)
	_ = controller.lock.Close()
}

func requestController(ctx context.Context, opts Options, request wireRequest) (wireResponse, error) {
	var response wireResponse
	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "unix", socketPath(opts))
	if err != nil {
		return response, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return response, err
	}
	if err := json.NewDecoder(bufio.NewReader(io.LimitReader(conn, 1<<20))).Decode(&response); err != nil {
		return response, err
	}
	return response, nil
}

func subscribeController(ctx context.Context, opts Options, provider ProviderID) (<-chan SnapshotEvent, error) {
	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(ctx, "unix", socketPath(opts))
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(conn).Encode(wireRequest{Version: ProtocolVersion, Operation: "subscribe", Providers: []ProviderID{provider}}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	updates := make(chan SnapshotEvent)
	go func() {
		defer close(updates)
		defer conn.Close()
		decoder := json.NewDecoder(bufio.NewReader(io.LimitReader(conn, 1<<20)))
		for {
			var event SnapshotEvent
			if err := decoder.Decode(&event); err != nil {
				return
			}
			select {
			case updates <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return updates, nil
}
