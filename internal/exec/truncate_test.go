package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// truncateSpec is the spec/exec.yaml truncate block used by the unit tests.
var truncateSpec = TruncateSpec{Enabled: true, MaxOutputBytes: 51200, MaxOutputLines: 2000, SpillPrefix: "harnez-exec-"}

// envMap returns a Getenv that serves only the given variables.
func envMap(vals map[string]string) func(string) string {
	return func(name string) string { return vals[name] }
}

// noTTY and tty are IsTerminal seams for the resolver tests.
func noTTY(io.Writer) bool { return false }
func tty(io.Writer) bool   { return true }

func TestResolveTruncateSettings_Precedence(t *testing.T) {
	cases := []struct {
		name     string
		opts     execOptions
		args     []string
		env      map[string]string
		spec     TruncateSpec
		isTTY    func(io.Writer) bool
		wantOn   bool
		wantByte int64
	}{
		{name: "spec default on a pipe", spec: truncateSpec, isTTY: noTTY, wantOn: true, wantByte: 51200},
		{name: "spec disabled", spec: TruncateSpec{Enabled: false, MaxOutputBytes: 51200}, isTTY: noTTY, wantOn: false, wantByte: 51200},
		{name: "env bytes beats spec", spec: truncateSpec, env: map[string]string{maxOutputBytesEnv: "100"}, isTTY: noTTY, wantOn: true, wantByte: 100},
		{name: "prefix bytes beats env", spec: truncateSpec, env: map[string]string{maxOutputBytesEnv: "100"}, args: []string{"sh", "-c", "HARNEZ_EXEC_MAX_OUTPUT_BYTES=200 make test"}, isTTY: noTTY, wantOn: true, wantByte: 200},
		{name: "flag beats prefix and env", opts: execOptions{MaxOutputBytes: 300, MaxOutputBytesSet: true}, spec: truncateSpec, env: map[string]string{maxOutputBytesEnv: "100"}, args: []string{"sh", "-c", "HARNEZ_EXEC_MAX_OUTPUT_BYTES=200 make test"}, isTTY: noTTY, wantOn: true, wantByte: 300},
		{name: "flag zero disables", opts: execOptions{MaxOutputBytes: 0, MaxOutputBytesSet: true}, spec: truncateSpec, isTTY: noTTY, wantOn: false, wantByte: 0},
		{name: "env zero disables", spec: truncateSpec, env: map[string]string{maxOutputBytesEnv: "0"}, isTTY: noTTY, wantOn: false, wantByte: 0},
		{name: "prefix truncate=0 disables", spec: truncateSpec, args: []string{"sh", "-c", "HARNEZ_EXEC_TRUNCATE=0 make test"}, isTTY: noTTY, wantOn: false, wantByte: 51200},
		{name: "env truncate=false disables", spec: truncateSpec, env: map[string]string{truncateEnv: "False"}, isTTY: noTTY, wantOn: false, wantByte: 51200},
		{name: "env truncate=1 forces on spec-disabled", spec: TruncateSpec{Enabled: false, MaxOutputBytes: 51200}, env: map[string]string{truncateEnv: "1"}, isTTY: noTTY, wantOn: true, wantByte: 51200},
		{name: "tty skips by default", spec: truncateSpec, isTTY: tty, wantOn: false, wantByte: 51200},
		{name: "tty forced by env truncate=1", spec: truncateSpec, env: map[string]string{truncateEnv: "1"}, isTTY: tty, wantOn: true, wantByte: 51200},
		{name: "tty forced by env truncate=true", spec: truncateSpec, env: map[string]string{truncateEnv: "true"}, isTTY: tty, wantOn: true, wantByte: 51200},
		{name: "tty forced by flag", opts: execOptions{MaxOutputBytes: 4096, MaxOutputBytesSet: true}, spec: truncateSpec, isTTY: tty, wantOn: true, wantByte: 4096},
		{name: "tty forced by prefix bytes", spec: truncateSpec, args: []string{"sh", "-c", "HARNEZ_EXEC_MAX_OUTPUT_BYTES=4096 make test"}, isTTY: tty, wantOn: true, wantByte: 4096},
		{name: "prefix truncate=1 beats env truncate=0", spec: truncateSpec, env: map[string]string{truncateEnv: "0"}, args: []string{"sh", "-c", "HARNEZ_EXEC_TRUNCATE=1 make test"}, isTTY: tty, wantOn: true, wantByte: 51200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			opts.Getenv = envMap(tc.env)
			opts.IsTerminal = tc.isTTY
			cfg, err := resolveTruncateSettings(opts, tc.args, tc.spec, io.Discard)
			if err != nil {
				t.Fatalf("resolveTruncateSettings: %v", err)
			}
			if cfg.Enabled != tc.wantOn {
				t.Errorf("Enabled = %v, want %v", cfg.Enabled, tc.wantOn)
			}
			if cfg.MaxBytes != tc.wantByte {
				t.Errorf("MaxBytes = %d, want %d", cfg.MaxBytes, tc.wantByte)
			}
			if cfg.MaxLines != tc.spec.MaxOutputLines {
				t.Errorf("MaxLines = %d, want spec %d", cfg.MaxLines, tc.spec.MaxOutputLines)
			}
		})
	}
}

