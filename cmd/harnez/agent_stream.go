package main

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
)

// heartbeatSchedule is the gap before each heartbeat; the last gap repeats.
var heartbeatSchedule = []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 10 * time.Minute}

// turnStream prints a running turn to stdout as labeled blocks:
// [session info], [message]/[compaction ack], [heartbeat] and [done].
type turnStream struct {
	w         io.Writer
	began     time.Time
	schedule  []time.Duration
	compacted bool // the first message acknowledges a queued /compact

	mu       sync.Mutex
	bytes    int
	messages int
	last     string
	stop     chan struct{}
	done     sync.WaitGroup
}

func newTurnStream(cmd *cobra.Command, compacted bool) *turnStream {
	return &turnStream{w: cmd.OutOrStdout(), began: time.Now(), schedule: heartbeatSchedule, compacted: compacted, stop: make(chan struct{})}
}

func shortDur(d time.Duration) string { return d.Round(time.Second).String() }

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
	t.printf("[wait: one synchronous turn; caller must wait for [done], no polling, no re-sending the prompt]\n")
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
		fmt.Fprintf(t.w, "[%s: %s]\n%s\n", label, shortDur(time.Since(t.began)), ev.Text)
		t.mu.Unlock()
	case "activity":
		t.mu.Lock()
		t.last = ev.Text
		t.mu.Unlock()
	}
}

// startHeartbeats prints progress lines on the schedule until finish is called.
func (t *turnStream) startHeartbeats() {
	t.done.Add(1)
	go func() {
		defer t.done.Done()
		for i := 0; ; i++ {
			gap := t.schedule[min(i, len(t.schedule)-1)]
			next := t.schedule[min(i+1, len(t.schedule)-1)]
			select {
			case <-t.stop:
				return
			case <-time.After(gap):
			}
			t.mu.Lock()
			fmt.Fprintf(t.w, "[heartbeat: ~%d tokens, %s, next: %s, last: %s]\n", t.bytes/4, shortDur(time.Since(t.began)), next, t.last)
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
	t.printf("[done: %d messages, last message is the reply, %d bytes, %d tokens, %s]\n", len(r.Messages), size, r.TokensTurn, shortDur(time.Since(t.began)))
}

// abort stops the heartbeat goroutine when the turn failed.
func (t *turnStream) abort() {
	close(t.stop)
	t.done.Wait()
}
