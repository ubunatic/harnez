// Package modelblock records expiring local "this model is out of quota"
// blocks (issue 1122) and recognises known quota errors that create them.
package modelblock

import (
	"bytes"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const specPath = "spec/model_blocks.yaml"

// Pattern is one known quota error, mirroring spec/schemas/model_blocks.schema.json.
type Pattern struct {
	Provider     string `yaml:"provider"`
	Match        string `yaml:"match"`
	ResetInRegex string `yaml:"reset_in_regex"`
	ResetAtRegex string `yaml:"reset_at_regex"`
	match        *regexp.Regexp
	resetIn      *regexp.Regexp
	resetAt      *regexp.Regexp
}

// Spec is the top-level shape of spec/model_blocks.yaml.
type Spec struct {
	DefaultDuration string    `yaml:"default_duration"`
	Patterns        []Pattern `yaml:"patterns"`
	defaultDuration time.Duration
}

// LoadSpec reads and validates the embedded spec.
func LoadSpec() (*Spec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, specPath)
	if err != nil {
		return nil, err
	}
	return parseSpec(data)
}

func parseSpec(data []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s: %w", specPath, err)
	}
	d, err := time.ParseDuration(s.DefaultDuration)
	if err != nil || d <= 0 {
		return nil, fmt.Errorf("%s: default_duration must be a positive duration", specPath)
	}
	s.defaultDuration = d
	for i := range s.Patterns {
		p := &s.Patterns[i]
		if p.Provider == "" || p.Match == "" {
			return nil, fmt.Errorf("%s: pattern %d needs provider and match", specPath, i)
		}
		if !anchored.MatchString(p.Match) {
			return nil, fmt.Errorf("%s: pattern %d match must be anchored with ^", specPath, i)
		}
		if p.match, err = regexp.Compile(p.Match); err != nil {
			return nil, fmt.Errorf("%s: pattern %d match: %w", specPath, i, err)
		}
		if p.ResetInRegex != "" {
			if p.resetIn, err = regexp.Compile(p.ResetInRegex); err != nil {
				return nil, fmt.Errorf("%s: pattern %d reset_in_regex: %w", specPath, i, err)
			}
		}
		if p.ResetAtRegex != "" {
			if p.resetAt, err = regexp.Compile(p.ResetAtRegex); err != nil {
				return nil, fmt.Errorf("%s: pattern %d reset_at_regex: %w", specPath, i, err)
			}
		}
	}
	return &s, nil
}

// anchored accepts a match regex that starts at the beginning of the text,
// optionally after inline flags such as (?i).
var anchored = regexp.MustCompile(`^(\(\?[a-z]+\))?\^`)

var spanToken = regexp.MustCompile(`(?i)(\d+)\s*(days?|d|hours?|hrs?|h|minutes?|mins?|m|seconds?|secs?|s)\b`)

// parseSpan reads spans like "3 hours 20 minutes" or "1d 2h"; zero if none.
func parseSpan(text string) time.Duration {
	var total time.Duration
	for _, m := range spanToken.FindAllStringSubmatch(text, -1) {
		var n int
		_, _ = fmt.Sscan(m[1], &n) // a parse failure leaves the target at its zero value
		switch strings.ToLower(m[2][:1]) {
		case "d":
			total += time.Duration(n) * 24 * time.Hour
		case "h":
			total += time.Duration(n) * time.Hour
		case "m":
			total += time.Duration(n) * time.Minute
		case "s":
			total += time.Duration(n) * time.Second
		}
	}
	return total
}

// Detect reports whether errText is a known quota error for provider and
// until when to block: the reset time the error names, else now plus the
// spec default.
func (s *Spec) Detect(provider, errText string, now time.Time) (time.Time, bool) {
	errText = strings.TrimSpace(errText)
	for _, p := range s.Patterns {
		if p.Provider != provider || !p.match.MatchString(errText) {
			continue
		}
		if p.resetAt != nil {
			if m := p.resetAt.FindStringSubmatch(errText); m != nil {
				if t, err := time.Parse(time.RFC3339, m[1]); err == nil && t.After(now) {
					return t, true
				}
			}
		}
		if p.resetIn != nil {
			if m := p.resetIn.FindStringSubmatch(errText); m != nil {
				if d := parseSpan(m[1]); d > 0 {
					return now.Add(d), true
				}
			}
		}
		return now.Add(s.defaultDuration), true
	}
	return time.Time{}, false
}