func TestResolveTruncateSettings_RejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		opts execOptions
		env  map[string]string
	}{
		{name: "negative flag", opts: execOptions{MaxOutputBytes: -1, MaxOutputBytesSet: true}},
		{name: "non-numeric env bytes", env: map[string]string{maxOutputBytesEnv: "50k"}},
		{name: "negative env bytes", env: map[string]string{maxOutputBytesEnv: "-3"}},
		{name: "unknown truncate value", env: map[string]string{truncateEnv: "maybe"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			opts.Getenv = envMap(tc.env)
			opts.IsTerminal = noTTY
			if _, err := resolveTruncateSettings(opts, nil, truncateSpec, io.Discard); err == nil {
				t.Fatal("resolveTruncateSettings accepted an invalid value")
			}
		})
	}
}

func TestResolveTruncateSettings_DefaultsToTerminalCheckOnFile(t *testing.T) {
	// A *os.File that is not a terminal (a regular file) is not skipped.
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := resolveTruncateSettings(execOptions{Getenv: envMap(nil)}, nil, truncateSpec, f)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled {
		t.Fatal("regular file was treated as a terminal")
	}
}

// capFor builds a cap with small limits and a spill dir under dir.
func capFor(t *testing.T, maxBytes int64, maxLines int, mirrored bool) (*outputCap, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := truncateSettings{Enabled: true, MaxBytes: maxBytes, MaxLines: maxLines, SpillDir: dir, SpillPrefix: "harnez-exec-"}
	return newOutputCap(cfg, mirrored), dir
}

