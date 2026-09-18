package readcard

import (
	"image/color"
	"path/filepath"
	"strings"
	"unicode"
)

// TokenType represents syntax highlight token classifications.
type TokenType int

const (
	TokenText TokenType = iota
	TokenKeyword
	TokenTypeIdent
	TokenString
	TokenComment
	TokenNumber
	TokenOperator
	TokenPunctuation
	TokenHeader
	TokenList
)

// Token represents a single highlighted chunk of text on a line.
type Token struct {
	Type TokenType
	Text string
	FG   *color.RGBA
	BG   *color.RGBA
}

// HighlightLine tokenizes a line of text for syntax highlighting based on file extension / language.
func HighlightLine(line string, ext string, inMultiComment *bool) []Token {
	if len(line) == 0 {
		return nil
	}

	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	// Markdown special handling
	if ext == "md" || ext == "markdown" {
		return highlightMarkdown(line)
	}

	var tokens []Token
	runes := []rune(line)
	n := len(runes)
	i := 0

	// Check if already in multiline comment
	if inMultiComment != nil && *inMultiComment {
		// Look for end of multiline comment */
		endIdx := strings.Index(line, "*/")
		if endIdx != -1 {
			tokens = append(tokens, Token{Type: TokenComment, Text: line[:endIdx+2]})
			*inMultiComment = false
			restTokens := HighlightLine(line[endIdx+2:], ext, inMultiComment)
			tokens = append(tokens, restTokens...)
			return tokens
		}
		return []Token{{Type: TokenComment, Text: line}}
	}

	for i < n {
		r := runes[i]

		// Whitespace
		if unicode.IsSpace(r) {
			start := i
			for i < n && unicode.IsSpace(runes[i]) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenText, Text: string(runes[start:i])})
			continue
		}

		// Line Comments
		if (r == '/' && i+1 < n && runes[i+1] == '/') ||
			(r == '#' && (ext != "c" && ext != "cpp" && ext != "h" && ext != "hpp")) ||
			(r == '-' && i+1 < n && runes[i+1] == '-' && (ext == "sql" || ext == "lua" || ext == "hs")) ||
			(r == ';' && (ext == "ini" || ext == "clj" || ext == "lisp" || ext == "asm")) {
			tokens = append(tokens, Token{Type: TokenComment, Text: string(runes[i:])})
			break
		}

		// Multiline comment start /*
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			endIdx := strings.Index(string(runes[i:]), "*/")
			if endIdx != -1 {
				cmtText := string(runes[i : i+endIdx+2])
				tokens = append(tokens, Token{Type: TokenComment, Text: cmtText})
				i += endIdx + 2
				continue
			}
			if inMultiComment != nil {
				*inMultiComment = true
			}
			tokens = append(tokens, Token{Type: TokenComment, Text: string(runes[i:])})
			break
		}

		// HTML/XML comment <!-- ... -->
		if r == '<' && i+3 < n && string(runes[i:i+4]) == "<!--" {
			endIdx := strings.Index(string(runes[i:]), "-->")
			if endIdx != -1 {
				cmtText := string(runes[i : i+endIdx+3])
				tokens = append(tokens, Token{Type: TokenComment, Text: cmtText})
				i += endIdx + 3
				continue
			}
			tokens = append(tokens, Token{Type: TokenComment, Text: string(runes[i:])})
			break
		}

		// Strings: "...", '...', `...`
		if r == '"' || r == '\'' || r == '`' {
			quote := r
			start := i
			i++
			escaped := false
			for i < n {
				if escaped {
					escaped = false
					i++
					continue
				}
				if quote != '`' && runes[i] == '\\' {
					escaped = true
					i++
					continue
				}
				if runes[i] == quote {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokenString, Text: string(runes[start:i])})
			continue
		}

		// Numbers: 123, 0x1f, 3.14
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(runes[i+1])) {
			start := i
			if r == '0' && i+1 < n && (runes[i+1] == 'x' || runes[i+1] == 'X' || runes[i+1] == 'b' || runes[i+1] == 'o') {
				i += 2
			}
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '.' || runes[i] == '_') {
				i++
			}
			tokens = append(tokens, Token{Type: TokenNumber, Text: string(runes[start:i])})
			continue
		}

		// Identifiers / Keywords / Types
		if unicode.IsLetter(r) || r == '_' || (r == '$' && (ext == "sh" || ext == "bash" || ext == "php")) {
			start := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[start:i])
			tokType := classifyWord(word, ext)
			tokens = append(tokens, Token{Type: tokType, Text: word})
			continue
		}

		// Operators & Punctuation
		if strings.ContainsRune("=+-*/%&|^!<>:~?@", r) {
			start := i
			for i < n && strings.ContainsRune("=+-*/%&|^!<>:~?@", runes[i]) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenOperator, Text: string(runes[start:i])})
			continue
		}

		if strings.ContainsRune("{}()[].,;", r) {
			tokens = append(tokens, Token{Type: TokenPunctuation, Text: string(r)})
			i++
			continue
		}

		// Fallback single character
		tokens = append(tokens, Token{Type: TokenText, Text: string(r)})
		i++
	}

	return tokens
}

