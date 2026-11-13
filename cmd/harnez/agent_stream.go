package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/agent"
	"ubunatic.com/harnez/internal/subagent"
)

// Stream modes: "full" prints every agent message as it arrives, "stats"
// prints only heartbeats and the final reply.
const (
	streamFull  = agent.StreamFull
	streamStats = agent.StreamStats
)

// confirmTimeout is how long an agent may stay silent before harnez warns that
// the early confirmation is missing.
var confirmTimeout = 10 * time.Second

// confirmTimer is replaceable in tests so warning order can be asserted with
// an event instead of delaying a fake provider reply.
var confirmTimer = func(d time.Duration) <-chan time.Time { return time.After(d) }

// heartbeatSchedule lists the elapsed times of the first heartbeats; after the
// last one a heartbeat follows every heartbeatRepeat.
var (
	heartbeatSchedule = []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 6 * time.Minute, 10 * time.Minute}
	heartbeatRepeat   = 5 * time.Minute
)

// withProtocol prepends the dispatch protocol, optional provider-family
// preamble (start only), the rules of the session's role (spec/agent.yaml) and,
// for plan-first turns, the review gate to the prompt.
func withProtocol(prompt string, planFirst bool, role string, inferred bool, provider string) string {
	return agent.WithProtocol(prompt, planFirst, role, inferred, provider)
}

func withResumeNote(prompt string, idle time.Duration) string {
	return agent.WithResumeNote(prompt, idle)
}

// heartbeatSchedule lists the elapsed times of the first heartbeats; after the
// last one a heartbeat follows every heartbeatRepeat.
// heartbeatAt returns the elapsed time of heartbeat i (0-based).
func heartbeatAt(i int) time.Duration {
	return agent.HeartbeatAt(i, heartbeatSchedule, heartbeatRepeat)
}

func checkStreamMode(mode string) error {
	return agent.CheckStreamMode(mode)
}

// turnStream prints a running turn to stdout as labeled blocks:
// [session info], [message]/[compaction ack] (full mode only), [heartbeat],
// [reply] (stats mode only) and [done].
type turnStream struct {
	w         io.Writer
	began     time.Time
	mode      string
	compacted bool   // the first message acknowledges a queued /compact
	stopCmd   string // shown when the agent violates the confirmation rule
	name      string // session name for the plan-first go-ahead hint
	planFirst bool

	mu       sync.Mutex
	bytes    int
	messages int
	commands int
	last     string
	// confirmed is set by the first non-ack message; violated and warned make
	// the protocol notices fire once.
	confirmed, violated, warned, planSeen bool
	lastShown                             string
	shown                                 bool
	doneMeta                              turnDoneMeta
	stop                                  chan struct{}
	done                                  sync.WaitGroup
}

type turnDoneMeta struct {
	sess         *subagent.Session
	compactLimit int
	quotaUnknown bool
	costSpec     subagent.CostCheckSpec
}

func newTurnStream(cmd *cobra.Command, mode string, compacted bool) *turnStream {
	return &turnStream{w: cmd.OutOrStdout(), began: time.Now(), mode: mode, compacted: compacted, stop: make(chan struct{})}
}

// humanCount renders 1230742 as 1.2M and 32700 as 32.7k.
func humanCount(n int) string { return subagent.HumanCount(n) }

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

// info prints the session header; resolved says how the session was chosen
// (new, name, dir or continue).
func (t *turnStream) info(id, agent, action, resolved string, extra ...string) {
	t.printf("[session info: id=%s agent=%s action=%s resolved=%s]\n", id, agent, action, resolved)
	for _, e := range extra {
		t.printf("%s\n", e)
	}
	t.printf("[wait: wait for [done]; %s]\n", waitHint(t.name))
}

func (t *turnStream) onEvent(ev subagent.Event) {
	t.mu.Lock()
	t.bytes += ev.Bytes
	t.mu.Unlock()
	switch ev.Kind {
	case "message":
		t.mu.Lock()
		defer t.mu.Unlock()
		t.messages++
		text, label := ev.Text, "message"
		switch {
		case t.compacted && t.messages == 1:
			label = "compaction ack"
		case strings.HasPrefix(text, "CONFIRM:"):
			label, text = "confirmation", strings.TrimSpace(strings.TrimPrefix(text, "CONFIRM:"))
		case strings.HasPrefix(text, "PLAN:"):
			label, text = "plan", strings.TrimSpace(strings.TrimPrefix(text, "PLAN:"))
			t.planSeen = true
		}
		if label != "compaction ack" {
			t.confirmed = true
		}
		t.last = "message"
		// Confirmation and plan are the caller's chance to intervene, so they
		// print in every mode.
		if t.mode == streamFull || label == "confirmation" || label == "plan" {
			t.lastShown, t.shown = text, true
			fmt.Fprintf(t.w, "[%s: %s]\n%s\n", label, shortDur(time.Since(t.began)), text)
		}
	case "activity":
		t.mu.Lock()
		defer t.mu.Unlock()
		t.last = ev.Text
		t.commands++
		if !t.confirmed && !t.violated {
			t.violated = true
			fmt.Fprintf(t.w, "[violation: %s before any confirmation; caller may stop the agent: %s]\n", ev.Text, t.stopCmd)
		}
	}
}

