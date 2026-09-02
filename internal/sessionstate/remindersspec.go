package sessionstate

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// remindersSpecPath is the embedded location of the session-usage reminder
// registry (issue 187). GapTip reads the summary reminder's wording and
// firing threshold from here rather than hardcoded Go string literals --
// see docs/other/Spec.md.
const remindersSpecPath = "spec/reminders.yaml"

// summaryReminderName is the one reminder GapTip currently looks up by name.
// The spec format supports more than one named reminder for future growth,
// but only "summary" (issue 187) is wired up today.
const summaryReminderName = "summary"

// validReplaces mirrors spec/schemas/reminders.schema.json's "replaces"
// enum: the existing GapTip tips a spec-defined reminder is allowed to
// declare it takes over from.
var validReplaces = map[string]bool{
	"plain_rate_gap":     true,
	"heartbeat_rate_gap": true,
}

// reminderSpec is one entry in spec/reminders.yaml. Mirrors
// spec/schemas/reminders.schema.json field-for-field.
type reminderSpec struct {
	Title               string   `yaml:"title"`
	Description         string   `yaml:"description,omitempty"`
	Message             string   `yaml:"message"`
	ThresholdMultiplier float64  `yaml:"threshold_multiplier"`
	Replaces            []string `yaml:"replaces"`
}

// remindersSpecFile is the top-level shape of spec/reminders.yaml.
type remindersSpecFile struct {
	Reminders map[string]reminderSpec `yaml:"reminders"`
}

// parseRemindersYAML parses and validates spec/reminders.yaml content. It
// returns a clear error (never panics) for malformed YAML or a spec that
// violates the schema's invariants -- the same "fail clearly, not panic"
// contract issue 132 established for spec/actions.yaml.
func parseRemindersYAML(data []byte) (remindersSpecFile, error) {
	var spec remindersSpecFile
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return remindersSpecFile{}, fmt.Errorf("reminders spec: parse: %w", err)
	}
	if len(spec.Reminders) == 0 {
		return remindersSpecFile{}, fmt.Errorf("reminders spec: no reminders defined")
	}
	for name, r := range spec.Reminders {
		if strings.TrimSpace(r.Title) == "" {
			return remindersSpecFile{}, fmt.Errorf("reminders spec: reminder %q: missing title", name)
		}
		if strings.TrimSpace(r.Message) == "" {
			return remindersSpecFile{}, fmt.Errorf("reminders spec: reminder %q: missing message", name)
		}
		if r.ThresholdMultiplier <= 0 {
			return remindersSpecFile{}, fmt.Errorf("reminders spec: reminder %q: threshold_multiplier must be > 0", name)
		}
		for _, rep := range r.Replaces {
			if !validReplaces[rep] {
				return remindersSpecFile{}, fmt.Errorf("reminders spec: reminder %q: unknown replaces entry %q", name, rep)
			}
		}
	}
	return spec, nil
}

// loadRemindersSpec reads and validates the embedded spec/reminders.yaml.
func loadRemindersSpec() (remindersSpecFile, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, remindersSpecPath)
	if err != nil {
		return remindersSpecFile{}, fmt.Errorf("reminders spec: read %s: %w", remindersSpecPath, err)
	}
	return parseRemindersYAML(data)
}

var remindersSpecOnce = sync.OnceValues(loadRemindersSpec)

// mustRemindersSpec returns the parsed embedded reminders spec, panicking if
// it's invalid. It is expected to always succeed -- spec/reminders.yaml is
// compiled into the binary and covered by
// TestEmbeddedRemindersSpecIsValid -- so a failure here means the binary
// itself was built with a broken spec, not a runtime/user condition. GapTip
// itself does NOT call this: it uses the error-returning summaryReminder()
// below so a broken embedded spec degrades to the older plain/heartbeat
// tips instead of ever panicking out of the "never block a real command"
// sessionTipHook path (cmd/harnez/main.go).
func mustRemindersSpec() remindersSpecFile {
	spec, err := remindersSpecOnce()
	if err != nil {
		panic(fmt.Sprintf("harnez: embedded %s is invalid: %v", remindersSpecPath, err))
	}
	return spec
}

// summaryReminder returns the embedded spec's "summary" reminder
// definition, or ok=false if the embedded spec failed to load/validate or
// doesn't define one. GapTip treats this as best-effort: any failure here
// just means the summary reminder doesn't fire this call, falling back to
// the existing plain/heartbeat tips, rather than surfacing an error or
// panicking.
func summaryReminder() (reminderSpec, bool) {
	spec, err := remindersSpecOnce()
	if err != nil {
		return reminderSpec{}, false
	}
	r, ok := spec.Reminders[summaryReminderName]
	return r, ok
}

// formatReminderDuration renders d as a short "1h30m"/"45m"/"2h" style
// string for substitution into a reminder message's {duration} placeholder
// -- concise enough for a single-line CLI tip, unlike time.Duration's own
// String() (which would print "1h30m0s").
func formatReminderDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// renderReminder substitutes r's {calls}/{duration} placeholders with real
// values computed from session state.
func renderReminder(r reminderSpec, calls int, elapsed time.Duration) string {
	replacer := strings.NewReplacer(
		"{calls}", strconv.Itoa(calls),
		"{duration}", formatReminderDuration(elapsed),
	)
	return replacer.Replace(r.Message)
}