func highlightMarkdown(line string) []Token {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return []Token{{Type: TokenHeader, Text: line}}
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return []Token{{Type: TokenList, Text: line}}
	}
	if strings.HasPrefix(trimmed, "```") {
		return []Token{{Type: TokenKeyword, Text: line}}
	}
	if strings.HasPrefix(trimmed, ">") {
		return []Token{{Type: TokenComment, Text: line}}
	}
	return []Token{{Type: TokenText, Text: line}}
}

func classifyWord(word string, ext string) TokenType {
	if isKeyword(word) {
		return TokenKeyword
	}
	if isType(word) {
		return TokenTypeIdent
	}
	return TokenText
}

var commonKeywords = map[string]bool{
	"func": true, "return": true, "if": true, "else": true, "for": true, "range": true,
	"package": true, "import": true, "type": true, "struct": true, "interface": true,
	"const": true, "var": true, "switch": true, "case": true, "default": true, "break": true,
	"continue": true, "go": true, "select": true, "defer": true, "map": true, "chan": true,
	"def": true, "class": true, "lambda": true, "with": true, "as": true, "try": true,
	"except": true, "finally": true, "raise": true, "from": true, "while": true, "yield": true,
	"async": true, "await": true, "fn": true, "let": true, "mut": true, "impl": true,
	"trait": true, "pub": true, "use": true, "mod": true, "match": true, "enum": true,
	"self": true, "super": true, "function": true, "export": true, "class_name": true,
	"nil": true, "null": true, "true": true, "false": true, "None": true, "True": true, "False": true,
	"SELECT": true, "FROM": true, "WHERE": true, "INSERT": true, "UPDATE": true, "DELETE": true,
	"CREATE": true, "TABLE": true, "INDEX": true, "DROP": true, "ALTER": true, "JOIN": true,
	"ON": true, "GROUP": true, "BY": true, "ORDER": true, "LIMIT": true, "HAVING": true,
}

var commonTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true, "float32": true, "float64": true, "complex64": true, "complex128": true,
	"string": true, "bool": true, "byte": true, "rune": true, "error": true, "any": true,
	"void": true, "char": true, "double": true, "float": true, "long": true, "short": true,
	"i8": true, "i16": true, "i32": true, "i64": true, "i128": true, "isize": true,
	"u8": true, "u16": true, "u32": true, "u64": true, "u128": true, "usize": true,
	"f32": true, "f64": true, "str": true, "Vec": true, "Option": true, "Result": true,
	"String": true, "Number": true, "Boolean": true, "Object": true, "Array": true, "Promise": true,
}

func isKeyword(w string) bool {
	return commonKeywords[w]
}

func isType(w string) bool {
	if commonTypes[w] {
		return true
	}
	// Capitalized Go identifiers (e.g. Command, Filter, Options, Context) often denote exported types
	if len(w) > 1 && unicode.IsUpper(rune(w[0])) && unicode.IsLower(rune(w[1])) {
		return true
	}
	return false
}

// DetectLanguage returns a friendly language name from filename.
func DetectLanguage(path string) string {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case base == "Makefile" || base == "makefile":
		return "Make"
	case base == "Containerfile" || base == "Dockerfile":
		return "Docker"
	case base == "AGENTS.md" || base == "CLAUDE.md":
		return "AgentsDoc"
	case ext == ".go":
		return "Go"
	case ext == ".py":
		return "Python"
	case ext == ".sh" || ext == ".bash":
		return "Shell"
	case ext == ".rs":
		return "Rust"
	case ext == ".c" || ext == ".h":
		return "C"
	case ext == ".cpp" || ext == ".hpp" || ext == ".cc":
		return "C++"
	case ext == ".js" || ext == ".jsx":
		return "JavaScript"
	case ext == ".ts" || ext == ".tsx":
		return "TypeScript"
	case ext == ".json":
		return "JSON"
	case ext == ".yaml" || ext == ".yml":
		return "YAML"
	case ext == ".toml":
		return "TOML"
	case ext == ".md":
		return "Markdown"
	case ext == ".sql":
		return "SQL"
	default:
		if ext != "" {
			return strings.ToUpper(strings.TrimPrefix(ext, "."))
		}
		return "Text"
	}
}
