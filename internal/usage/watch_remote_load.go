package usage

import (
	"context"
	"sync"
	"time"
)

// Remote load retry backoff is an internal policy rather than a user-facing
// setting, so keep it as constants instead of duplicating it in spec/.
const (
	remoteLoadRetryInitial = 10 * time.Second
	remoteLoadRetryMax     = 2 * time.Minute
)

type remoteLoadWaitFunc func(context.Context, time.Duration)
type remoteLoadCollectFunc func(context.Context, string) (*LoadSnapshot, error)

func remoteLoadCollectionEnabled(host string, disabled bool) bool {
	return host != "" && !disabled
}

// runRemoteLoadManager owns the full lifecycle of --watch's remote Load
// data source for one configured host: it keeps trying to establish the
// streaming channel (StartRemoteLoadStream), and whenever that's down —
// never established, or dropped mid-session — falls back to periodic batch
// polling (CollectRemoteLoadSnapshot) so the box still shows fresh data,
// retrying the stream in the background the whole time. It runs for the
// life of ctx (RunWatchWithOptions's sigCtx) and returns once ctx is done.
//
// setStop is called with the current stream's teardown func every time a
// stream is established (nil once it ends), so RunWatchWithOptions's own
// deferred cleanup can synchronously tear down whichever ssh
// ControlMaster/child happens to be live at the moment it returns, rather
// than trusting this goroutine to get there first (see the "no zombie"
// requirement in issue 110 — the caller, not just this manager, must be
// able to guarantee cleanup on every exit path).
//
// setStreaming reports the current data-source mode so the UI can show
// whether the box is populated live over the stream or via a batch-poll
// fallback (issue 110 follow-up) — it is purely a display signal and never
// influences the fetch logic itself.
func runRemoteLoadManager(ctx context.Context, host string, mu *sync.Mutex, last **LoadSnapshot, redraw func(), setStop func(func()), setStreaming func(bool)) {
	runRemoteLoadManagerWithPolicy(ctx, host, mu, last, redraw, setStop, setStreaming,
		remoteLoadRetryInitial, remoteLoadRetryMax, waitOrDone, CollectRemoteLoadSnapshot)
}

func runRemoteLoadManagerWithPolicy(ctx context.Context, host string, mu *sync.Mutex, last **LoadSnapshot, redraw func(), setStop func(func()), setStreaming func(bool), initial, max time.Duration, wait remoteLoadWaitFunc, collect remoteLoadCollectFunc) {
	setLast := func(snap *LoadSnapshot) {
		_ = WriteCachedRemoteLoad(host, snap)
		mu.Lock()
		*last = snap
		mu.Unlock()
		redraw()
	}

	retryDelay := initial
	for ctx.Err() == nil {
		ch, stop, err := StartRemoteLoadStream(ctx, host)
		if err != nil {
			// Streaming unavailable right now (stale remote harnez binary
			// predating `load-stream`, network blip, etc.) — fall back to
			// one batch poll now and retry establishing the stream after
			// the current backoff delay.
			setStreaming(false)
			pollSucceeded := pollRemoteLoad(ctx, host, setLast, collect)
			if pollSucceeded {
				retryDelay = initial
			}
			wait(ctx, retryDelay)
			if ctx.Err() == nil && !pollSucceeded && retryDelay < max {
				retryDelay = min(retryDelay*2, max)
			}
			continue
		}

		retryDelay = initial
		setStop(stop)
		setStreaming(true)
		drainRemoteLoadStream(ctx, ch, setLast)
		stop()
		setStop(nil)
		setStreaming(false)

		if ctx.Err() != nil {
			return
		}
		// The stream ended (remote load-stream process died, connection
		// dropped, etc.) rather than this manager shutting down — same
		// fallback-then-retry behavior as a failed initial connect.
		pollSucceeded := pollRemoteLoad(ctx, host, setLast, collect)
		if pollSucceeded {
			retryDelay = initial
		}
		wait(ctx, retryDelay)
		if ctx.Err() == nil && !pollSucceeded && retryDelay < max {
			retryDelay = min(retryDelay*2, max)
		}
	}
}

// drainRemoteLoadStream reads every sample off ch, applying each via
// setLast, until ch closes (the stream ended) or ctx is done.
func drainRemoteLoadStream(ctx context.Context, ch <-chan LoadSnapshot, setLast func(*LoadSnapshot)) {
	for {
		select {
		case snap, ok := <-ch:
			if !ok {
				return
			}
			s := snap
			setLast(&s)
		case <-ctx.Done():
			return
		}
	}
}

// pollOnce fetches one batch remote Load snapshot and, only on success,
// applies it via setLast — a failed poll leaves the last-known snapshot in
// place (stale-but-present) rather than blanking the box, matching how a
// failed --host fetch elsewhere in this file already degrades.
func pollRemoteLoad(ctx context.Context, host string, setLast func(*LoadSnapshot), collect remoteLoadCollectFunc) bool {
	snap, err := collect(ctx, host)
	if err == nil && snap != nil {
		setLast(snap)
		return true
	}
	return false
}

// waitOrDone blocks for d or until ctx is done, whichever comes first.
func waitOrDone(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
