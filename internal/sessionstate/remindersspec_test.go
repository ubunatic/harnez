package sessionstate

import (
	"strings"
	"testing"
	"time"
)

// TestEmbeddedRemindersSpecIsValid guards the committed spec/reminders.yaml
// itself: the binary embeds it via //go:embed (embed.go), and
// mustRemindersSpec() panics if it's malformed, so this is the test that
// would catch a broken spec at CI time instead of at first invocation.
func TestEmbeddedRemindersSpecIsValid(t *testing.T) {
	spec, err := loadRemindersSpec()
	if err != nil {
		t.Fatalf("embedded spec/reminders.yaml failed to load: %v", err)
	}
	if len(spec.Reminders) == 0 {
		t.Fatalf("expected at least one reminder in the embedded spec")
	}
	if _, ok := spec.Reminders[summaryReminderName]; !ok {
		t.Fatalf("expected the embedded spec to define a %q reminder", summaryReminderName)
	}

	// mustRemindersSpec() must not panic against the real embedded spec.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("mustRemindersSpec() panicked against the embedded spec: %v", r)
		}
	}()
	mustRemindersSpec()
}

func TestParseRemindersYAML_MalformedFailsClearly(t *testing.T) {
	_, err := parseRemindersYAML([]byte("reminders: [this is not a map"))
	if err == nil {
		t.Fatalf("expected an error for malformed YAML, got nil")
	}
}

func TestParseRemindersYAML_ValidatesRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"no reminders", "reminders: {}\n"},
		{"missing title", "reminders:\n  summary:\n    message: \"x\"\n    threshold_multiplier: 4\n    replaces: []\n"},
		{"missing message", "reminders:\n  summary:\n    title: \"x\"\n    threshold_multiplier: 4\n    replaces: []\n"},
		{"zero threshold", "reminders:\n  summary:\n    title: \"x\"\n    message: \"x\"\n    threshold_multiplier: 0\n    replaces: []\n"},
		{"negative threshold", "reminders:\n  summary:\n    title: \"x\"\n    message: \"x\"\n    threshold_multiplier: -1\n    replaces: []\n"},
		{"unknown replaces entry", "reminders:\n  summary:\n    title: \"x\"\n    message: \"x\"\n    threshold_multiplier: 4\n    replaces: [\"bogus\"]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseRemindersYAML([]byte(tc.yaml)); err == nil {
				t.Errorf("expected an error for case %q", tc.name)
			}
		})
	}
}

func TestParseRemindersYAML_AcceptsValidSpec(t *testing.T) {
	yaml := "reminders:\n  summary:\n    title: \"Summary\"\n    message: \"{calls} calls in {duration}\"\n    threshold_multiplier: 4\n    replaces: [\"plain_rate_gap\", \"heartbeat_rate_gap\"]\n"
	spec, err := parseRemindersYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("expected valid spec to parse cleanly, got: %v", err)
	}
	r, ok := spec.Reminders["summary"]
	if !ok {
		t.Fatalf("expected a %q reminder", "summary")
	}
	if r.ThresholdMultiplier != 4 {
		t.Errorf("expected threshold_multiplier=4, got %v", r.ThresholdMultiplier)
	}
}

func TestFormatReminderDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Minute, "45m"},
		{90 * time.Minute, "1h30m"},
		{2 * time.Hour, "2h"},
		{30 * time.Second, "1m"}, // time.Duration.Round rounds .5 up
		{10 * time.Second, "0m"},
	}
	for _, tc := range cases {
		if got := formatReminderDuration(tc.d); got != tc.want {
			t.Errorf("formatReminderDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestRenderReminder_SubstitutesRealNumbers(t *testing.T) {
	r := reminderSpec{Message: "Summary: {calls} tool calls in {duration}; call `harnez rate --ok`."}
	got := renderReminder(r, 83, 92*time.Minute)
	want := "Summary: 83 tool calls in 1h32m; call `harnez rate --ok`."
	if got != want {
		t.Errorf("renderReminder() = %q, want %q", got, want)
	}
}

// Issue 187: GapTip condition-trigger / composition tests.

func TestGapTip_SummaryThresholdSupersedesHeartbeatAndPlain(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}

	spec, err := loadRemindersSpec()
	if err != nil {
		t.Fatalf("loadRemindersSpec: %v", err)
	}
	r := spec.Reminders[summaryReminderName]
	summaryThreshold := int(r.ThresholdMultiplier * float64(rateGapThreshold))
	if summaryThreshold <= heartbeatGapThreshold {
		t.Fatalf("expected the summary threshold (%d) to be strictly larger than heartbeatGapThreshold (%d)",
			summaryThreshold, heartbeatGapThreshold)
	}

	for i := 0; i < summaryThreshold; i++ {
		Record(&s, "distill", now)
	}

	tip, ok := GapTip(s, false, now)
	if !ok {
		t.Fatalf("expected a summary tip to fire after %d unrated calls", s.Total)
	}
	if !strings.Contains(tip, "Summary:") {
		t.Errorf("expected the summary reminder wording, got: %q", tip)
	}
	if !strings.Contains(tip, "harnez rate --ok") {
		t.Errorf("expected the summary reminder to still point at `harnez rate --ok`, got: %q", tip)
	}
}

func TestGapTip_BelowSummaryThresholdStillUsesHeartbeatTip(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}
	// Past heartbeatGapThreshold but nowhere near the (4x larger) summary
	// threshold: must still get the plain heartbeat wording, not summary.
	for i := 0; i < heartbeatGapThreshold; i++ {
		Record(&s, "distill", now)
	}

	tip, ok := GapTip(s, false, now)
	if !ok {
		t.Fatalf("expected a heartbeat tip to fire after %d unrated calls", s.Total)
	}
	if strings.Contains(tip, "Summary:") {
		t.Errorf("expected the plain heartbeat tip below the summary threshold, got: %q", tip)
	}
}

func TestGapTip_SummaryReminderTimeOnlyTrigger(t *testing.T) {
	start := time.Now()
	s := State{Calls: map[string]Invocation{}}
	Record(&s, "rate", start)

	spec, err := loadRemindersSpec()
	if err != nil {
		t.Fatalf("loadRemindersSpec: %v", err)
	}
	r := spec.Reminders[summaryReminderName]
	summaryIdle := time.Duration(r.ThresholdMultiplier * float64(rateGapIdleDuration()))

	later := start.Add(summaryIdle + time.Minute)
	for i := 0; i < 12; i++ { // stays well under any call-count threshold
		Record(&s, "distill", later)
	}

	tip, ok := GapTip(s, false, later)
	if !ok {
		t.Fatalf("expected a time-based summary tip after %v idle", later.Sub(start))
	}
	if !strings.Contains(tip, "Summary:") {
		t.Errorf("expected the summary reminder wording, got: %q", tip)
	}
}

func TestGapTip_SummaryReminderRespectsFeedbackDisabled(t *testing.T) {
	now := time.Now()
	s := State{Calls: map[string]Invocation{}}

	spec, err := loadRemindersSpec()
	if err != nil {
		t.Fatalf("loadRemindersSpec: %v", err)
	}
	r := spec.Reminders[summaryReminderName]
	summaryThreshold := int(r.ThresholdMultiplier * float64(rateGapThreshold))
	for i := 0; i < summaryThreshold; i++ {
		Record(&s, "distill", now)
	}
	Record(&s, "find", now) // clear the unrelated find-underuse heuristic

	if tip, ok := GapTip(s, true, now); ok {
		t.Errorf("expected feedbackDisabled=true to suppress the summary reminder too, got: %q", tip)
	}
	if _, ok := GapTip(s, false, now); !ok {
		t.Errorf("expected feedbackDisabled=false to still surface the summary tip for the same state")
	}
}
