package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/term"
)

// Environment overrides for the shown-output cap (issue 771). A leading
// VAR=val prefix on the wrapped shell command is honoured the same way
// detectQuota1 reads HARNEZ_QUOTA_1.
const (
	truncateEnv       = "HARNEZ_EXEC_TRUNCATE"
	maxOutputBytesEnv = "HARNEZ_EXEC_MAX_OUTPUT_BYTES"
)

// truncateSettings is the shown-output cap resolved for one run.
type truncateSettings struct {
	Enabled     bool   // the cap applies only when Enabled and MaxBytes > 0
	MaxBytes    int64  // shown-output byte limit; 0 disables truncation
	MaxLines    int    // shown-output line limit; 0 means no line limit
	SpillDir    string // "" means os.TempDir()
	SpillPrefix string
}

func (s truncateSettings) active() bool { return s.Enabled && s.MaxBytes > 0 }

// leadingAssignRE matches one leading NAME=value prefix of a shell command.
var leadingAssignRE = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)=(\S*)\s+`)

// truncateCommandOverrides collects the non-empty HARNEZ_EXEC_* leading
// assignments of the wrapped command. A later assignment wins.
func truncateCommandOverrides(args []string) map[string]string {
	vals := map[string]string{}
	for _, script := range unwrapShellCommand(args) {
		for m := leadingAssignRE.FindStringSubmatch(script); m != nil; m = leadingAssignRE.FindStringSubmatch(script) {
			if strings.HasPrefix(m[1], "HARNEZ_EXEC_") && m[2] != "" {
				vals[m[1]] = m[2]
			}
			script = script[len(m[0]):]
		}
	}
	return vals
}

// parseTruncateToggle reads a HARNEZ_EXEC_TRUNCATE value: 1/true or 0/false.
func parseTruncateToggle(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	}
	return false, fmt.Errorf("%s must be 1, true, 0 or false, got %q", truncateEnv, v)
}

// parseMaxOutputBytes reads a non-negative byte count.
func parseMaxOutputBytes(name, v string) (int64, error) {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer, got %q", name, v)
	}
	return n, nil
}

// resolveTruncateSettings applies the precedence of issue 771:
//
//	--max-output-bytes flag > HARNEZ_EXEC_* command prefix > process env > spec/exec.yaml
//
// The on/off switch follows the first explicit source in the order flag,
// HARNEZ_EXEC_TRUNCATE, HARNEZ_EXEC_MAX_OUTPUT_BYTES. An explicit switch
// overrides the TTY rule. With no explicit switch, the cap is on when the spec
// enables it and out is not an interactive terminal.
func resolveTruncateSettings(opts execOptions, args []string, spec TruncateSpec, out io.Writer) (truncateSettings, error) {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	overrides := truncateCommandOverrides(args)
	env := func(name string) string {
		if v := overrides[name]; v != "" {
			return v
		}
		return getenv(name)
	}

	cfg := truncateSettings{
		MaxBytes:    spec.MaxOutputBytes,
		MaxLines:    spec.MaxOutputLines,
		SpillDir:    spec.SpillDir,
		SpillPrefix: spec.SpillPrefix,
	}
	if opts.MaxOutputBytesSet {
		if opts.MaxOutputBytes < 0 {
			return cfg, fmt.Errorf("--max-output-bytes must be a non-negative integer, got %d", opts.MaxOutputBytes)
		}
		cfg.MaxBytes = opts.MaxOutputBytes
	} else if v := env(maxOutputBytesEnv); v != "" {
		n, err := parseMaxOutputBytes(maxOutputBytesEnv, v)
		if err != nil {
			return cfg, err
		}
		cfg.MaxBytes = n
	}

	toggleSet := true
	switch envToggle := env(truncateEnv); {
	case opts.MaxOutputBytesSet:
		cfg.Enabled = opts.MaxOutputBytes > 0
	case envToggle != "":
		on, err := parseTruncateToggle(envToggle)
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = on
	case env(maxOutputBytesEnv) != "":
		cfg.Enabled = cfg.MaxBytes > 0
	default:
		toggleSet = false
	}
	if !toggleSet {
		isTerminal := opts.IsTerminal
		if isTerminal == nil {
			isTerminal = writerIsTerminal
		}
		cfg.Enabled = spec.Enabled && !isTerminal(out)
	}
	return cfg, nil
}

// writerIsTerminal reports whether w is an interactive terminal.
func writerIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// outputCap is the shown-output budget shared by stdout and stderr. Bytes
// within the budget are forwarded to the caller as they arrive. Once the
// budget is reached, the rest goes only to the spill file, so the child never
// blocks on a full pipe. The spill file is created lazily on the first
// withheld byte, and it starts with the shown head. A mirrored cap never
// writes a spill file: the Quota-1 log already holds the full stream.
type outputCap struct {
	mu        sync.Mutex
	cfg       truncateSettings
	mirrored  bool
	shown     []byte // bytes forwarded so far, at most cfg.MaxBytes
	newlines  int    // newlines in shown
	withheld  bool   // the cap held back at least one byte
	byLines   bool   // the line limit, not the byte limit, held output back first
	spill     *os.File
	spillPath string
	err       error // first spill I/O error; spilling stops after it
}

// newOutputCap returns a cap for cfg. mirrored marks a run whose Quota-1 log
// already holds the full stream.
func newOutputCap(cfg truncateSettings, mirrored bool) *outputCap {
	return &outputCap{cfg: cfg, mirrored: mirrored}
}

// writer returns the writer that forwards to dst under the shared budget. With
// the cap off it returns dst itself.
func (c *outputCap) writer(dst io.Writer) io.Writer {
	if !c.cfg.active() {
		return dst
	}
	return capWriter{c: c, dst: dst}
}

type capWriter struct {
	c   *outputCap
	dst io.Writer
}

func (w capWriter) Write(p []byte) (int, error) {
	return w.c.write(p, w.dst)
}

// allowance returns how many leading bytes of p may be shown, and whether the
// line limit is what stops them. Caller holds c.mu.
func (c *outputCap) allowance(p []byte) (int, bool) {
	if c.withheld {
		// Truncation has begun. Withhold every later write, even one that fits
		// the room left by a rune-boundary back-off, so the shown text stays a
		// prefix of the stream and the spill keeps the rest in order.
		return 0, c.byLines
	}
	keep := len(p)
	if room := c.cfg.MaxBytes - int64(len(c.shown)); int64(keep) > room {
		keep = int(room)
		// Never end the shown text mid-rune when the byte cut lands inside p.
		for keep > 0 && keep < len(p) && !utf8.RuneStart(p[keep]) {
			keep--
		}
	}
	if c.cfg.MaxLines > 0 {
		left := c.cfg.MaxLines - c.newlines
		if left <= 0 {
			return 0, true
		}
		n := 0
		for i, b := range p[:keep] {
			if b == '\n' {
				n++
				if n == left {
					return i + 1, true
				}
			}
		}
	}
	return keep, false
}

func (c *outputCap) write(p []byte, dst io.Writer) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keep, byLines := c.allowance(p)
	if keep > 0 {
		if _, err := dst.Write(p[:keep]); err != nil {
			return 0, err
		}
		c.shown = append(c.shown, p[:keep]...)
		c.newlines += bytes.Count(p[:keep], []byte{'\n'})
	}
	if keep < len(p) {
		if !c.withheld {
			c.withheld, c.byLines = true, byLines
		}
		c.spillLocked(p[keep:])
	}
	return len(p), nil
}

// spillLocked appends rest to the spill file, creating it with the shown head
// on first use. Caller holds c.mu.
func (c *outputCap) spillLocked(rest []byte) {
	if c.mirrored || c.err != nil {
		return
	}
	if c.spill == nil {
		f, err := createSpillFile(c.cfg)
		if err != nil {
			c.err = err
			return
		}
		c.spill, c.spillPath = f, f.Name()
		if _, err := f.Write(c.shown); err != nil {
			c.err = err
			return
		}
	}
	if _, err := c.spill.Write(rest); err != nil {
		c.err = err
	}
}

// createSpillFile opens a new spill file in cfg.SpillDir, or os.TempDir() when
// that is empty.
func createSpillFile(cfg truncateSettings) (*os.File, error) {
	dir := cfg.SpillDir
	if dir == "" {
		dir = os.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return os.CreateTemp(dir, cfg.SpillPrefix+"*.log")
}

// settle finishes the cap after the child exits and returns the path the
// notice should cite ("" when nothing was held back). quotaLog is the Quota-1
// log, or "" when Quota-1 is off. A failed run keeps that log, which holds the
// full stream. A passing run drops it, so its content moves to a spill file
// when output was held back.
func (c *outputCap) settle(exitCode int, quotaLog string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.spill != nil {
		if err := c.spill.Close(); err != nil && c.err == nil {
			c.err = err
		}
	}

	if quotaLog == "" {
		if !c.withheld {
			return "", nil
		}
		return c.spillPath, c.err
	}
	if exitCode != 0 {
		if !c.mirrored && c.spillPath != "" {
			// Distilled overflow: the raw Quota-1 log is the full output.
			_ = os.Remove(c.spillPath)
		}
		if !c.withheld {
			return "", nil
		}
		return quotaLog, nil
	}
	if !c.withheld {
		_ = os.Remove(quotaLog)
		return "", nil
	}
	if !c.mirrored {
		_ = os.Remove(quotaLog)
		return c.spillPath, c.err
	}
	return c.adoptQuotaLog(quotaLog)
}

// adoptQuotaLog turns the passing Quota-1 log into the spill file. It renames
// the log into a fresh spill name, and copies only when the rename crosses
// filesystems.
func (c *outputCap) adoptQuotaLog(quotaLog string) (string, error) {
	f, err := createSpillFile(c.cfg)
	if err != nil {
		return "", err
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := moveFile(quotaLog, path); err != nil {
		// Leave no empty or partial spill behind; the quota log stays put.
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// moveFile renames src to dst, falling back to copy-then-remove across
// filesystems. A failed copy removes the partial dst it created.
func moveFile(src, dst string) (err error) {
	if os.Rename(src, dst) == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

// truncated reports whether the cap held back any output.
func (c *outputCap) truncated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.withheld
}

// shownBytes is the byte count forwarded to the caller, banner excluded.
func (c *outputCap) shownBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return int64(len(c.shown))
}

// notice returns the closing notice for a truncated stream. It cites path and
// ends with a newline. The notice starts on its own line.
func (c *outputCap) notice(path string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	limit := fmt.Sprintf("%.1fKB", float64(c.cfg.MaxBytes)/1024)
	if c.byLines {
		limit = fmt.Sprintf("%d lines", c.cfg.MaxLines)
	}
	banner := fmt.Sprintf("[Full output: %s. Truncated: %d lines shown (%s limit)]\n", path, countLines(c.shown), limit)
	if len(c.shown) > 0 && c.shown[len(c.shown)-1] != '\n' {
		return "\n" + banner
	}
	return banner
}

// countLines counts the lines in b. A final unterminated line also counts.
func countLines(b []byte) int {
	n := bytes.Count(b, []byte{'\n'})
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}
