package jobs

import (
	"regexp"
	"strings"
)

// Ring keeps the last Cap bytes written to it; older bytes are dropped.
type Ring struct {
	buf     []byte
	start   int // index of the oldest byte
	n       int // bytes held
	dropped bool
}

// NewRing returns a ring that holds at most size bytes (minimum 1).
func NewRing(size int) *Ring {
	if size < 1 {
		size = 1
	}
	return &Ring{buf: make([]byte, size)}
}

// Write appends p, dropping the oldest bytes beyond the capacity.
func (r *Ring) Write(p []byte) (int, error) {
	written := len(p)
	size := len(r.buf)
	if len(p) >= size {
		r.dropped = r.dropped || r.n > 0 || len(p) > size
		copy(r.buf, p[len(p)-size:])
		r.start, r.n = 0, size
		return written, nil
	}
	for len(p) > 0 {
		end := (r.start + r.n) % size
		chunk := size - end
		if chunk > len(p) {
			chunk = len(p)
		}
		copy(r.buf[end:end+chunk], p[:chunk])
		p = p[chunk:]
		r.n += chunk
		if r.n > size {
			over := r.n - size
			r.start = (r.start + over) % size
			r.n = size
			r.dropped = true
		}
	}
	return written, nil
}

// Len returns the number of bytes held.
func (r *Ring) Len() int { return r.n }

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

var escSeq = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|.)`)

// Lines returns the held output as display-safe lines: escape sequences and
// control characters are removed, tabs become spaces and a partial first
// line left by truncation is dropped.
func (r *Ring) Lines() []string {
	s := string(r.Bytes())
	if r.dropped {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
	}
	s = escSeq.ReplaceAllString(s, "")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
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
		case c < 0x20 || c == 0x7f:
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