// watchConfirmation warns once when the agent is still silent after confirmTimeout.
func (t *turnStream) watchConfirmation() {
	t.done.Add(1)
	go func() {
		defer t.done.Done()
		select {
		case <-t.stop:
			return
		case <-confirmTimer(confirmTimeout):
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		if !t.confirmed && !t.warned && !t.violated {
			t.warned = true
			fmt.Fprintf(t.w, "[warning: no confirmation after %s; caller may stop the agent: %s]\n", shortDur(confirmTimeout), t.stopCmd)
		}
	}()
}

// watch starts the confirmation watchdog and prints progress summaries on the
// heartbeat schedule until finish or abort is called.
func (t *turnStream) watch() {
	t.watchConfirmation()
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
	if t.planFirst {
		t.finishPlanFirst(r)
		return
	}
	size := 0
	for _, m := range r.Messages {
		size += len(m)
	}
	if t.mode == streamStats {
		t.printf("[reply: %s]\n%s\n[done: %d messages (only the reply is shown), %d bytes, %s]\n", shortDur(time.Since(t.began)), r.Response, len(r.Messages), size, t.doneLineTail(r))
		t.emitCostCheck(r)
		return
	}
	if !t.shown || t.lastShown != r.Response {
		t.printf("[reply: %s]\n%s\n", shortDur(time.Since(t.began)), r.Response)
	}
	t.printf("[done: %d messages, last message is the reply, %d bytes, %s]\n", len(r.Messages), size, t.doneLineTail(r))
	t.emitCostCheck(r)
}

// abort stops the heartbeat goroutine when the turn failed.
func (t *turnStream) abort() {
	close(t.stop)
	t.done.Wait()
}

// tokenSummary reports per-turn and cumulative session tokens separately so the
// host can spot high cached re-read relative to new work added on the turn.
func tokenSummary(r *subagent.TurnResult, sess *subagent.Session) string {
	turnPart := "turn unknown"
	if r != nil && !r.TurnTokensUnknown {
		turnPart = fmt.Sprintf("turn %s new (%s out), %s cached", humanCount(subagent.CompactionTokens(r)), humanCount(r.OutputTokens), humanCount(r.CachedTokens))
	}
	sessPart := "session unknown"
	if sess != nil {
		if sess.TokenTotalsKnown {
			sessNew := max(sess.TokensCumulative-sess.CachedTokensTotal, 0)
			sessPart = fmt.Sprintf("session %s new, %s cached", humanCount(sessNew), humanCount(sess.CachedTokensTotal))
		}
	} else if r != nil && !r.TurnTokensUnknown {
		sessPart = fmt.Sprintf("session %s new, %s cached", humanCount(subagent.CompactionTokens(r)), humanCount(r.CachedTokens))
	}
	return fmt.Sprintf("tokens: %s; %s", turnPart, sessPart)
}

func contextSummary(r *subagent.TurnResult, compactLimit int) string {
	if r == nil || r.ContextTokens <= 0 {
		return "context usage unknown"
	}
	base := "context usage " + humanCount(r.ContextTokens)
	if r.ContextWindowSize > 0 {
		base = fmt.Sprintf("context usage %s of %s window", humanCount(r.ContextTokens), humanCount(r.ContextWindowSize))
	}
	if compactLimit > 0 {
		base += ", compaction limit " + humanCount(compactLimit)
	}
	return base
}

func (t *turnStream) doneLineTail(r *subagent.TurnResult) string {
	parts := []string{
		tokenSummary(r, t.doneMeta.sess),
		contextSummary(r, t.doneMeta.compactLimit),
	}
	if r != nil && r.ToolCallLimit > 0 && r.ToolCalls != nil {
		tc := fmt.Sprintf("tool calls: %d/%d", *r.ToolCalls, r.ToolCallLimit)
		if r.ToolCallLimitReached {
			tc += " (limit reached)"
		}
		parts = append(parts, tc)
	}
	if t.doneMeta.quotaUnknown {
		parts = append(parts, "quota unknown: cost check is the only signal")
	}
	parts = append(parts, shortDur(time.Since(t.began)))
	return strings.Join(parts, ", ")
}

func (t *turnStream) emitCostCheck(r *subagent.TurnResult) {
	if line := costCheckLine(r, t.doneMeta); line != "" {
		t.printf("%s\n", line)
	}
}

func costCheckLine(r *subagent.TurnResult, meta turnDoneMeta) string {
	if r == nil || meta.sess == nil || meta.sess.Role == "" {
		return ""
	}
	newTokens, cachedTokens := 0, 0
	if !r.TurnTokensUnknown {
		newTokens = subagent.CompactionTokens(r)
		cachedTokens = r.CachedTokens
	}
	freshStart := meta.sess.FreshStartContextTokens
	if freshStart <= 0 && meta.sess.Turn == 1 && r.ContextTokens > 0 {
		freshStart = r.ContextTokens
	}
	line, ok := subagent.CheckCost(meta.costSpec, subagent.CostCheckInput{
		Role:         meta.sess.Role,
		Model:        meta.sess.Provider + ":" + meta.sess.Model,
		Turn:         meta.sess.Turn,
		CachedTokens: cachedTokens,
		NewTokens:    newTokens,
		ContextUsage: r.ContextTokens,
		CompactLimit: meta.compactLimit,
		FreshStart:   freshStart,
	})
	if !ok {
		return ""
	}
	return line
}

// finishPlanFirst ends a plan-first turn: the plan was already printed live,
// so only the gate hint (or a missing-plan warning) and the totals follow.
func (t *turnStream) finishPlanFirst(r *subagent.TurnResult) {
	if !t.planSeen {
		t.printf("[warning: plan-first turn ended without a PLAN: message; the last message was: %s]\n", firstLine(r.Response))
	}
	t.printf("[gate: plan-first turn ended, nothing was executed; to proceed: harnez agent resume %s \"go ahead\"]\n", t.name)
	t.printf("[done: %d messages, %s]\n", len(r.Messages), t.doneLineTail(r))
	t.emitCostCheck(r)
}
