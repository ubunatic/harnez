package usage

import (
	"testing"

	"ubunatic.com/harnez/internal/rograph"
)

// TestEmbeddedColorsSpecIsValid guards the committed spec/colors.yaml
// itself: the binary embeds it via //go:embed (embed.go), and
// mustWatchColors() panics if it's malformed, so this is the test that
// would catch a broken spec at CI time instead of at first --watch launch.
func TestEmbeddedColorsSpecIsValid(t *testing.T) {
	spec, err := loadWatchColors()
	if err != nil {
		t.Fatalf("embedded spec/colors.yaml failed to load: %v", err)
	}
	if len(spec.Colors) == 0 {
		t.Fatalf("expected at least one color in the embedded spec")
	}
	if _, ok := spec.Colors["panel-bg"]; !ok {
		t.Fatalf("expected embedded spec to define \"panel-bg\"")
	}
	for _, name := range []string{"time-gauge-fg", "time-gauge-bg"} {
		if _, ok := spec.Colors[name]; !ok {
			t.Fatalf("expected embedded spec to define %q", name)
		}
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("mustWatchColors() panicked against the embedded spec: %v", r)
		}
	}()
	mustWatchColors()
}

func TestTimeGaugeColorsResolveIndependentlyFromSpec(t *testing.T) {
	spec := mustWatchColors()
	for _, name := range []string{"time-gauge-fg", "time-gauge-bg"} {
		if got, want := colorSGR(name), spec.Colors[name].SGR; got != want {
			t.Errorf("colorSGR(%q) = %q, want %q (from embedded spec)", name, got, want)
		}
	}
}

// TestParseWatchColorsYAMLMalformedFailsClearly mirrors issue 132's "fail
// clearly, not panic" acceptance criterion for the actions spec.
func TestParseWatchColorsYAMLMalformedFailsClearly(t *testing.T) {
	_, err := parseWatchColorsYAML([]byte("colors: [this is not a map"))
	if err == nil {
		t.Fatalf("expected an error for malformed YAML, got nil")
	}
}

func TestParseWatchColorsYAMLValidatesRequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		yaml  string
		valid bool
	}{
		{"no colors", "colors: {}\n", false},
		{"missing title", "colors:\n  panel-bg:\n    sgr: \"100\"\n", false},
		{"missing sgr", "colors:\n  accent:\n    title: Accent\n", false},
		{"missing time gauge background", "colors:\n  time-gauge-bg:\n    title: Time gauge background\n", true},
		{"empty panel background", "colors:\n  panel-bg:\n    title: Panel background\n    sgr: \"\"\n", true},
		{"null panel background", "colors:\n  panel-bg:\n    title: Panel background\n    sgr: null\n", true},
		{"empty time gauge background", "colors:\n  time-gauge-bg:\n    title: Time gauge background\n    sgr: \"\"\n", true},
		{"null time gauge background", "colors:\n  time-gauge-bg:\n    title: Time gauge background\n    sgr: null\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseWatchColorsYAML([]byte(c.yaml))
			if (err == nil) != c.valid {
				t.Fatalf("parseWatchColorsYAML(%q) error = %v, want valid=%t", c.name, err, c.valid)
			}
		})
	}
}

// TestPanelBackgroundSGRResolvesFromSpec proves the value actually comes
// from spec/colors.yaml rather than a hardcoded fallback: issue 136's fix
// changes the "100" default in rograph from a Go literal to a spec-sourced
// value, and mustWatchColors/colorSGR is the only path there.
func TestPanelBackgroundSGRResolvesFromSpec(t *testing.T) {
	got := colorSGR("panel-bg")
	spec := mustWatchColors()
	want := spec.Colors["panel-bg"].SGR
	if got != want {
		t.Errorf("colorSGR(\"panel-bg\") = %q, want %q (from embedded spec)", got, want)
	}
	if got == "" {
		t.Errorf("expected a non-empty SGR code for panel-bg")
	}
}

// TestPanelBackgroundSGRMissingColorPanics guards the "broken build, not a
// runtime condition" contract: an unknown color name panics rather than
// silently falling back to something unspecced.
func TestPanelBackgroundSGRMissingColorPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected colorSGR to panic for an unknown color name")
		}
	}()
	colorSGR("not-a-real-color")
}

// TestInitWiresRographDefaultFromSpec is the layering-decision proof: usage
// package init() must have already pushed spec/colors.yaml's "panel-bg"
// value into rograph.DefaultBackgroundANSI by the time any test in this
// package (or the binary) runs, since rograph itself has no way to read the
// spec.
func TestInitWiresRographDefaultFromSpec(t *testing.T) {
	want := colorSGR("panel-bg")
	if rograph.DefaultBackgroundANSI != want {
		t.Errorf("rograph.DefaultBackgroundANSI = %q, want %q (spec/colors.yaml panel-bg)", rograph.DefaultBackgroundANSI, want)
	}
}
