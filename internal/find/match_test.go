package find

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Time-Gauge", "time gauge"},
		{"  VRAM/GTT  memory ", "vram gtt memory"},
		{"`harnez find`", "harnez find"},
		{"CAFÉ", "café"},
		{"", ""},
		{"a   b\tc\nd", "a b c d"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDamerauLevenshtein_Transposition(t *testing.T) {
	// "vrma" -> "vram" is a single adjacent transposition; true
	// Damerau-Levenshtein counts it as one edit (plain Levenshtein would
	// count it as two: a substitution plus another substitution, or two
	// substitutions via a delete+insert path).
	if d := damerauLevenshtein("vrma", "vram"); d != 1 {
		t.Errorf("damerauLevenshtein(vrma, vram) = %d, want 1", d)
	}
	if d := damerauLevenshtein("vram", "vram"); d != 0 {
		t.Errorf("damerauLevenshtein(vram, vram) = %d, want 0", d)
	}
	if d := damerauLevenshtein("kitten", "sitting"); d != 3 {
		t.Errorf("damerauLevenshtein(kitten, sitting) = %d, want 3", d)
	}
}

func TestFuzzBudget_LengthThresholds(t *testing.T) {
	cases := []struct {
		tokenLen int
		want     int
	}{
		{1, 0}, {4, 0}, {5, 1}, {8, 1}, {9, 2}, {20, 2},
	}
	for _, c := range cases {
		if got := fuzzBudget(c.tokenLen); got != c.want {
			t.Errorf("fuzzBudget(%d) = %d, want %d", c.tokenLen, got, c.want)
		}
	}
}

func TestAlternativeClass_ShortTokenProtection(t *testing.T) {
	// "gtt" (3 chars) gets zero typo tolerance: a one-edit-away field
	// token like "gt" or "ott" must NOT match it, protecting short
	// identifiers from noisy fuzzy expansion.
	if c := alternativeClass(Normalize("gtt"), Normalize("ott memory"), ""); c != ClassNone {
		t.Errorf("expected no match for short token typo, got class %d", c)
	}
	// But an exact short token still matches.
	if c := alternativeClass(Normalize("gtt"), Normalize("vram and gtt load"), ""); c != ClassTitleExact {
		t.Errorf("expected exact substring match, got class %d", c)
	}
}

func TestAlternativeClass_FuzzyThresholds(t *testing.T) {
	// "diagnostcs" (10 chars, budget 2) is 1 transposition + context away
	// from "diagnostics"; should fuzzy-match as a token.
	title := Normalize("LSP diagnostics noise")
	if c := alternativeClass(Normalize("diagnostcs"), title, ""); c != ClassTitleFuzzy {
		t.Errorf("expected title fuzzy match (class %d), got %d", ClassTitleFuzzy, c)
	}
}

func TestAlternativeClass_TitleBeatsBody(t *testing.T) {
	title := Normalize("VRAM memory panel")
	body := Normalize("some unrelated body text")
	if c := alternativeClass(Normalize("vram"), title, body); c != ClassTitleExact {
		t.Errorf("expected ClassTitleExact, got %d", c)
	}

	title2 := Normalize("unrelated title")
	body2 := Normalize("mentions vram deep in the body")
	if c := alternativeClass(Normalize("vram"), title2, body2); c != ClassBodyExact {
		t.Errorf("expected ClassBodyExact, got %d", c)
	}
}

func TestAlternativeClass_PrefixMatch(t *testing.T) {
	title := Normalize("vram and gtt combined load panel")
	// Each token of "vra pane" prefix-matches a distinct title token
	// ("vra"->"vram", "pane"->"panel"), but the two-word alternative
	// itself never appears contiguously in the title, so this can only
	// resolve via the multi-token prefix path (class 2), not the
	// exact-substring path (class 1).
	if c := alternativeClass(Normalize("vra pane"), title, ""); c != ClassTitlePrefix {
		t.Errorf("expected ClassTitlePrefix, got %d", c)
	}
}

func TestGroupClass_BestOfAlternatives(t *testing.T) {
	title := Normalize("vram panel")
	body := Normalize("mentions gtt somewhere in the body")
	g := Group{Alternatives: []string{"gtt", "vram"}}
	if c := groupClass(g, title, body); c != ClassTitleExact {
		t.Errorf("expected group to retain best alternative (ClassTitleExact), got %d", c)
	}
}

func TestGroupClass_NoAlternativeMatches(t *testing.T) {
	g := Group{Alternatives: []string{"zzzzzz"}}
	if c := groupClass(g, Normalize("vram panel"), Normalize("gtt body")); c != ClassNone {
		t.Errorf("expected ClassNone, got %d", c)
	}
}
