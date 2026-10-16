package jobs

import (
	"regexp"
	"strings"
)

// Ring keeps the last size bytes written to it; older bytes are dropped.
// Its buffer grows on demand up to size.
type Ring struct {
	buf     []byte
	size    int
	start   int // index of the oldest byte once the buffer is full
	n       int // bytes held
	dropped bool
	lines   []string // Lines cache; nil after a write
}

// NewRing returns a ring that holds at most size bytes (minimum 1).
func NewRing(size int) *Ring {
	return &Ring{size: max(size, 1)}
}

// Write appends p, dropping the oldest bytes beyond the capacity.
func (r *Ring) Write(p []byte) (int, error) {
	written := len(p)
	if written == 0 {
		return 0, nil
	}
	r.lines = nil
	if len(p) >= r.size {
		r.dropped = r.dropped || r.n > 0 || len(p) > r.size
		if cap(r.buf) < r.size {
			r.buf = make([]byte, r.size)
		}
		r.buf = r.buf[:r.size]
		copy(r.buf, p[len(p)-r.size:])
		r.start, r.n = 0, r.size
		return written, nil
	}
	for len(p) > 0 {
		if len(r.buf) < r.size { // still growing: start == 0
			take := min(r.size-len(r.buf), len(p))
			r.buf = append(r.buf, p[:take]...)
			r.n += take
			p = p[take:]
			continue
		}
		end := (r.start + r.n) % r.size
		chunk := min(r.size-end, len(p))
		copy(r.buf[end:end+chunk], p[:chunk])
		p = p[chunk:]
		r.n += chunk
		if r.n > r.size {
			r.start = (r.start + r.n - r.size) % r.size
			r.n = r.size
			r.dropped = true
		}
	}
	return written, nil
}

// Reset empties the ring and keeps its buffer.
func (r *Ring) Reset() {
	r.buf, r.start, r.n, r.dropped, r.lines = r.buf[:0], 0, 0, false, nil
}

// Len returns the number of bytes held.
func (r *Ring) Len() int { return r.n }

// Cap returns the bytes currently allocated.
func (r *Ring) Cap() int { return cap(r.buf) }

// Truncated reports whether older output was dropped.
func (r *Ring) Truncated() bool { return r.dropped }

// Bytes returns a copy of the held bytes, oldest first.
func (r *Ring) Bytes() []byte {
	out := make([]byte, 0, r.n)
	end := r.start + r.n
	if end <= len(r.buf) {
		return append(out, r.buf[r.start:end]...)
	}
	out = append(out, r.buf[r.start:]...)
	return append(out, r.buf[:end-len(r.buf)]...)
}

var (
	escSeq = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|.)`)
	sgrSeq = regexp.MustCompile(`^\x1b\[[0-9;]*m$`)
)

// keepSGR drops every escape sequence except SGR colors and attributes.
func keepSGR(seq string) string {
	if sgrSeq.MatchString(seq) {
		return seq
	}
	return ""
}

// Lines returns the held output as display-safe lines: escape sequences
// other than SGR and control characters are removed, tabs become spaces and a partial first
// line left by truncation is dropped.
// The result is cached until the next write; callers must not modify it.
func (r *Ring) Lines() []string {
	if r.lines == nil {
		r.lines = r.lines0()
	}
	return r.lines
}

func (r *Ring) lines0() []string {
	s := string(r.Bytes())
	if r.dropped {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
	}
	s = escSeq.ReplaceAllStringFunc(s, keepSGR)
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = clean(l)
	}
	return lines
}

func clean(l string) string {
	if i := strings.LastIndexByte(strings.TrimSuffix(l, "\r"), '\r'); i >= 0 {
		l = l[i+1:] // a carriage return redraws the line: keep the last frame
	}
	var b strings.Builder
	for _, c := range l {
		switch {
		case c == '\t':
			b.WriteString("    ")
		case c == 0x1b: // only kept SGR sequences are left
			b.WriteRune(c)
		case c < 0x20 || c == 0x7f:
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
