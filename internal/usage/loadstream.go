package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// loadStreamInterval is the cadence at which `harnez load-stream` (the
// remote side, see RunLoadStream) samples and emits one LoadSnapshot line.
// It matches the existing local Load box's own redraw cadence (watch.go's
// loadTicker) — issue 110 Decision §6 explicitly reuses that interval
// rather than inventing a new one. A var (not const) so tests can shrink it.
var loadStreamInterval = time.Second

// RunLoadStream is the remote side of issue 110's streaming Load channel:
// the body of `harnez load-stream`, invoked over ssh by
// StartRemoteLoadStream's persistent child session. It loops sampling
// CollectLoadSnapshot at loadStreamInterval, writing one JSON-marshaled
// LoadSnapshot line to out per sample, until either:
//
//   - ctx is done (SIGINT/SIGTERM delivered to this process directly), or
//   - in reaches EOF (the natural signal that its parent ssh channel
//     closed, e.g. because the local --watch session tore down its
//     ControlMaster and killed this child).
//
// Both exits are clean returns — this process must never linger past its
// ssh parent's lifetime (the "no zombie" requirement from issue 110).
func RunLoadStream(ctx context.Context, out io.Writer, in io.Reader) error {
	stdinClosed := make(chan struct{})
	go func() {
		defer close(stdinClosed)
		// A single byte-at-a-time read is enough: nothing is ever expected
		// to arrive on stdin, we only care about detecting EOF/error, i.e.
		// the channel closing out from under us.
		buf := make([]byte, 1)
		for {
			if _, err := in.Read(buf); err != nil {
				return
			}
		}
	}()

	w := bufio.NewWriter(out)
	ticker := time.NewTicker(loadStreamInterval)
	defer ticker.Stop()

	for {
		snap := CollectLoadSnapshot()
		data, err := json.Marshal(snap)
		if err == nil {
			w.Write(data)
			w.WriteByte('\n')
			w.Flush()
		}

		select {
		case <-ctx.Done():
			return nil
		case <-stdinClosed:
			return nil
		case <-ticker.C:
		}
	}
}

// remoteLoadStreamCmd is the command run on the remote host over the
// persistent ssh channel. $PATH is extended the same way remote.go and
// history.go already do for `harnez usage --json` / `harnez usage history
// record`, so a `harnez` installed via `make install` (~/go/bin) is found
// even under a non-interactive ssh shell.
const remoteLoadStreamCmd = `PATH="$PATH:$HOME/go/bin:$HOME/bin:/usr/local/bin" harnez load-stream`

// sshControlMasterStartCmd, sshControlMasterExitCmd, and
// sshLoadStreamChildCmd construct (but do not run) the three ssh
// invocations StartRemoteLoadStream/controlMaster.Close need. They are
// package-level vars, not plain functions, so tests can substitute a fake
// command in place of a real `ssh` binary — the same
// mockable-exec.Command-boundary pattern agy.go's runAGYUsageCmdFn uses —
// without requiring a real SSH target.
var (
	sshControlMasterStartCmd = func(ctlPath, host string) *exec.Cmd {
		return exec.Command("ssh",
			"-M", "-S", ctlPath,
			"-o", "ControlPersist=60",
			"-o", "ConnectTimeout=5",
			"-o", "BatchMode=yes",
			"-fN", host)
	}
	sshControlMasterExitCmd = func(ctlPath, host string) *exec.Cmd {
		return exec.Command("ssh", "-S", ctlPath, "-o", "BatchMode=yes", "-O", "exit", host)
	}
	sshLoadStreamChildCmd = func(ctx context.Context, ctlPath, host string) *exec.Cmd {
		return exec.CommandContext(ctx, "ssh", "-S", ctlPath, "-o", "BatchMode=yes", host, remoteLoadStreamCmd)
	}
)

// controlMaster owns one OpenSSH ControlMaster connection's lifecycle, 1:1
// with a single `harnez usage --watch` process (issue 110 Decision §4): no
// cross-invocation persistence, no daemon, started when streaming is
// attempted and torn down via Close on that same process's exit path.
type controlMaster struct {
	host    string
	ctlPath string
	dir     string
}