func TestOutputCap_ForwardsWithinBudgetAndSpillsRemainder(t *testing.T) {
	c, dir := capFor(t, 10, 0, false)
	var shown bytes.Buffer
	w := c.writer(&shown)
	full := []byte("0123456789ABCDEFGHIJ")
	for _, chunk := range [][]byte{full[:4], full[4:12], full[12:]} {
		if n, err := w.Write(chunk); err != nil || n != len(chunk) {
			t.Fatalf("Write(%q) = %d, %v", chunk, n, err)
		}
	}
	if shown.String() != "0123456789" {
		t.Fatalf("shown = %q, want first 10 bytes", shown.String())
	}
	path, err := c.settle(0, "")
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if filepath.Dir(path) != dir || !strings.HasPrefix(filepath.Base(path), "harnez-exec-") {
		t.Fatalf("spill path %q not an harnez-exec- file in %q", path, dir)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(full) {
		t.Fatalf("spill = %q, want verbatim full stream %q", got, full)
	}
	if c.shownBytes() != 10 {
		t.Fatalf("shownBytes = %d, want 10", c.shownBytes())
	}
}

func TestOutputCap_NoSpillWhenWithinLimit(t *testing.T) {
	c, dir := capFor(t, 64, 3, false)
	var shown bytes.Buffer
	w := c.writer(&shown)
	if _, err := w.Write([]byte("a\nb\nc\n")); err != nil {
		t.Fatal(err)
	}
	path, err := c.settle(0, "")
	if err != nil || path != "" || c.truncated() {
		t.Fatalf("settle = %q, %v, truncated=%v; want no spill", path, err, c.truncated())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("spill dir has %d entries, want none", len(entries))
	}
	if shown.String() != "a\nb\nc\n" {
		t.Fatalf("shown = %q", shown.String())
	}
}

func TestOutputCap_LineLimitStopsAfterNthNewline(t *testing.T) {
	c, _ := capFor(t, 1<<20, 2, false)
	var shown bytes.Buffer
	w := c.writer(&shown)
	if _, err := w.Write([]byte("one\ntwo\nthree\nfour")); err != nil {
		t.Fatal(err)
	}
	if shown.String() != "one\ntwo\n" {
		t.Fatalf("shown = %q, want two lines", shown.String())
	}
	if !c.truncated() || !c.byLines {
		t.Fatalf("truncated=%v byLines=%v, want both true", c.truncated(), c.byLines)
	}
	if got := c.notice("P"); got != "[Full output: P. Truncated: 2 lines shown (2 lines limit)]\n" {
		t.Fatalf("notice = %q", got)
	}
}

func TestOutputCap_ExactFitIsNotTruncated(t *testing.T) {
	c, _ := capFor(t, 4, 0, false)
	var shown bytes.Buffer
	if _, err := c.writer(&shown).Write([]byte("abcd")); err != nil {
		t.Fatal(err)
	}
	if c.truncated() {
		t.Fatal("output exactly at the byte limit was reported truncated")
	}
}

func TestOutputCap_NoticeStartsOnItsOwnLine(t *testing.T) {
	c, _ := capFor(t, 4, 0, false)
	var shown bytes.Buffer
	if _, err := c.writer(&shown).Write([]byte("abcdefg")); err != nil {
		t.Fatal(err)
	}
	if got := c.notice("P"); got != "\n[Full output: P. Truncated: 1 lines shown (0.0KB limit)]\n" {
		t.Fatalf("notice = %q", got)
	}
	c2, _ := capFor(t, 4, 0, false)
	var shown2 bytes.Buffer
	if _, err := c2.writer(&shown2).Write([]byte("ab\ncdefg")); err != nil {
		t.Fatal(err)
	}
	if got := c2.notice("P"); got != "\n[Full output: P. Truncated: 2 lines shown (0.0KB limit)]\n" {
		t.Fatalf("notice = %q", got)
	}
	c3, _ := capFor(t, 6, 0, false)
	var shown3 bytes.Buffer
	if _, err := c3.writer(&shown3).Write([]byte("ab\ncd\nefg\n")); err != nil {
		t.Fatal(err)
	}
	if got := c3.notice("P"); got != "[Full output: P. Truncated: 2 lines shown (0.0KB limit)]\n" {
		t.Fatalf("notice after newline = %q", got)
	}
}

func TestOutputCap_CutDoesNotSplitRune(t *testing.T) {
	c, _ := capFor(t, 4, 0, false)
	var shown bytes.Buffer
	// "é" is two bytes; a cut at byte 4 would split the second é.
	if _, err := c.writer(&shown).Write([]byte("aéé")); err != nil {
		t.Fatal(err)
	}
	if shown.String() != "aé" {
		t.Fatalf("shown = %q, want rune-aligned %q", shown.String(), "aé")
	}
}

// A rune-boundary back-off leaves room for a small follow-up write. That write
// must still go to the spill, not to dst, and the spill must keep every byte
// in order.
func TestOutputCap_FollowUpAfterRuneCutSpillsInOrder(t *testing.T) {
	c, _ := capFor(t, 4, 0, false)
	var shown bytes.Buffer
	w := c.writer(&shown)
	// "aéé" is 5 bytes. The cap cuts at "aé" (3 bytes), so 1 byte of room is
	// left. The writes that follow must not use that room.
	for _, chunk := range []string{"aéé", "x", "yz"} {
		if n, err := w.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("Write(%q) = %d, %v", chunk, n, err)
		}
	}
	if shown.String() != "aé" {
		t.Fatalf("shown = %q, want %q (nothing after the cut)", shown.String(), "aé")
	}
	path, err := c.settle(0, "")
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "aéé"+"xyz" {
		t.Fatalf("spill = %q, want full stream %q", got, "aéé"+"xyz")
	}
	if c.shownBytes() != int64(len("aé")) {
		t.Fatalf("shownBytes = %d, want %d", c.shownBytes(), len("aé"))
	}
}

// A failed adopt must not leave an empty or partial spill file behind.
func TestOutputCap_AdoptFailureLeavesNoSpill(t *testing.T) {
	c, dir := capFor(t, 4, 0, false)
	if _, err := c.adoptQuotaLog(filepath.Join(dir, "missing-quota.log")); err == nil {
		t.Fatal("adoptQuotaLog of a missing log succeeded")
	}
	if got := spillFiles(t, dir); len(got) != 0 {
		t.Fatalf("failed adopt left %v, want none", got)
	}
}

