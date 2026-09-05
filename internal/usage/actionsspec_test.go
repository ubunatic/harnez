package usage

import (
	"strings"
	"testing"
)

// TestEmbeddedActionsSpecIsValid guards the committed spec/actions.yaml
// itself: the binary embeds it via //go:embed (embed.go), and
// mustWatchActions() panics if it's malformed, so this is the test that
// would catch a broken spec at CI time instead of at first --watch launch.
func TestEmbeddedActionsSpecIsValid(t *testing.T) {
	wa, err := loadWatchActions()
	if err != nil {
		t.Fatalf("embedded spec/actions.yaml failed to load: %v", err)
	}
	if len(wa.spec.Actions) == 0 {
		t.Fatalf("expected at least one action in the embedded spec")
	}

	// Every box id watch.go actually renders must have a symbol.
	for _, box := range []string{"all_usage", "claude", "agy", "codex", "history", "processes", "load"} {
		if wa.boxTitleSymbol(box) == "" {
			t.Errorf("expected embedded spec to define a symbol for box %q", box)
		}
	}

	// mustWatchActions() must not panic against the real embedded spec.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("mustWatchActions() panicked against the embedded spec: %v", r)
		}
	}()
	mustWatchActions()
}

// TestParseWatchActionsYAMLMalformedFailsClearly is issue 132's "fail
// clearly, not panic" acceptance criterion: bad YAML must come back as a
// plain error, never a runtime panic.
func TestParseWatchActionsYAMLMalformedFailsClearly(t *testing.T) {
	_, err := parseWatchActionsYAML([]byte("actions: [this is not a map"))
	if err == nil {
		t.Fatalf("expected an error for malformed YAML, got nil")
	}
}

func TestParseWatchActionsYAMLValidatesRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "no actions",
			yaml: "actions: {}\n",
		},
		{
			name: "missing title",
			yaml: "actions:\n  quit:\n    keys: [\"q\"]\n    symbol: \"q\"\n    category: session\n",
		},
		{
			name: "missing keys",
			yaml: "actions:\n  quit:\n    title: Quit\n    symbol: \"q\"\n    category: session\n",
		},
		{
			name: "empty keys list",
			yaml: "actions:\n  quit:\n    title: Quit\n    keys: []\n    symbol: \"q\"\n    category: session\n",
		},
		{
			name: "missing symbol",
			yaml: "actions:\n  quit:\n    title: Quit\n    keys: [\"q\"]\n    category: session\n",
		},
		{
			name: "invalid category",
			yaml: "actions:\n  quit:\n    title: Quit\n    keys: [\"q\"]\n    symbol: \"q\"\n    category: bogus\n",
		},
		{
			name: "box category missing box id",
			yaml: "actions:\n  toggle_x:\n    title: X\n    keys: [\"1\"]\n    symbol: \"¹\"\n    category: box\n",
		},
		{
			name: "box category unknown box id",
			yaml: "actions:\n  toggle_x:\n    title: X\n    keys: [\"1\"]\n    symbol: \"¹\"\n    category: box\n    box: not_a_real_box\n",
		},
		{
			name: "non-box category with box id set",
			yaml: "actions:\n  quit:\n    title: Quit\n    keys: [\"q\"]\n    symbol: \"q\"\n    category: session\n    box: load\n",
		},
		{
			name: "unrecognized key token",
			yaml: "actions:\n  quit:\n    title: Quit\n    keys: [\"<bogus>\"]\n    symbol: \"q\"\n    category: session\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseWatchActionsYAML([]byte(c.yaml))
			if err == nil {
				t.Fatalf("expected a validation error for %q, got nil", c.name)
			}
		})
	}
}

