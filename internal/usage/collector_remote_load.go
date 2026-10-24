package usage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// remoteLoadPollOptions configures the collector's remote load poll (issue
// 879 M2). Collect and Wait are injected so tests drive the schedule without
// sleeping; configuredRemoteLoadPoll fills them with the real SSH call and
// waitOrDone.
type remoteLoadPollOptions struct {
	Host    string
	Poll    time.Duration // cadence after a successful poll
	Initial time.Duration // first retry delay after a failed poll
	Max     time.Duration // cap for the retry delay
	Collect remoteLoadCollectFunc
	Wait    remoteLoadWaitFunc
	Out     io.Writer
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
	poll, _, err := RemoteLoadPolicy()
	if err != nil {
		fmt.Fprintf(out, "harnez server: remote load poll disabled: %v\n", err)
		return nil
	}
	return &remoteLoadPollOptions{
		Host:    strings.TrimSpace(local.Load.WatchHost),
		Poll:    poll,
		Initial: remoteLoadRetryInitial,
		Max:     remoteLoadRetryMax,
		Collect: CollectRemoteLoadSnapshot,
		Wait:    waitOrDone,
		Out:     out,
	}
}

// runRemoteLoadPoller polls the remote load host until ctx is done. It polls
// once at startup, so the cache is filled without any client or quota tick. A
// success is cached and followed by opt.Poll; a failure waits the retry delay,
// which doubles up to opt.Max. So a host that is down costs one SSH call per
// retry delay, not one per collector tick.
func runRemoteLoadPoller(ctx context.Context, opt remoteLoadPollOptions) {
	retryDelay := opt.Initial
	for ctx.Err() == nil {
		snap, err := opt.Collect(ctx, opt.Host)
		if err == nil && snap != nil {
			if writeErr := WriteCachedRemoteLoad(opt.Host, snap); writeErr != nil {
				fmt.Fprintf(opt.Out, "harnez server: remote load cache write failed: %v\n", writeErr)
			}
			retryDelay = opt.Initial
			opt.Wait(ctx, opt.Poll)
			continue
		}
		opt.Wait(ctx, retryDelay)
		retryDelay = nextRemoteLoadRetryDelay(retryDelay, opt.Max)
	}
}