func TestOutputCap_MirroredWritesNoSpill(t *testing.T) {
	c, dir := capFor(t, 4, 0, true)
	var shown bytes.Buffer
	if _, err := c.writer(&shown).Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	if !c.truncated() {
		t.Fatal("mirrored cap did not withhold output")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("mirrored cap created %d spill files, want none", len(entries))
	}
}

func TestOutputCap_SettleQuotaLogPassAdoptsAsSpill(t *testing.T) {
	c, dir := capFor(t, 4, 0, true)
	quotaLog := filepath.Join(t.TempDir(), "quota.log")
	if err := os.WriteFile(quotaLog, []byte("abcdefgh"), 0644); err != nil {
		t.Fatal(err)
	}
	var shown bytes.Buffer
	if _, err := c.writer(&shown).Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	path, err := c.settle(0, quotaLog)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("spill %q not in %q", path, dir)
	}
	if got, _ := os.ReadFile(path); string(got) != "abcdefgh" {
		t.Fatalf("adopted spill = %q, want full output", got)
	}
	if _, err := os.Stat(quotaLog); !os.IsNotExist(err) {
		t.Fatalf("quota log still present after a passing run: %v", err)
	}
}

func TestOutputCap_SettleQuotaLogFailKeepsLogAsSpill(t *testing.T) {
	c, _ := capFor(t, 4, 0, true)
	quotaLog := filepath.Join(t.TempDir(), "quota.log")
	if err := os.WriteFile(quotaLog, []byte("abcdefgh"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.writer(io.Discard).Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	path, err := c.settle(1, quotaLog)
	if err != nil || path != quotaLog {
		t.Fatalf("settle = %q, %v; want the quota log %q", path, err, quotaLog)
	}
	if _, err := os.Stat(quotaLog); err != nil {
		t.Fatalf("failing run removed the quota log: %v", err)
	}
}

func TestCountLinesAndNotice(t *testing.T) {
	cases := map[string]int{"": 0, "a": 1, "a\n": 1, "a\nb": 2, "a\nb\n": 2}
	for in, want := range cases {
		if got := countLines([]byte(in)); got != want {
			t.Errorf("countLines(%q) = %d, want %d", in, got, want)
		}
	}
}

// truncationNoticeRE matches the closing notice of a truncated stream.
var truncationNoticeRE = regexp.MustCompile(`\[Full output: (.+)\. Truncated: (\d+) lines shown \(([^)]+) limit\)\]\n$`)

// execRunOpts returns testExecOptions with the given env and TTY seam.
func execRunOpts(t *testing.T, env map[string]string) (execOptions, string) {
	t.Helper()
	opts := testExecOptions(t)
	opts.InsertTimeout = 2 * time.Second
	opts.Getenv = envMap(env)
	opts.IsTerminal = noTTY
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	// Keep state out of $HOME so runs stay quiet in read-only sandboxes.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return opts, tmp
}

// spillFiles lists harnez-exec- files in dir.
func spillFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "harnez-exec-") {
			names = append(names, filepath.Join(dir, e.Name()))
		}
	}
	return names
}

