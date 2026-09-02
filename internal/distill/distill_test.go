package distill

import (
	"fmt"
	"strings"
	"testing"
)

// syntheticGoTestOutput builds a `go test -v` transcript with n passing
// tests and one failure, roughly matching real-world verbose test output.
func syntheticGoTestOutput(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("TestCase%d", i)
		fmt.Fprintf(&b, "=== RUN   %s\n--- PASS: %s (0.00s)\n", name, name)
	}
	b.WriteString("=== RUN   TestBroken\n--- FAIL: TestBroken (0.01s)\n    broken_test.go:42: expected 1, got 2\nFAIL\n")
	fmt.Fprintf(&b, "FAIL\texample.com/pkg\t%0.3fs\n", float64(n)*0.001)
	return b.String()
}

// BenchmarkDistill_GoTest reports the byte-size reduction (a rough proxy for
// token-count reduction, ~4 bytes/token) of distilling a verbose `go test`
// run with mostly-passing tests.
func BenchmarkDistill_GoTest(b *testing.B) {
	raw := syntheticGoTestOutput(500)
	var distilled string
	for i := 0; i < b.N; i++ {
		distilled = Distill(raw, Options{MaxLines: 0})
	}
	b.ReportMetric(float64(len(raw)), "raw-bytes")
	b.ReportMetric(float64(len(distilled)), "distilled-bytes")
	b.ReportMetric(float64(len(raw))/float64(len(distilled)), "reduction-x")
}

func TestStripANSI(t *testing.T) {
	in := "\x1b[32mPASS\x1b[0m: \x1b[1mok\x1b[0m"
	want := "PASS: ok"
	if got := StripANSI(in); got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", in, got, want)
	}
}

func TestFilterGoTest(t *testing.T) {
	in := `=== RUN   TestFoo
--- PASS: TestFoo (0.00s)
=== RUN   TestBar
--- FAIL: TestBar (0.01s)
    bar_test.go:12: expected 1, got 2
FAIL
FAIL	example.com/pkg	0.02s
=== RUN   TestBaz
--- PASS: TestBaz (0.00s)
ok  	example.com/other	0.01s`

	got := FilterGoTest(strings.NewReader(in))

	if strings.Contains(got, "TestFoo") {
		t.Errorf("expected passing TestFoo to be dropped, got:\n%s", got)
	}
	if strings.Contains(got, "TestBaz") {
		t.Errorf("expected passing TestBaz to be dropped, got:\n%s", got)
	}
	if !strings.Contains(got, "--- FAIL: TestBar") {
		t.Errorf("expected failing TestBar to be kept, got:\n%s", got)
	}
	if !strings.Contains(got, "bar_test.go:12") {
		t.Errorf("expected failure detail line to be kept, got:\n%s", got)
	}
	if !strings.Contains(got, "FAIL\texample.com/pkg") {
		t.Errorf("expected package FAIL summary to be kept, got:\n%s", got)
	}
	if !strings.Contains(got, "ok  \texample.com/other") {
		t.Errorf("expected package ok summary to be kept, got:\n%s", got)
	}
}

func TestFilterGoTest_BuildError(t *testing.T) {
	in := `# example.com/pkg
./foo.go:10:2: undefined: bar
FAIL	example.com/pkg [build failed]`

	got := FilterGoTest(strings.NewReader(in))
	if got != in {
		t.Errorf("build error output should pass through unchanged:\ngot:\n%s\nwant:\n%s", got, in)
	}
}

func TestFilterGit(t *testing.T) {
	in := `On branch main
Untracked files:
  (use "git add <file>..." to include in what will be committed)
	a.txt
	b.txt
	c.txt

nothing added to commit`

	got := FilterGit(strings.NewReader(in))
	if !strings.Contains(got, "[3 untracked files omitted]") {
		t.Errorf("expected untracked files collapsed, got:\n%s", got)
	}
	if strings.Contains(got, "a.txt") {
		t.Errorf("expected untracked filenames dropped, got:\n%s", got)
	}
	if !strings.Contains(got, "On branch main") {
		t.Errorf("expected leading context kept, got:\n%s", got)
	}
	if !strings.Contains(got, "nothing added to commit") {
		t.Errorf("expected trailing context kept, got:\n%s", got)
	}
}

func TestFilterDeduplicate(t *testing.T) {
	in := "warn: retry\nwarn: retry\nwarn: retry\nok\nwarn: retry\n"
	got := FilterDeduplicate(strings.NewReader(in))
	want := "[x3] warn: retry\nok\nwarn: retry"
	if got != want {
		t.Errorf("FilterDeduplicate() = %q, want %q", got, want)
	}
}

func TestFilterDeduplicate_NoRepeats(t *testing.T) {
	in := "a\nb\nc"
	got := FilterDeduplicate(strings.NewReader(in))
	if got != in {
		t.Errorf("FilterDeduplicate() = %q, want unchanged %q", got, in)
	}
}

