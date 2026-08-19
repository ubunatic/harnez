package jsonc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// ToStrings converts a []any (from JSON unmarshal) or []string to []string.
func ToStrings(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		ss := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				ss = append(ss, s)
			}
		}
		return ss
	}
	return nil
}

// UnionStrings returns a∪b, preserving order (a first, then new items from b).
func UnionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// StripComments removes // line comments from JSONC for parsing.
func StripComments(data []byte) []byte {
	var result []byte
	inString := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			result = append(result, b)
			if b == '\\' && i+1 < len(data) {
				i++
				result = append(result, data[i])
			} else if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			result = append(result, b)
		} else if b == '/' && i+1 < len(data) && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				result = append(result, '\n')
			}
		} else {
			result = append(result, b)
		}
	}
	return result
}

// Read parses a JSONC file into a map.
func Read(path string) map[string]any {
	m := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	// Use Decoder so trailing content after the first JSON value is ignored.
	dec := json.NewDecoder(bytes.NewReader(StripComments(data)))
	dec.Decode(&m) //nolint:errcheck
	return m
}

// MarshalPretty formats a value to indented JSON without trailing newline.
func MarshalPretty(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(v)
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// Validate checks that the data is valid JSON.
func Validate(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("generated invalid JSON")
	}
	return nil
}