// newControlMaster starts a background `ssh -M -S <ctl> -fN host` master
// connection for host, using a per-process-unique control socket under a
// freshly created temp dir — there is no cross-process sharing to design
// for (Decision §4), so a unique path per call is simplest and avoids any
// collision between concurrent --watch sessions.
func newControlMaster(host string) (*controlMaster, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("ssh host cannot be empty")
	}
	if !validSSHHostRe.MatchString(host) || strings.HasPrefix(host, "-") {
		return nil, fmt.Errorf("invalid ssh host %q", host)
	}

	dir, err := os.MkdirTemp("", "harnez-load-ctl-*")
	if err != nil {
		return nil, fmt.Errorf("create control socket dir: %w", err)
	}
	// ssh enforces a short max length on control socket paths (the
	// UNIX-domain socket path limit, ~104-108 bytes on most platforms) —
	// os.MkdirTemp's default base (often a long /tmp/...) plus a
	// descriptive name can blow that budget, so keep the socket's own
	// basename short.
	ctlPath := filepath.Join(dir, "cm")

	cmd := sshControlMasterStartCmd(ctlPath, host)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("start ssh control master to %s: %s", host, msg)
	}

	return &controlMaster{host: host, ctlPath: ctlPath, dir: dir}, nil
}

// Close tears the master down and removes its control socket dir. It is
// idempotent and safe to call even when setup only partially completed
// (e.g. the master started but a child spawn afterward failed) — under no
// exit path may it leave an `ssh -M` process or control socket behind.
func (cm *controlMaster) Close() {
	if cm == nil || cm.dir == "" {
		return
	}
	// Deliberately not tied to the caller's ctx: Close typically runs
	// during --watch's own shutdown, by which point that ctx is already
	// cancelled, and an already-cancelled context would stop this cleanup
	// exec.Cmd before "-O exit" ever ran — exactly backwards.
	_ = sshControlMasterExitCmd(cm.ctlPath, cm.host).Run()
	os.RemoveAll(cm.dir)
	cm.dir = ""
}

// StartRemoteLoadStream is the local (--watch-process) side of issue 110's
// streaming Load channel: it opens a ControlMaster to host (Option A), then
// spawns `harnez load-stream` over it as a persistent child session,
// scanning its stdout line by line and delivering each parsed LoadSnapshot
// on the returned channel.
//
// The returned stop func tears everything down (kills the streaming child,
// closes the ControlMaster, removes its control socket dir) and is safe to
// call multiple times or after a partial failure. Callers must always call
// it, typically via defer, even when err != nil, since setup can fail after
// the ControlMaster is already up.
//
// The channel closes when the child's stdout ends (process exited, ssh
// connection dropped, or ctx was cancelled) — callers should treat a closed
// channel as "streaming is no longer available" and fall back to batch
// polling (Decision §2/§3), optionally retrying StartRemoteLoadStream again
// later.
func StartRemoteLoadStream(ctx context.Context, host string) (<-chan LoadSnapshot, func(), error) {
	noop := func() {}

	cm, err := newControlMaster(host)
	if err != nil {
		return nil, noop, err
	}

	cmd := sshLoadStreamChildCmd(ctx, cm.ctlPath, host)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cm.Close()
		return nil, noop, fmt.Errorf("pipe stdout for load-stream child on %s: %w", host, err)
	}
	if err := cmd.Start(); err != nil {
		cm.Close()
		return nil, noop, fmt.Errorf("start load-stream child on %s: %w", host, err)
	}

	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
			cm.Close()
		})
	}

	ch := make(chan LoadSnapshot, 1)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var snap LoadSnapshot
			if err := json.Unmarshal(line, &snap); err != nil {
				// Malformed/partial line (e.g. torn by a network blip):
				// skip it and keep streaming rather than tearing the
				// whole channel down over one bad sample.
				continue
			}
			select {
			case ch <- snap:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, stop, nil
}