// shellRows reads the shell telemetry rows written to opts.DBPath.
func shellRows(t *testing.T, opts execOptions) []telemetry.ToolCall {
	t.Helper()
	db, err := telemetry.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("Open telemetry db: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(telemetry.Filter{CallType: "shell"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return rows
}

func TestRunExecWrapper_BelowLimitIsUnchanged(t *testing.T) {
	opts, tmp := execRunOpts(t, nil)
	var out, errOut bytes.Buffer
	code, err := runExecWrapper([]string{"sh", "-c", "echo small-out; echo small-err >&2"}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil || code != 0 {
		t.Fatalf("runExecWrapper = %d, %v", code, err)
	}
	if out.String() != "small-out\n" || errOut.String() != "small-err\n" {
		t.Fatalf("out=%q err=%q, want unchanged", out.String(), errOut.String())
	}
	if files := spillFiles(t, tmp); len(files) != 0 {
		t.Fatalf("spill files = %v, want none", files)
	}
	rows := shellRows(t, opts)
	if len(rows) != 1 || rows[0].OutputBytes == nil || *rows[0].OutputBytes != rows[0].RawBytes {
		t.Fatalf("rows = %+v, want one row with OutputBytes == RawBytes", rows)
	}
}

// bigMixedScript writes n numbered lines to stdout and to stderr, then exits 3.
func bigMixedScript(n int) string {
	return fmt.Sprintf("i=1; while [ $i -le %d ]; do echo \"out $i\"; echo \"err $i\" >&2; i=$((i+1)); done; exit 3", n)
}

// countTag counts the lines of b that start with tag, such as "out ".
func countTag(b []byte, tag string) int {
	n := bytes.Count(b, []byte("\n"+tag))
	if bytes.HasPrefix(b, []byte(tag)) {
		n++
	}
	return n
}

// checkTruncatedMixed verifies the cap for a mixed stdout+stderr run that
// exited 3 and wrote n lines to each stream. wantLimit is the notice's limit
// text. A line limit must be stopped exactly at its line count. A byte limit
// may leave one partial line at the cut.
func checkTruncatedMixed(t *testing.T, opts execOptions, tmp string, out, errOut *bytes.Buffer, code int, wantLimit string, n int, maxBytes int64) {
	t.Helper()
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 preserved", code)
	}
	m := truncationNoticeRE.FindStringSubmatchIndex(out.String())
	if m == nil {
		t.Fatalf("stdout has no truncation notice:\n%s", out.String())
	}
	notice := out.String()[m[0]:]
	shownOut := out.String()[:m[0]]
	if strings.HasSuffix(shownOut, "\n") && strings.HasPrefix(notice, "\n") {
		t.Fatal("double newline before notice")
	}
	path := out.String()[m[2]:m[3]]
	if filepath.Dir(path) != tmp || !strings.HasPrefix(filepath.Base(path), "harnez-exec-") {
		t.Fatalf("notice path %q is not an harnez-exec- file in %q", path, tmp)
	}

	// Stdout and stderr share one budget, so the notice counts lines of both.
	shownNewlines := bytes.Count([]byte(shownOut), []byte("\n")) + bytes.Count(errOut.Bytes(), []byte("\n"))
	gotLines, err := strconv.Atoi(out.String()[m[4]:m[5]])
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(wantLimit, "2000 lines") {
		if gotLines != 2000 || shownNewlines != 2000 {
			t.Fatalf("notice says %d lines, shown has %d newlines; want exactly 2000", gotLines, shownNewlines)
		}
	} else if gotLines < shownNewlines || gotLines > shownNewlines+1 {
		t.Fatalf("notice says %d lines shown, shown output has %d newlines", gotLines, shownNewlines)
	}
	if limit := out.String()[m[6]:m[7]]; limit != wantLimit {
		t.Fatalf("notice limit = %q, want %q", limit, wantLimit)
	}

	shownBytes := int64(len(shownOut)) + int64(errOut.Len())
	if shownBytes > maxBytes {
		t.Fatalf("shown %d bytes exceeds the shared byte cap %d", shownBytes, maxBytes)
	}

	// The spill holds the complete, untruncated stream verbatim.
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read spill: %v", err)
	}
	if got := countTag(full, "out "); got != n {
		t.Fatalf("spill has %d stdout lines, want %d", got, n)
	}
	if got := countTag(full, "err "); got != n {
		t.Fatalf("spill has %d stderr lines, want %d", got, n)
	}
	if errOut.Len() == 0 {
		t.Fatal("stderr shown is empty, want the shared budget to admit some")
	}

	rows := shellRows(t, opts)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.ExitCode == nil || *row.ExitCode != 3 {
		t.Fatalf("telemetry exit = %v, want 3", row.ExitCode)
	}
	if row.RawBytes < int64(len(full)) {
		t.Fatalf("RawBytes = %d, want >= full output %d", row.RawBytes, len(full))
	}
	if row.OutputBytes == nil {
		t.Fatal("OutputBytes not recorded")
	}
	if *row.OutputBytes != shownBytes {
		t.Fatalf("OutputBytes = %d, want shown bytes %d (notice excluded)", *row.OutputBytes, shownBytes)
	}
}

func TestRunExecWrapper_ByteLimitMixedStreams(t *testing.T) {
	opts, tmp := execRunOpts(t, nil)
	opts.MaxOutputBytes = 4096
	opts.MaxOutputBytesSet = true
	var out, errOut bytes.Buffer
	code, err := runExecWrapper([]string{"sh", "-c", bigMixedScript(3000)}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("runExecWrapper: %v", err)
	}
	checkTruncatedMixed(t, opts, tmp, &out, &errOut, code, "4.0KB", 3000, 4096)
}

