// Package hookdispatch runs every harnez handler for one agent hook event
// (issue 1304). A host hook entry calls `harnez hook <event>`; the registry
// decides which handlers run, in what order, under which time budget, and how
// their replies are merged into one valid reply for the host.
package hookdispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"
)

// Kind is the output contract a handler declares for its event.
type Kind int

const (
	// Silent handlers write nothing. Any bytes they write are dropped and noted.
	Silent Kind = iota
	// JSON handlers write one Claude hook reply object, or {}.
	JSON
	// Text handlers write plain text that the host adds as context (SessionStart).
	Text
)

// Func is a handler body. It reads the hook payload from in and writes its reply to out.
type Func func(ctx context.Context, in io.Reader, out io.Writer) error

// Handler is one harnez behaviour attached to an event.
type Handler struct {
	Name string
	Kind Kind
	// Tools limits the handler to these tool_name values. Empty means every payload.
	Tools []string
	Run   Func
}

// Registry maps each hook event name to its handlers, in run order.
type Registry map[string][]Handler

// Options configures one dispatch.
type Options struct {
	// Budget bounds one handler run. Required; it comes from spec/hooks.yaml.
	Budget time.Duration
	// Stderr receives notes about dropped handler output. Nil discards them.
	Stderr io.Writer
}

// Validate rejects registries whose handlers cannot be merged into one reply.
func (r Registry) Validate() error {
	for _, event := range r.Events() {
		hasJSON, hasText := false, false
		for _, h := range r[event] {
			if h.Name == "" {
				return fmt.Errorf("hook event %s: handler without a name", event)
			}
			if h.Run == nil {
				return fmt.Errorf("hook event %s: handler %s has no Run", event, h.Name)
			}
			switch h.Kind {
			case Silent:
			case JSON:
				hasJSON = true
			case Text:
				hasText = true
			default:
				return fmt.Errorf("hook event %s: handler %s has unknown kind %d", event, h.Name, h.Kind)
			}
		}
		if hasJSON && hasText {
			return fmt.Errorf("hook event %s mixes JSON and text handlers; one event needs one output contract", event)
		}
	}
	return nil
}

// Events returns the registered event names in sorted order.
func (r Registry) Events() []string {
	events := make([]string, 0, len(r))
	for event := range r {
		events = append(events, event)
	}
	sort.Strings(events)
	return events
}

// Dispatch runs the handlers registered for event, one after another, each
// under opts.Budget with panic recovery. A handler whose run fails, panics or
// exceeds the budget loses only its own output; the others still count. The
// merged reply goes to out. Dispatch returns an error only for a misconfigured
// call (unknown event, invalid registry, no budget), never for a handler failure.
func Dispatch(ctx context.Context, reg Registry, event string, raw []byte, out io.Writer, opts Options) error {
	if err := reg.Validate(); err != nil {
		return err
	}
	handlers, ok := reg[event]
	if !ok {
		return fmt.Errorf("unknown hook event %q", event)
	}
	if opts.Budget <= 0 {
		return errors.New("hook dispatch: handler budget must be positive")
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tool := toolName(raw)

	var replies [][]byte
	var kind Kind
	for _, h := range handlers {
		if len(h.Tools) > 0 && !slices.Contains(h.Tools, tool) {
			continue
		}
		data, err := runHandler(ctx, h, raw, opts.Budget)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "harnez hook %s: handler %s dropped: %v\n", event, h.Name, err)
			continue
		}
		if len(bytes.TrimSpace(data)) == 0 {
			continue
		}
		if h.Kind == Silent {
			fmt.Fprintf(opts.Stderr, "harnez hook %s: silent handler %s wrote output; dropped\n", event, h.Name)
			continue
		}
		kind = h.Kind
		replies = append(replies, data)
	}
	return writeReplies(out, opts.Stderr, event, kind, replies)
}

// runHandler runs one handler on its own goroutine. The reply is read only
// after the handler returns. On timeout the goroutine is abandoned and its
// buffer is never read, so a late write cannot reach the output.
func runHandler(ctx context.Context, h Handler, raw []byte, budget time.Duration) ([]byte, error) {
	hctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
			done <- err
		}()
		err = h.Run(hctx, bytes.NewReader(raw), &buf)
	}()
	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case <-hctx.Done():
		return nil, fmt.Errorf("exceeded the %s handler budget", budget)
	}
}

// writeReplies writes the merged reply. One reply passes through byte for byte,
// so a single handler behaves exactly as it did when it ran alone.
func writeReplies(out, errOut io.Writer, event string, kind Kind, replies [][]byte) error {
	switch len(replies) {
	case 0:
		return nil
	case 1:
		_, err := out.Write(replies[0])
		return err
	}
	if kind == Text {
		var parts []string
		for _, r := range replies {
			if s := strings.TrimSpace(string(r)); s != "" {
				parts = append(parts, s)
			}
		}
		_, err := fmt.Fprintln(out, strings.Join(parts, "\n"))
		return err
	}
	merged, notes := mergeClaudeReplies(replies)
	for _, note := range notes {
		fmt.Fprintf(errOut, "harnez hook %s: %s\n", event, note)
	}
	_, err := out.Write(merged)
	return err
}

// toolName reads tool_name from the payload. A payload that is not JSON has no tool.
func toolName(raw []byte) string {
	var p struct {
		ToolName string `json:"tool_name"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	return p.ToolName
}
