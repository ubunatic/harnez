package readcard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseLineNumbers returns a cadence (0 disables the gutter, 1 labels every line).
func ParseLineNumbers(mode string) (int, error) {
	switch mode {
	case "", "all":
		return 1, nil
	case "none", "off":
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimPrefix(mode, "every:"))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid line-numbers %q: use all, off, or a positive cadence", mode)
	}
	return n, nil
}

// CompressionResult includes original source anchors for each displayed line.
type CompressionResult struct {
	Lines       []string
	SourceLines []int
}

// Compress safely compacts supported source formats without evaluating source code.
// Go is parsed and its token stream checked after compaction. JSON uses json.Compact.
// Shell uses a conservative lexical subset; complex expansion/heredoc syntax is
// preserved verbatim. Both shell modes preserve newlines and all quoted bytes.
func Compress(lines []string, filename, mode string, start int) (CompressionResult, error) {
	result := CompressionResult{Lines: append([]string(nil), lines...)}
	for i := range lines {
		result.SourceLines = append(result.SourceLines, start+i)
	}
	if mode == "" || mode == "off" {
		return result, nil
	}
	if mode != "ws" && mode != "ast" {
		return result, fmt.Errorf("invalid compression %q: use ws, ast, or off", mode)
	}
	source := strings.Join(lines, "\n")
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".json":
		var dst bytes.Buffer
		if err := json.Compact(&dst, []byte(source)); err != nil {
			return result, fmt.Errorf("compress JSON: %w", err)
		}
		return CompressionResult{Lines: []string{dst.String()}, SourceLines: []int{start}}, nil
	case ".go":
		if _, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.ParseComments); err != nil {
			return result, fmt.Errorf("compress Go requires a complete valid file: %w", err)
		}
		protected, signature := goTokens(source)
		var out strings.Builder
		for i := 0; i < len(source); {
			if protected[i] || (source[i] != ' ' && source[i] != '\t') {
				out.WriteByte(source[i])
				i++
				continue
			}
			j := i
			for j < len(source) && !protected[j] && (source[j] == ' ' || source[j] == '\t') {
				j++
			}
			if i > 0 && source[i-1] != '\n' && j < len(source) && source[j] != '\n' {
				out.WriteByte(' ')
			}
			i = j
		}
		compacted := strings.Split(out.String(), "\n")
		result = CompressionResult{}
		offset := 0
		for i, line := range compacted {
			// Blank lines within raw strings and block comments are significant.
			if line != "" || (offset < len(protected) && protected[offset]) {
				result.Lines = append(result.Lines, line)
				result.SourceLines = append(result.SourceLines, start+i)
			}
			offset += len(lines[i]) + 1
		}
		_, after := goTokens(strings.Join(result.Lines, "\n"))
		if signature != after {
			return CompressionResult{}, fmt.Errorf("compression changed Go tokens")
		}
		return result, nil
	case ".sh", ".bash", ".zsh":
		result.Lines = strings.Split(compactShell(source), "\n")
		return result, nil
	default:
		return result, fmt.Errorf("compression is supported for Go, JSON, and Shell files, not %q", filename)
	}
}

func goTokens(source string) ([]bool, string) {
	protected := make([]bool, len(source))
	fset := token.NewFileSet()
	f := fset.AddFile("source.go", -1, len(source))
	var s scanner.Scanner
	s.Init(f, []byte(source), nil, scanner.ScanComments)
	var signature strings.Builder
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		fmt.Fprintf(&signature, "%d:%q;", tok, lit)
		if tok == token.STRING || tok == token.CHAR || tok == token.COMMENT {
			offset := f.Offset(pos)
			for i := offset; i < offset+len(lit) && i < len(protected); i++ {
				protected[i] = true
			}
		}
	}
	return protected, signature.String()
}

func compactShell(source string) string {
	// Expansion grammars, heredoc bodies and continuations need a full shell
	// parser. Keeping them untouched is safer than guessing token boundaries.
	for _, syntax := range []string{"<<", "`", "$(", "${", "\\", "((", "[["} {
		if strings.Contains(source, syntax) {
			return source
		}
	}
	var out strings.Builder
	var quote byte
	comment, wordStart := false, true
	for i := 0; i < len(source); i++ {
		c := source[i]
		if comment {
			out.WriteByte(c)
			if c == '\n' {
				comment = false
				wordStart = true
			}
			continue
		}
		if quote != 0 {
			out.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			out.WriteByte(c)
			wordStart = false
			continue
		}
		if c == '#' && wordStart {
			comment = true
			out.WriteByte(c)
			continue
		}
		if c == ' ' || c == '\t' {
			for i+1 < len(source) && (source[i+1] == ' ' || source[i+1] == '\t') {
				i++
			}
			// Preserve a separator even at line start: quoting boundaries and
			// shell assignment syntax must never become adjacent accidentally.
			out.WriteByte(' ')
			wordStart = true
			continue
		}
		out.WriteByte(c)
		wordStart = strings.ContainsRune("\n;|&()", rune(c))
	}
	if quote != 0 {
		return source
	}
	return out.String()
}