func TestRunExecWrapper_LineLimitMixedStreams(t *testing.T) {
	opts, tmp := execRunOpts(t, nil)
	// Bytes stay far below the 1MB cap, so the spec line limit (2000) is what stops output.
	opts.MaxOutputBytes = 1 << 20
	opts.MaxOutputBytesSet = true
	var out, errOut bytes.Buffer
	code, err := runExecWrapper([]string{"sh", "-c", bigMixedScript(3000)}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("runExecWrapper: %v", err)
	}
	checkTruncatedMixed(t, opts, tmp, &out, &errOut, code, "2000 lines", 3000, 1<<20)
}

func TestRunExecWrapper_ZeroByteFlagDisablesCleanly(t *testing.T) {
	opts, tmp := execRunOpts(t, nil)
	opts.MaxOutputBytes = 0
	opts.MaxOutputBytesSet = true
	var out, errOut bytes.Buffer
	code, err := runExecWrapper([]string{"sh", "-c", bigMixedScript(3000)}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil || code != 3 {
		t.Fatalf("runExecWrapper = %d, %v; want exit 3", code, err)
	}
	if strings.Contains(out.String(), "[Full output") || len(spillFiles(t, tmp)) != 0 {
		t.Fatal("--max-output-bytes=0 still truncated or spilled")
	}
	if rows := shellRows(t, opts); len(rows) != 1 || rows[0].OutputBytes == nil || *rows[0].OutputBytes != rows[0].RawBytes {
		t.Fatalf("rows = %+v, want OutputBytes == RawBytes", rows)
	}
}

func TestRunExecWrapper_EnvAndPrefixOverridesReachRun(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		command  string
		tty      bool
		wantNote bool
	}{
		{name: "env truncate=0 disables", env: map[string]string{truncateEnv: "0"}, command: bigMixedScript(3000)},
		{name: "prefix truncate=0 disables", command: "HARNEZ_EXEC_TRUNCATE=0 true; " + bigMixedScript(3000), wantNote: false},
		{name: "prefix bytes applies", command: "HARNEZ_EXEC_MAX_OUTPUT_BYTES=4096 true; " + bigMixedScript(3000), wantNote: true},
		{name: "tty skipped without force", command: bigMixedScript(3000), tty: true},
		{name: "env truncate=1 forces on tty", env: map[string]string{truncateEnv: "1"}, command: bigMixedScript(3000), tty: true, wantNote: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, _ := execRunOpts(t, tc.env)
			opts.IsTerminal = noTTY
			if tc.tty {
				opts.IsTerminal = tty
			}
			var out, errOut bytes.Buffer
			code, err := runExecWrapper([]string{"sh", "-c", tc.command}, opts, strings.NewReader(""), &out, &errOut)
			if err != nil || code != 3 {
				t.Fatalf("runExecWrapper = %d, %v", code, err)
			}
			if got := strings.Contains(out.String(), "[Full output"); got != tc.wantNote {
				t.Fatalf("notice present = %v, want %v", got, tc.wantNote)
			}
		})
	}
}