func TestParseWatchActionsYAMLAcceptsAMinimalValidSpec(t *testing.T) {
	spec, err := parseWatchActionsYAML([]byte(`
actions:
  quit:
    title: Quit
    keys: ["q", "<c-c>", "<esc>"]
    symbol: "q"
    category: session
  toggle_load:
    title: Load
    keys: ["7"]
    symbol: "⁷"
    category: box
    box: load
`))
	if err != nil {
		t.Fatalf("expected a minimal valid spec to parse cleanly, got %v", err)
	}
	wa := buildWatchActions(spec)
	if wa.actionForKey('q') != "quit" {
		t.Errorf("expected 'q' to map to quit, got %q", wa.actionForKey('q'))
	}
	if wa.actionForKey(3) != "quit" {
		t.Errorf("expected Ctrl-C (byte 3) to map to quit, got %q", wa.actionForKey(3))
	}
	if wa.actionForKey(27) != "quit" {
		t.Errorf("expected Esc (byte 27) to map to quit, got %q", wa.actionForKey(27))
	}
	if wa.actionForKey('7') != "toggle_load" {
		t.Errorf("expected '7' to map to toggle_load, got %q", wa.actionForKey('7'))
	}
	if got := wa.boxTitleSymbol("load"); got != "⁷" {
		t.Errorf("expected load's title symbol to be ⁷, got %q", got)
	}
	if got := wa.actionForKey('z'); got != "" {
		t.Errorf("expected an unmapped key to return \"\", got %q", got)
	}
}

// TestDecodeSpecKey covers the control-token decoding used by both
// validation and dispatch.
func TestDecodeSpecKey(t *testing.T) {
	cases := []struct {
		tok  string
		want byte
		ok   bool
	}{
		{"<c-c>", 3, true},
		{"<esc>", 27, true},
		{"q", 'q', true},
		{"1", '1', true},
		{"", 0, false},
		{"<bogus>", 0, false},
		{"ab", 0, false},
	}
	for _, c := range cases {
		got, ok := decodeSpecKey(c.tok)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("decodeSpecKey(%q) = (%v, %v), want (%v, %v)", c.tok, got, ok, c.want, c.ok)
		}
	}
}

// TestSpecDrivenDispatchMatchesDocumentedKeys is a lockstep check between
// applyWatchSectionKey's box-toggle behavior and the embedded spec's key
// bindings: every box-category action's primary key must actually flip its
// documented watchSections field, proving watch.go reads dispatch key data
// from the spec rather than a separate hardcoded table that could drift.
func TestSpecDrivenDispatchMatchesDocumentedKeys(t *testing.T) {
	wa := mustWatchActions()

	boxField := map[string]func(watchSections) bool{
		"all_usage": func(s watchSections) bool { return s.AllUsage },
		"claude":    func(s watchSections) bool { return s.Claude },
		"agy":       func(s watchSections) bool { return s.AGY },
		"codex":     func(s watchSections) bool { return s.Codex },
		"history":   func(s watchSections) bool { return s.History },
		"processes": func(s watchSections) bool { return s.Processes },
		"load":      func(s watchSections) bool { return s.Load },
		"mic":       func(s watchSections) bool { return s.Mic },
	}

	for name, a := range wa.spec.Actions {
		if a.Category != "box" {
			continue
		}
		getField, ok := boxField[a.Box]
		if !ok {
			t.Fatalf("action %q names unhandled box %q", name, a.Box)
		}
		if len(a.Keys) == 0 {
			t.Fatalf("action %q has no keys", name)
		}
		key, ok := decodeSpecKey(a.Keys[0])
		if !ok {
			t.Fatalf("action %q key %q did not decode", name, a.Keys[0])
		}

		sec := defaultWatchSections()
		before := getField(sec)
		if !applyWatchSectionKey(&sec, key) {
			t.Fatalf("applyWatchSectionKey did not recognize key %q for action %q", a.Keys[0], name)
		}
		if getField(sec) == before {
			t.Errorf("expected key %q (action %q) to flip box %q, got unchanged: %+v", a.Keys[0], name, a.Box, sec)
		}
	}
}

// TestControlsOverlayDocumentsCollisionNote guards issue 132's acceptance
// criterion to document keys that changed to avoid collisions with the new
// numbered scheme: the old C/G/O/H/P/L letter toggles were dropped in favor
// of digits, and this must be visible in the overlay, not just a commit
// message.
func TestControlsOverlayDocumentsCollisionNote(t *testing.T) {
	plain := stripANSI(strings.Join(controlsOverlayLines(), "\n"))
	if !strings.Contains(plain, "132") {
		t.Errorf("expected overlay to reference issue 132's key-scheme change, got:\n%s", plain)
	}
}
