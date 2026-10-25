package usage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// Retry backoff is an internal policy, separate from the configured cadence.
const (
	remoteLoadRetryInitial = 10 * time.Second
	remoteLoadRetryMax     = 2 * time.Minute
)

type remoteLoadCollectFunc func(context.Context, string) (*LoadSnapshot, error)

// remoteLoadPollOptions configures the server-owned remote load poll. Collect
// and Wait are injected so tests drive the schedule without sleeping.
type remoteLoadPollWaitFunc func(context.Context, time.Duration, <-chan struct{})

type remoteLoadPollOptions struct {
	Host          string
	PollAttached  time.Duration
	PollDetached  time.Duration
	Initial       time.Duration // first retry delay after a failed poll
	Max           time.Duration // cap for the retry delay
	ActiveClients func() int
	WakeOnAttach  <-chan struct{}
	Collect       remoteLoadCollectFunc
	Wait          remoteLoadPollWaitFunc
	Out           io.Writer
}

// configuredRemoteLoadPoll returns the poll for load.watch_host from local.yaml,
// or nil when no host is configured. A malformed local.yaml or an invalid
// spec disables the poll with a message: remote load must never stop the
// server. --offline does not disable it, because it gates only the quota API.
func configuredRemoteLoadPoll(homeDir string, out io.Writer) *remoteLoadPollOptions {
	local, _, err := LoadLocalConfig(homeDir)
	if err != nil {
		fmt.Fprintf(out, "harnez server: remote load poll disabled: %v\n", err)
		return nil
	}
	if local == nil || strings.TrimSpace(local.Load.WatchHost) == "" {
		return nil
	}
	attached, detached, _, err := RemoteLoadPolicy()
	if err != nil {
		fmt.Fprintf(out, "harnez server: remote load poll disabled: %v\n", err)
		return nil
	}
	return &remoteLoadPollOptions{
		Host:         strings.TrimSpace(local.Load.WatchHost),
		PollAttached: attached,
		PollDetached: detached,
		Initial:      remoteLoadRetryInitial,
		Max:          remoteLoadRetryMax,
		Collect:      CollectRemoteLoadSnapshot,
		Wait:         waitRemoteLoadPoll,
		Out:          out,
	}
}

// runRemoteLoadPoller polls the remote load host until ctx is done. It polls
// once at startup, so the cache is filled without any client or quota tick. A
// success is cached and followed by the attached or detached cadence. A new
// client interrupts a detached wait; failures back off while attached and use
// the detached cadence while no UI is active.
func runRemoteLoadPoller(ctx context.Context, opt remoteLoadPollOptions) {
	retryDelay := opt.Initial
	failing := false
	for ctx.Err() == nil {
		snap, err := opt.Collect(ctx, opt.Host)
		if ctx.Err() != nil {
			return
		}
		if err == nil && snap != nil {
			if failing {
				fmt.Fprintf(opt.Out, "harnez server: remote load %s recovered\n", opt.Host)
				failing = false
			}
			if writeErr := WriteCachedRemoteLoad(opt.Host, snap); writeErr != nil {
				fmt.Fprintf(opt.Out, "harnez server: remote load cache write failed: %v\n", writeErr)
			}
			retryDelay = opt.Initial
			opt.wait(ctx, opt.pollInterval())
			continue
		}
		if !failing {
			message := "empty load snapshot"
			if err != nil {
				message = strings.TrimSpace(err.Error())
			}
			fmt.Fprintf(opt.Out, "harnez server: remote load %s failed: %s\n", opt.Host, message)
			failing = true
		}
		delay := retryDelay
		if interval := opt.pollInterval(); interval > delay {
			delay = interval
		}
		opt.wait(ctx, delay)
		retryDelay = nextRemoteLoadRetryDelay(retryDelay, opt.Max)
	}
}

func (opt remoteLoadPollOptions) pollInterval() time.Duration {
	if opt.ActiveClients != nil && opt.ActiveClients() > 0 {
		return opt.PollAttached
	}
	return opt.PollDetached
}

func (opt remoteLoadPollOptions) wait(ctx context.Context, delay time.Duration) {
	wait := opt.Wait
	if wait == nil {
		wait = waitRemoteLoadPoll
	}
	wait(ctx, delay, opt.WakeOnAttach)
}

func waitRemoteLoadPoll(ctx context.Context, delay time.Duration, wakeOnAttach <-chan struct{}) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	case <-wakeOnAttach:
	}
}

func nextRemoteLoadRetryDelay(delay, max time.Duration) time.Duration {
	return min(delay*2, max)
}