func TestRunExecWrapper_FlagBeatsEnvAndPrefix(t *testing.T) {
	opts, tmp := execRunOpts(t, map[string]string{maxOutputBytesEnv: "1048576"})
	opts.MaxOutputBytes = 4096
	opts.MaxOutputBytesSet = true
	var out, errOut bytes.Buffer
	cmd := "HARNEZ_EXEC_MAX_OUTPUT_BYTES=2097152 true; " + bigMixedScript(3000)
	code, err := runExecWrapper([]string{"sh", "-c", cmd}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	checkTruncatedMixed(t, opts, tmp, &out, &errOut, code, "4.0KB", 3000, 4096)
}

// bigLinesScript prints 200 distinct lines of about 1KB each (about 200KB),
// which is over the byte limit but under the 2000-line limit. The lines must
// differ: --distill raw collapses runs of identical lines into one "[xN]" line.
const bigLinesScript = `awk 'BEGIN { for (i = 1; i <= 200; i++) printf "%06d %01000d\n", i, i }'`

func TestRunExecWrapper_DistillLargeOutputSpillsDistilled(t *testing.T) {
	opts, tmp := execRunOpts(t, nil)
	opts.Distill = "raw"
	var out, errOut bytes.Buffer
	code, err := runExecWrapper([]string{"sh", "-c", bigLinesScript}, opts, strings.NewReader(""), &out, &errOut)
	if err != nil || code != 0 {
		t.Fatalf("runExecWrapper = %d, %v", code, err)
	}
	m := truncationNoticeRE.FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("distilled output has no notice:\n%.200s", out.String())
	}
	if m[3] != "50.0KB" {
		t.Fatalf("notice limit = %q, want 50.0KB", m[3])
	}
	path := m[1]
	if filepath.Dir(path) != tmp {
		t.Fatalf("spill %q not in %q", path, tmp)
	}
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(full) < 200000 {
		t.Fatalf("distilled spill is %d bytes, want the full distilled output", len(full))
	}
	// Shown output is everything before the banner, less the newline that
	// separates the banner from a shown part that lacks one.
	idx := strings.Index(out.String(), "[Full output")
	if shown := len(strings.TrimSuffix(out.String()[:idx], "\n")); shown > 51200 {
		t.Fatalf("shown %d bytes, want <= 51200", shown)
	}
	// raw mode leaves DistilledBytes NULL (existing behaviour), so the row
	// records the full raw size and the shown size only.
	if rows := shellRows(t, opts); len(rows) != 1 || rows[0].DistilledBytes != nil || rows[0].OutputBytes == nil || rows[0].RawBytes < 200000 {
		t.Fatalf("rows = %+v, want nil DistilledBytes, OutputBytes and full RawBytes", rows)
	}
}

func TestRunExecWrapper_Quota1Truncation(t *testing.T) {
	skipShort(t, "runs real subprocesses under quota-1")
	cases := []struct {
		name    string
		distill string
		script  string
		exit    int
	}{
		{name: "fail reuses quota log", script: bigLinesScript + "; exit 1", exit: 1},
		{name: "fail with distill reuses quota log", distill: "raw", script: bigLinesScript + "; exit 1", exit: 1},
		{name: "pass uses spill file", script: bigLinesScript, exit: 0},
		{name: "pass with distill uses spill file", distill: "raw", script: bigLinesScript, exit: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, tmp := execRunOpts(t, nil)
			opts.Quota1 = true
			opts.Quota1Dir = t.TempDir()
			opts.Distill = tc.distill
			var out, errOut bytes.Buffer
			code, err := runExecWrapper([]string{"sh", "-c", tc.script}, opts, strings.NewReader(""), &out, &errOut)
			if err != nil {
				t.Fatalf("runExecWrapper: %v", err)
			}
			if code != tc.exit {
				t.Fatalf("exit = %d, want %d", code, tc.exit)
			}
			m := truncationNoticeRE.FindStringSubmatch(out.String())
			if m == nil {
				t.Fatalf("no notice:\n%.200s", out.String())
			}
			path := m[1]
			logDir := filepath.Dir(mustQuotaLogPath(t, opts.Quota1Dir))
			if tc.exit != 0 {
				// The failing run keeps its Quota-1 log as the full output and
				// writes no second copy.
				if filepath.Dir(path) != logDir {
					t.Fatalf("failing run cited %q, want the quota log in %q", path, logDir)
				}
				if got := spillFiles(t, tmp); len(got) != 0 {
					t.Fatalf("failing run left spill files %v, want none", got)
				}
				info, err := os.Stat(path)
				if err != nil || info.Size() < 200000 {
					t.Fatalf("quota log %q: %v (size %d), want the full output", path, err, sizeOf(info))
				}
				return
			}
			// The passing run drops its Quota-1 log and cites a spill file.
			if filepath.Dir(path) != tmp || !strings.HasPrefix(filepath.Base(path), "harnez-exec-") {
				t.Fatalf("passing run cited %q, want a spill file in %q", path, tmp)
			}
			full, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(full) < 200000 {
				t.Fatalf("spill is %d bytes, want the full output", len(full))
			}
			if entries, _ := os.ReadDir(logDir); len(entries) != 0 {
				t.Fatalf("passing run left %d quota log(s) in %s", len(entries), logDir)
			}
		})
	}
}

// sizeOf returns info's size, or 0 when info is nil.
func sizeOf(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

// mustQuotaLogPath returns the directory Quota-1 writes its logs to.
func mustQuotaLogPath(t *testing.T, dir string) string {
	t.Helper()
	p, err := quota1LogPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
