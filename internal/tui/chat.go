package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// TurnRole identifies turn message origin.
type TurnRole string

const (
	RoleUser      TurnRole = "user"
	RoleAssistant TurnRole = "assistant"
	RoleTool      TurnRole = "tool"
	RoleError     TurnRole = "error"
)

// Turn represents a single chat turn in the conversation history.
type Turn struct {
	Role      TurnRole
	Name      string
	Content   string
	Timestamp time.Time
}

// ChatThread maintains conversation turns and scroll viewport.
type ChatThread struct {
	turns         []Turn
	maxScrollBack int
	scrollOffset  int
	spec          ChatSpec
}

// NewChatThread constructs a ChatThread according to spec.
func NewChatThread(spec ChatSpec) *ChatThread {
	maxScroll := spec.MaxScrollBack
	if maxScroll < 10 {
		maxScroll = 1000
	}
	return &ChatThread{
		maxScrollBack: maxScroll,
		spec:          spec,
	}
}

// AddTurn appends a turn to the thread and resets scroll offset to bottom.
func AddTurn(ct *ChatThread, role TurnRole, name, content string) {
	t := Turn{
		Role:      role,
		Name:      name,
		Content:   content,
		Timestamp: time.Now(),
	}
	ct.turns = append(ct.turns, t)
	if len(ct.turns) > ct.maxScrollBack {
		ct.turns = ct.turns[len(ct.turns)-ct.maxScrollBack:]
	}
	ct.scrollOffset = 0 // jump to bottom
}

// AppendLast appends text to the last turn when it has role and name, else
// adds a new turn. Consecutive messages of one reply join with a blank line.
func AppendLast(ct *ChatThread, role TurnRole, name, text string) {
	if n := len(ct.turns); n > 0 && ct.turns[n-1].Role == role && ct.turns[n-1].Name == name {
		t := &ct.turns[n-1]
		if t.Content != "" {
			t.Content += "\n\n"
		}
		t.Content += text
		ct.scrollOffset = 0
		return
	}
	AddTurn(ct, role, name, text)
}

// ScrollUp decreases scroll offset (moves view upwards into history).
func ScrollUp(ct *ChatThread, delta int) {
	ct.scrollOffset += delta
}

// ScrollDown increases scroll offset (moves view downwards toward latest).
func ScrollDown(ct *ChatThread, delta int) {
	ct.scrollOffset -= delta
	if ct.scrollOffset < 0 {
		ct.scrollOffset = 0
	}
}

// Turns returns all turns in the thread.
func (ct *ChatThread) Turns() []Turn {
	return ct.turns
}

// FormattedLines renders formatted and wrapped lines of all turns.
func (ct *ChatThread) FormattedLines(width int) []string {
	if width < 10 {
		width = 80
	}
	var out []string
	for _, t := range ct.turns {
		header := ct.formatHeader(t)
		if header != "" {
			out = append(out, header)
		}
		lines := strings.Split(t.Content, "\n")
		for _, line := range lines {
			wrapped := wrapLine(line, width)
			out = append(out, wrapped...)
		}
		out = append(out, "") // blank separator
	}
	return out
}

// VisibleLines returns the lines visible in viewport of specified height.
func (ct *ChatThread) VisibleLines(width, height int) []string {
	all := ct.FormattedLines(width)
	if height <= 0 {
		return all
	}
	total := len(all)
	if total <= height {
		return all
	}

	maxOffset := total - height
	offset := ct.scrollOffset
	if offset > maxOffset {
		offset = maxOffset
		ct.scrollOffset = offset
	}

	end := total - offset
	start := end - height
	if start < 0 {
		start = 0
	}
	if end > total {
		end = total
	}
	return all[start:end]
}

func (ct *ChatThread) formatHeader(t Turn) string {
	var prefix string
	switch t.Role {
	case RoleUser:
		prefix = ct.spec.UserPrefix
		if prefix == "" {
			prefix = "User:"
		}
	case RoleAssistant:
		prefix = ct.spec.AssistantPrefix
		if prefix == "" {
			prefix = "Agent:"
		}
		model := t.Name
		if model == "" {
			model = "model"
		}
		prefix = strings.ReplaceAll(prefix, "{{ .Model }}", model)
	case RoleTool:
		prefix = ct.spec.ToolPrefix
		if prefix == "" {
			prefix = "[Tool]"
		}
		toolName := t.Name
		if toolName == "" {
			toolName = "exec"
		}
		prefix = strings.ReplaceAll(prefix, "{{ .Name }}", toolName)
	case RoleError:
		prefix = ct.spec.ErrorPrefix
		if prefix == "" {
			prefix = "Error:"
		}
	default:
		prefix = string(t.Role) + ":"
	}

	if ct.spec.ShowTimestamps && !t.Timestamp.IsZero() {
		return fmt.Sprintf("%s (%s)", prefix, t.Timestamp.Format("15:04:05"))
	}
	return prefix
}

func wrapLine(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	if runewidth.StringWidth(s) <= width {
		return []string{s}
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0

	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if currentWidth+rw > width {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
