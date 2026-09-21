package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
)

// Stream modes: "full" prints every agent message as it arrives, "stats"
// prints only heartbeats and the final reply.
const (
	streamFull  = "full"
	streamStats = "stats"
)

// heartbeatSchedule lists the elapsed times of the first heartbeats; after the
// last one a heartbeat follows every heartbeatRepeat.
var (
	heartbeatSchedule = []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 6 * time.Minute, 10 * time.Minute}
	heartbeatRepeat   = 5 * time.Minute
)

// heartbeatAt returns the elapsed time of heartbeat i (0-based).
func heartbeatAt(i int) time.Duration {
	if i < len(heartbeatSchedule) {
		return heartbeatSchedule[i]
	}
	return heartbeatSchedule[len(heartbeatSchedule)-1] + time.Duration(i-len(heartbeatSchedule)+1)*heartbeatRepeat
}

func checkStreamMode(mode string) error {
	if mode != streamFull && mode != streamStats {
		return fmt.Errorf("invalid --stream %q: want %q or %q", mode, streamFull, streamStats)
	}
	return nil
}

// turnStream prints a running turn to stdout as labeled blocks:
// [session info], [message]/[compaction ack] (full mode only), [heartbeat],
// [reply] (stats mode only) and [done].
type turnStream struct {
	w         io.Writer
	began     time.Time
	mode      string
	compacted bool // the first message acknowledges a queued /compact

	mu       sync.Mutex
	bytes    int
	messages int
	commands int
	last     string
	stop     chan struct{}
	done     sync.WaitGroup
}

func newTurnStream(cmd *cobra.Command, mode string, compacted bool) *turnStream {
	return &turnStream{w: cmd.OutOrStdout(), began: time.Now(), mode: mode, compacted: compacted, stop: make(chan struct{})}
}

// humanCount renders 1230742 as 1.2M and 32700 as 32.7k.
func humanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// shortDur renders 1m0s as 1m and 1h0m0s as 1h.
func shortDur(d time.Duration) string {
	s := d.Round(time.Second).String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

func (t *turnStream) printf(format string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.w, format, args...)
}

func (t *turnStream) info(id, agent, action string, extra ...string) {
	t.printf("[session info: id=%s agent=%s action=%s]\n", id, agent, action)
	for _, e := range extra {
		t.printf("%s\n", e)
	}
	t.printf("[wait: synchronous turn, wait for [done]; no polling, no re-sending the prompt]\n")
}

func (t *turnStream) onEvent(ev subagent.Event) {
	t.mu.Lock()
	t.bytes += ev.Bytes
	t.mu.Unlock()
	switch ev.Kind {
	case "message":
		t.mu.Lock()
		t.messages++
		label := "message"
		if t.compacted && t.messages == 1 {
			label = "compaction ack"
		}
		t.last = "message"
		if t.mode == streamFull {
			fmt.Fprintf(t.w, "[%s: %s]\n%s\n", label, shortDur(time.Since(t.began)), ev.Text)
		}
		t.mu.Unlock()
	case "activity":
		t.mu.Lock()
		t.last = ev.Text
		t.commands++
		t.mu.Unlock()
	}
}

// startHeartbeats prints progress summaries on the schedule until finish is called.
func (t *turnStream) startHeartbeats() {
	t.done.Add(1)
	go func() {
		defer t.done.Done()
		for i := 0; ; i++ {
			select {
			case <-t.stop:
				return
			case <-time.After(time.Until(t.began.Add(heartbeatAt(i)))):
			}
			t.mu.Lock()
			fmt.Fprintf(t.w, "[heartbeat: %s, ~%s tokens, %d messages, %d commands, next: %s, last: %s]\n", shortDur(time.Since(t.began)), humanCount(t.bytes/4), t.messages, t.commands, shortDur(heartbeatAt(i+1)), t.last)
			t.mu.Unlock()
		}
	}()
}

func (t *turnStream) finish(r *subagent.TurnResult) {
	close(t.stop)
	t.done.Wait()
	size := 0
	for _, m := range r.Messages {
		size += len(m)
	}
	if t.mode == streamStats {
		t.printf("[reply: %s]\n%s\n[done: %d messages (only the reply is shown), %d bytes, %s, %s]\n", shortDur(time.Since(t.began)), r.Response, len(r.Messages), size, tokenSummary(r), shortDur(time.Since(t.began)))
		return
	}
	t.printf("[done: %d messages, last message is the reply, %d bytes, %s, %s]\n", len(r.Messages), size, tokenSummary(r), shortDur(time.Since(t.began)))
}

// abort stops the heartbeat goroutine when the turn failed.
func (t *turnStream) abort() {
	close(t.stop)
	t.done.Wait()
}

// tokenSummary separates tokens the turn added from input re-read from the
// provider cache, so long-context sessions do not look expensive.
func tokenSummary(r *subagent.TurnResult) string {
	return fmt.Sprintf("tokens: %s new (%s out), %s cached", humanCount(subagent.CompactionTokens(r)), humanCount(r.OutputTokens), humanCount(r.CachedTokens))
}