func TestFilterHeadTail(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	got := FilterHeadTail(lines, 10)
	gotLines := strings.Split(got, "\n")

	if len(gotLines) != 11 {
		t.Fatalf("expected 11 output lines (10 + omission marker), got %d:\n%s", len(gotLines), got)
	}
	if gotLines[5] != "[... 10 lines omitted ...]" {
		t.Errorf("expected omission marker in the middle, got %q", gotLines[5])
	}
}

func TestFilterHeadTail_UnderLimit(t *testing.T) {
	lines := []string{"a", "b", "c"}
	got := FilterHeadTail(lines, 10)
	if got != "a\nb\nc" {
		t.Errorf("FilterHeadTail() = %q, want unchanged", got)
	}
}

func TestFilterHeadTailBytes(t *testing.T) {
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, fmt.Sprintf("line %03d: some build log content here", i))
	}
	in := strings.Join(lines, "\n")

	maxBytes := 2000
	got := FilterHeadTailBytes(in, maxBytes)

	if len(got) >= len(in) {
		t.Fatalf("expected output shorter than input (%d bytes); got %d bytes >= input %d bytes", maxBytes, len(got), len(in))
	}
	if !strings.Contains(got, truncationSentinel) {
		t.Fatalf("expected truncation sentinel %q in output, got:\n%s", truncationSentinel, got)
	}
	if !strings.HasPrefix(got, "line 000:") {
		t.Errorf("expected the head to be preserved, got prefix %q", got[:20])
	}
	if !strings.HasSuffix(got, "line 499: some build log content here") {
		t.Errorf("expected the tail to be preserved, got suffix %q", got[len(got)-40:])
	}
	// The note must state how many bytes were omitted and the cap applied.
	if !strings.Contains(got, fmt.Sprintf("%d-byte cap", maxBytes)) {
		t.Errorf("expected note to mention the %d-byte cap, got:\n%s", maxBytes, got)
	}
}

func TestFilterHeadTailBytes_UnderLimit(t *testing.T) {
	in := "short output\nwith a few lines\n"
	got := FilterHeadTailBytes(in, 10_000)
	if got != in {
		t.Errorf("FilterHeadTailBytes() = %q, want unchanged", got)
	}
}

func TestFilterHeadTailBytes_Disabled(t *testing.T) {
	in := strings.Repeat("x", 5000)
	got := FilterHeadTailBytes(in, 0)
	if got != in {
		t.Errorf("FilterHeadTailBytes() with maxBytes=0 = %q, want unchanged input", got)
	}
}

func TestDistill_MaxBytesCap(t *testing.T) {
	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, fmt.Sprintf("unique log line %04d with enough content to add up in bytes", i))
	}
	in := strings.Join(lines, "\n")

	got := Distill(in, Options{Mode: ModeRaw, MaxLines: 0, MaxBytes: 1000, NoDedup: true})
	if len(got) > 1000+len(truncationSentinel)+80 {
		t.Fatalf("expected Distill to respect MaxBytes cap, got %d bytes:\n%s", len(got), got)
	}
	if !strings.Contains(got, truncationSentinel) {
		t.Errorf("expected Distill output to contain the truncation sentinel, got:\n%s", got)
	}
}

func TestDetectMode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Mode
	}{
		{"gotest", "=== RUN   TestFoo\n--- PASS: TestFoo", ModeGoTest},
		{"git", "On branch main\nnothing to commit", ModeGit},
		{"raw", "hello world", ModeRaw},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectMode(c.in); got != c.want {
				t.Errorf("DetectMode(%q) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

func TestDetectModeFromArgs(t *testing.T) {
	cases := []struct {
		args []string
		want Mode
	}{
		{[]string{"go", "test", "./..."}, ModeGoTest},
		{[]string{"git", "status"}, ModeGit},
		{[]string{"ls", "-la"}, ModeRaw},
		{nil, ModeRaw},
	}
	for _, c := range cases {
		if got := DetectModeFromArgs(c.args); got != c.want {
			t.Errorf("DetectModeFromArgs(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestDistill_EndToEnd(t *testing.T) {
	in := "\x1b[32m=== RUN   TestFoo\x1b[0m\n--- PASS: TestFoo (0.00s)\n--- FAIL: TestBar (0.00s)\n    oops\nFAIL"
	got := Distill(in, Options{MaxLines: 100})

	if strings.Contains(got, "\x1b[") {
		t.Errorf("expected ANSI codes stripped, got:\n%s", got)
	}
	if strings.Contains(got, "TestFoo") {
		t.Errorf("expected passing test dropped, got:\n%s", got)
	}
	if !strings.Contains(got, "TestBar") {
		t.Errorf("expected failing test kept, got:\n%s", got)
	}
}

func TestDistill_RawModeSkipsStructuredFilter(t *testing.T) {
	in := "hello\nhello\nworld"
	got := Distill(in, Options{Mode: ModeRaw})
	want := "[x2] hello\nworld"
	if got != want {
		t.Errorf("Distill() = %q, want %q", got, want)
	}
}

func TestDistill_NoDedup(t *testing.T) {
	in := "hello\nhello\nworld"
	got := Distill(in, Options{Mode: ModeRaw, NoDedup: true})
	if got != in {
		t.Errorf("Distill() with NoDedup = %q, want unchanged %q", got, in)
	}
}
