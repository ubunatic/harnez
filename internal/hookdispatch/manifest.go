package hookdispatch

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// CommandPrefix is the host command for one event: `harnez hook <Event>`.
const CommandPrefix = "harnez hook "

// Manifest lists, per event, the handlers that run and their tool routing,
// without run functions. cmd/harnez binds the functions with Bind. The host
// settings are derived from the manifest, so the events and matchers cannot
// drift from the handlers.
type Manifest map[string][]Handler

// Entry is one host hook entry: the event, the matcher ("" for none) and the command.
type Entry struct {
	Event   string
	Matcher string
	Command string
}

// Events returns the manifest's event names in sorted order.
func (m Manifest) Events() []string {
	return sortedKeys(m)
}

// Entries returns one entry per event, in event order. The matcher is the
// union of the handlers' Tools in run order, joined with "|". An event whose
// handlers declare no tools gets no matcher. An event that mixes handlers with
// and without tools is rejected, because the matcher would skip the untooled ones.
func (m Manifest) Entries() ([]Entry, error) {
	var out []Entry
	for _, event := range m.Events() {
		var tools []string
		tooled, untooled := 0, 0
		for _, h := range m[event] {
			if len(h.Tools) == 0 {
				untooled++
				continue
			}
			tooled++
			for _, tool := range h.Tools {
				if !slices.Contains(tools, tool) {
					tools = append(tools, tool)
				}
			}
		}
		if tooled > 0 && untooled > 0 {
			return nil, fmt.Errorf("hook event %s mixes handlers with and without tools; a matcher would skip the untooled ones", event)
		}
		e := Entry{Event: event, Command: CommandPrefix + event}
		e.Matcher = strings.Join(tools, "|")
		out = append(out, e)
	}
	return out, nil
}

// Bind returns the registry for m. runs maps "Event/Name" to the handler body.
// Bind fails if a manifest handler has no body or a body has no manifest
// handler. m itself is not changed.
func (m Manifest) Bind(runs map[string]Func) (Registry, error) {
	want := map[string]bool{}
	reg := Registry{}
	for _, event := range m.Events() {
		for _, h := range m[event] {
			key := event + "/" + h.Name
			want[key] = true
			run, ok := runs[key]
			if !ok {
				return nil, fmt.Errorf("hook manifest %s: no Run bound", key)
			}
			h.Run = run
			reg[event] = append(reg[event], h)
		}
	}
	for _, key := range sortedKeys(runs) {
		if !want[key] {
			return nil, fmt.Errorf("hook Run %s has no manifest handler", key)
		}
	}
	if err := reg.Validate(); err != nil {
		return nil, err
	}
	return reg, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
