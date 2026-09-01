package find

import "testing"

func TestParseQuery_PlainAND(t *testing.T) {
	q, err := ParseQuery("vram gtt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.Filters) != 0 {
		t.Fatalf("expected no filters, got %v", q.Filters)
	}
	if len(q.Groups) != 2 {
		t.Fatalf("expected 2 AND groups, got %d", len(q.Groups))
	}
	if q.Groups[0].Alternatives[0] != "vram" || q.Groups[1].Alternatives[0] != "gtt" {
		t.Fatalf("unexpected groups: %+v", q.Groups)
	}
}

func TestParseQuery_OR(t *testing.T) {
	q, err := ParseQuery("vram|gtt memory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(q.Groups))
	}
	if len(q.Groups[0].Alternatives) != 2 {
		t.Fatalf("expected first group to have 2 OR alternatives, got %+v", q.Groups[0])
	}
	if q.Groups[0].Alternatives[0] != "vram" || q.Groups[0].Alternatives[1] != "gtt" {
		t.Fatalf("unexpected alternatives: %+v", q.Groups[0].Alternatives)
	}
	if q.Groups[1].Alternatives[0] != "memory" {
		t.Fatalf("unexpected second group: %+v", q.Groups[1])
	}
}

func TestParseQuery_FieldAliasesAndValues(t *testing.T) {
	for _, tc := range []struct {
		field, value string
	}{
		{"status", "open"}, {"is", "open"},
		{"status", "in-progress"}, {"status", "blocked"},
		{"status", "closed"}, {"status", "draft"},
		{"STATUS", "OPEN"}, {"Is", "Closed"},
	} {
		q, err := ParseQuery(tc.field + ":" + tc.value)
		if err != nil {
			t.Fatalf("%s:%s: unexpected error: %v", tc.field, tc.value, err)
		}
		if len(q.Filters) != 1 {
			t.Fatalf("%s:%s: expected 1 filter, got %+v", tc.field, tc.value, q.Filters)
		}
	}
}

func TestParseQuery_PrecedenceFieldPlusOR(t *testing.T) {
	q, err := ParseQuery("status:open vram|gtt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.Filters) != 1 || q.Filters[0].Value != StatusOpenValue {
		t.Fatalf("expected 1 status:open filter, got %+v", q.Filters)
	}
	if len(q.Groups) != 1 || len(q.Groups[0].Alternatives) != 2 {
		t.Fatalf("expected 1 OR group with 2 alternatives, got %+v", q.Groups)
	}
}

func TestParseQuery_MalformedCases(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"|",
		"|vram",
		"vram|",
		"a||b",
		"status:open|closed",
		"is:open|closed",
		"(vram)",
		`"vram"`,
		"'vram'",
		"-vram",
		"!vram",
		"status:",
		"is:",
		"foo:bar",
		"status:archived",
		"is:wip",
	}
	for _, c := range cases {
		if _, err := ParseQuery(c); err == nil {
			t.Errorf("ParseQuery(%q): expected usage error, got nil", c)
		}
	}
}

func TestParseQuery_ColonInPlainTextIsNotAFilter(t *testing.T) {
	// A bare digit-led "colon" term (e.g. a clock-like token) is not a
	// recognized field:value filter, since the field name must be
	// alphabetic; it is parsed as ordinary OR-free ungrouped text.
	q, err := ParseQuery("3:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.Filters) != 0 {
		t.Fatalf("expected no filters, got %+v", q.Filters)
	}
	if len(q.Groups) != 1 || q.Groups[0].Alternatives[0] != "3:00" {
		t.Fatalf("expected one bare-text group, got %+v", q.Groups)
	}
}
