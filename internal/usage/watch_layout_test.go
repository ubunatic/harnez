package usage

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// Issue 690: terminal layout measures display width, not runes; a wide rune never pushes a
// truncated string past its limit.
func TestTruncateVisibleKeepsDisplayWidth(t *testing.T) {
	for _, tt := range []struct {
		in   string
		max  int
		want string
	}{
		{"🤖🤖🤖🤖🤖🤖", 5, "🤖..."},
		{"🤖🤖🤖🤖🤖🤖", 6, "🤖..."},
		{"模型模型模型", 8, "模型..."},
		{"abcdefgh", 6, "abc..."},
	} {
		got := truncateVisible(tt.in, tt.max)
		if stripANSI(got) != tt.want || visLen(got) > tt.max {
			t.Errorf("truncateVisible(%q, %d) = %q (width %d); want %q", tt.in, tt.max, stripANSI(got), visLen(got), tt.want)
		}
	}
	if got := visLen("🤖模型ab"); got != 8 {
		t.Errorf("visLen = %d; want 8", got)
	}
}

// Issue 690 done-when: a CJK model name keeps the usage table columns aligned.
func TestUsageTableAlignsCJKModelName(t *testing.T) {
	agents := usageTableFixture()
	agents[1].ActiveModel = "模型测试"
	lines := usageTableLines(agents, nil, true, testTime)
	header := stripANSI(lines[0])
	want := runewidth.StringWidth(header[:strings.Index(header, "Account")])
	row := stripANSI(lines[1])
	if !strings.Contains(row, "模型测试") {
		t.Fatalf("claude row lost the model: %q", row)
	}
	if got := runewidth.StringWidth(row[:strings.LastIndex(row, "Pro")]); got != want {
		t.Fatalf("Account column starts at display column %d, header at %d:\n%s\n%s", got, want, header, row)
	}
}
