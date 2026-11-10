package usage

import (
	"testing"
)

func TestTruncateVisible(t *testing.T) {
	s := "🤖🤖🤖🤖🤖🤖"
	got := truncateVisible(s, 5)
	if visLen(got) != 5 {
		t.Errorf("truncateVisible(%q, 5) = %q (visLen %d), want visLen 5", s, got, visLen(got))
	}
}

func TestVisLen(t *testing.T) {
	s := "🤖"
	if got := visLen(s); got != 2 {
		t.Errorf("visLen(%q) = %d, want 2", s, got)
	}
}
