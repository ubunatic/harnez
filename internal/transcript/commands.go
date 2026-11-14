package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"ubunatic.com/harnez/internal/usage"
)

// ResolveCommands finds a unique Claude or Codex transcript, or accepts a file path.
func ResolveCommands(id, provider string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("session ID or path is required")
	}
	if info, err := os.Stat(id); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("session path is not a regular file: %s", id)
		}
		return id, nil
	}
	if strings.ContainsAny(id, `/\`) || strings.HasSuffix(id, ".jsonl") {
		return "", fmt.Errorf("session file not found: %s", id)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var matches []string
	if provider == "" || provider == "claude" {
		claude := os.Getenv("CLAUDE_CONFIG_DIR")
		if claude == "" {
			claude = filepath.Join(home, ".claude")
		}
		files, err := usage.ClaudeTranscriptFiles(claude)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		for _, path := range files {
			if strings.HasPrefix(strings.TrimSuffix(filepath.Base(path), ".jsonl"), id) {
				matches = append(matches, path)
			}
		}
	}
	if provider == "" || provider == "codex" {
		codex := os.Getenv("CODEX_HOME")
		if codex == "" {
			codex = filepath.Join(home, ".codex")
		}
		files, err := filepath.Glob(filepath.Join(codex, "sessions", "*", "*", "*", "rollout-*.jsonl"))
		if err != nil {
			return "", err
		}
		for _, path := range files {
			base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
			// Rollout IDs follow the timestamp and are normally UUIDs.
			if len(base) >= 36 && strings.HasPrefix(base[len(base)-36:], id) {
				matches = append(matches, path)
			}
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("transcript for session %q not found", id)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("ambiguous session %q: %d transcripts; use a full ID or path", id, len(matches))
	}
	return matches[0], nil
}

// Commands reads provider JSONL and emits shell tool inputs in transcript order.
func Commands(r io.Reader, emit func(string) error) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for line := 1; s.Scan(); line++ {
		var row struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			Payload struct {
				Type, Name               string
				Arguments, Input, Action json.RawMessage
			} `json:"payload"`
		}
		if err := json.Unmarshal(s.Bytes(), &row); err != nil {
			return fmt.Errorf("decode line %d: %w", line, err)
		}
		var commands []string
		if row.Type == "assistant" {
			var blocks []struct {
				Type, Name string
				Input      struct {
					Command string `json:"command"`
				} `json:"input"`
			}
			if len(row.Message.Content) > 0 && row.Message.Content[0] == '[' {
				if err := json.Unmarshal(row.Message.Content, &blocks); err != nil {
					return fmt.Errorf("decode line %d: %w", line, err)
				}
			}
			for _, b := range blocks {
				if b.Type == "tool_use" && b.Name == "Bash" {
					commands = append(commands, b.Input.Command)
				}
			}
		}
		if row.Type == "response_item" {
			p := row.Payload
			if (p.Type == "function_call" || p.Type == "custom_tool_call") && (p.Name == "exec_command" || p.Name == "shell_command" || p.Name == "shell") {
				raw := p.Arguments
				if len(raw) == 0 {
					raw = p.Input
				}
				commands = append(commands, commandInput(raw))
			} else if p.Type == "custom_tool_call" && p.Name == "exec" {
				var source string
				if json.Unmarshal(p.Input, &source) == nil {
					commands = append(commands, jsCommands(source)...)
				}
			} else if p.Type == "local_shell_call" {
				commands = append(commands, commandInput(p.Action))
			}
		}
		for _, command := range commands {
			if command != "" {
				if err := emit(command); err != nil {
					return err
				}
			}
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("read transcript: %w", err)
	}
	return nil
}

func commandInput(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return ""
		}
		raw = []byte(text)
		if !json.Valid(raw) {
			return text
		}
	}
	var input struct {
		Cmd     string          `json:"cmd"`
		Command json.RawMessage `json:"command"`
	}
	if json.Unmarshal(raw, &input) != nil {
		return ""
	}
	if input.Cmd != "" {
		return input.Cmd
	}
	var text string
	if json.Unmarshal(input.Command, &text) == nil {
		return text
	}
	var args []string
	if json.Unmarshal(input.Command, &args) == nil {
		for i, arg := range args {
			args[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		}
		return strings.Join(args, " ")
	}
	return ""
}

// jsCommands extracts literal command fields from calls in Codex exec blocks.
// Dynamic expressions cannot be reconstructed from a transcript and are skipped.
func jsCommands(source string) []string {
	type token struct {
		text    string
		literal bool
	}
	var tokens []token
	for i := 0; i < len(source); {
		c := source[i]
		if unicode.IsSpace(rune(c)) {
			i++
			continue
		}
		if c == '/' && i+1 < len(source) && source[i+1] == '/' {
			i += 2
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(source) && source[i+1] == '*' {
			i += 2
			for i+1 < len(source) && source[i:i+2] != "*/" {
				i++
			}
			i = min(i+2, len(source))
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote := c
			i++
			var b strings.Builder
			valid := true
			closed := false
			for i < len(source) {
				if source[i] == quote {
					i++
					closed = true
					break
				}
				if quote == '`' && source[i] == '$' && i+1 < len(source) && source[i+1] == '{' {
					valid = false
				}
				if source[i] == '\\' {
					if i+1 >= len(source) {
						valid = false
						i++
						break
					}
					if source[i+1] == '\n' {
						i += 2
						continue
					}
					r, _, tail, err := strconv.UnquoteChar(source[i:], quote)
					if err != nil {
						valid = false
						i += 2
						continue
					}
					b.WriteRune(r)
					i = len(source) - len(tail)
					continue
				}
				b.WriteByte(source[i])
				i++
			}
			tokens = append(tokens, token{b.String(), valid && closed})
			continue
		}
		if c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			start := i
			i++
			for i < len(source) && (source[i] == '_' || source[i] == '$' || source[i] >= 'a' && source[i] <= 'z' || source[i] >= 'A' && source[i] <= 'Z' || source[i] >= '0' && source[i] <= '9') {
				i++
			}
			tokens = append(tokens, token{text: source[start:i]})
			continue
		}
		tokens = append(tokens, token{text: string(c)})
		i++
	}
	var commands []string
	for i := 0; i+5 < len(tokens); i++ {
		if tokens[i].text != "tools" || tokens[i+1].text != "." || (tokens[i+2].text != "exec_command" && tokens[i+2].text != "shell_command" && tokens[i+2].text != "shell") || tokens[i+3].text != "(" || tokens[i+4].text != "{" {
			continue
		}
		depth := 1
		for j := i + 5; j < len(tokens) && depth > 0; j++ {
			if !tokens[j].literal && tokens[j].text == "{" {
				depth++
			}
			if !tokens[j].literal && tokens[j].text == "}" {
				depth--
				continue
			}
			if depth == 1 && (tokens[j].text == "cmd" || tokens[j].text == "command") && j+2 < len(tokens) && tokens[j+1].text == ":" && tokens[j+2].literal && (j+3 < len(tokens) && (tokens[j+3].text == "," || tokens[j+3].text == "}")) {
				commands = append(commands, tokens[j+2].text)
				break
			}
		}
	}
	return commands
}
