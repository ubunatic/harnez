package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// errAgentSignalled is the cancel cause when a signal ends a foreground turn
// (issue 1029). It keeps a signal stop apart from a --timeout deadline.
type errAgentSignalled struct{ sig os.Signal }

func (e errAgentSignalled) Error() string {
	return fmt.Sprintf("interrupted by %s", signalName(e.sig))
}

func signalName(sig os.Signal) string {
	switch sig {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGHUP:
		return "SIGHUP"
	}
	return sig.String()
}

// agentSignalContext cancels the returned context on SIGINT, SIGTERM or
// SIGHUP, so the provider's cmd.Cancel kills its process group instead of
// Go's default exit leaving the child running. After the first signal the
// default handling is back: a second signal ends harnez at once if the
// cleanup hangs. Call stop when the turn is over.
func agentSignalContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigs:
			signal.Stop(sigs)
			cancel(errAgentSignalled{sig})
		case <-done:
		}
	}()
	return ctx, func() {
		signal.Stop(sigs)
		close(done)
		cancel(context.Canceled)
	}
}

// agentSignalCause returns the signal stop that ended ctx, if any.
func agentSignalCause(ctx context.Context) (errAgentSignalled, bool) {
	var sig errAgentSignalled
	if ctx == nil {
		return sig, false
	}
	ok := errors.As(context.Cause(ctx), &sig)
	return sig, ok
}
