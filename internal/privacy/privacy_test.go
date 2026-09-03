package privacy

import (
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{
		"":                LevelPublic,
		"public":          LevelPublic,
		"agent-sanitized": LevelAgentSanitized,
		"internal":        LevelInternal,
		"raw":             LevelRaw,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		if err != nil {
			t.Fatalf("ParseLevel(%q): unexpected error: %v", in, err)
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseLevel_Unknown(t *testing.T) {
	if _, err := ParseLevel("bogus"); err == nil {
		t.Fatal("ParseLevel(\"bogus\"): expected error, got nil")
	}
}

func TestLevel_StringRoundTrip(t *testing.T) {
	for _, l := range []Level{LevelPublic, LevelAgentSanitized, LevelInternal, LevelRaw} {
		got, err := ParseLevel(l.String())
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", l.String(), err)
		}
		if got != l {
			t.Errorf("round trip of %v via %q = %v", l, l.String(), got)
		}
	}
}

// TestScrubText_RedactsHomePathEmailAndToken constructs a fixture note
// containing a fake absolute home path, a fake email address, and a fake
// API-key-shaped token, and asserts ScrubText redacts all three in place
// while keeping the rest of the text intact (as opposed to dropping the
// field entirely, which is LevelPublic's behavior, not LevelInternal's).
func TestScrubText_RedactsHomePathEmailAndToken(t *testing.T) {
	raw := "Fixed bug in /home/testuser/projects/secret-client for someone@example.com using key sk-ABCDEFGHIJ1234567890abcdefghij"
	got := ScrubText(raw)

	for _, leaked := range []string{"/home/testuser", "testuser", "someone@example.com", "someone", "sk-ABCDEFGHIJ1234567890abcdefghij"} {
		if strings.Contains(got, leaked) {
			t.Errorf("ScrubText output still contains sensitive substring %q: %q", leaked, got)
		}
	}
	if !strings.Contains(got, "Fixed bug in") {
		t.Errorf("ScrubText dropped non-sensitive context, want it preserved: %q", got)
	}
	if !strings.Contains(got, "~") {
		t.Errorf("ScrubText did not replace home path with ~: %q", got)
	}
	if !strings.Contains(got, "[redacted-email]") {
		t.Errorf("ScrubText did not redact email: %q", got)
	}
	if !strings.Contains(got, "[redacted-token]") {
		t.Errorf("ScrubText did not redact token: %q", got)
	}
}

func TestScrubText_EmptyIsEmpty(t *testing.T) {
	if got := ScrubText(""); got != "" {
		t.Errorf("ScrubText(\"\") = %q, want \"\"", got)
	}
}
