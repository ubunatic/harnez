package tui

import (
	"strings"

	"ubunatic.com/loom"
)

// AppWidget implements loom.Widget for harnez tui interactive chat interface.
type AppWidget struct {
	Spec          *Spec
	Model         string
	SessionID     string
	Status        string
	Chat          *ChatThread
	Prompt        *PromptBuffer
	OnSubmit      func(prompt string)
	QuitRequested bool
}

// NewAppWidget instantiates a new Loom AppWidget for harnez tui.
func NewAppWidget(spec *Spec, model, sessionID string, onSubmit func(prompt string)) *AppWidget {
	if spec == nil {
		var err error
		spec, err = LoadSpec()
		if err != nil {
			spec = &Spec{
				Header: HeaderSpec{Show: true, Format: "harnez tui — [Model: {{ .Model }}] [Session: {{ .SessionID }}] [Status: {{ .Status }}]"},
				Chat:   ChatSpec{MinHeight: 5, MaxScrollBack: 1000, ShowTimestamps: true, UserPrefix: "User:", AssistantPrefix: "Agent ({{ .Model }}):", ToolPrefix: "[Tool: {{ .Name }}]"},
				Input:  InputSpec{EnterSend: true, CtrlJNewline: true, EditorHotkey: "ctrl+g", HistorySize: 100, MinLines: 3, MaxLines: 8, PromptSymbol: "> "},
				Theme:  ThemeSpec{BorderStyle: "sharp"},
			}
		}
	}
	if model == "" {
		model = "codex:luna"
	}
	if sessionID == "" {
		sessionID = "local-tui-session"
	}

	chat := NewChatThread(spec.Chat)
	prompt := NewPromptBuffer(spec.Input.MinLines, spec.Input.MaxLines, spec.Input.PromptSymbol)

	return &AppWidget{
		Spec:      spec,
		Model:     model,
		SessionID: sessionID,
		Status:    "ready",
		Chat:      chat,
		Prompt:    prompt,
		OnSubmit:  onSubmit,
	}
}

// Draw renders the full Loom canvas layout (Header, Chat Thread, Prompt Box).
func (app *AppWidget) Draw(c *loom.Canvas, r loom.Rect) {
	if r.W <= 0 || r.H <= 0 {
		return
	}

	headerHeight := 0
	if app.Spec.Header.Show {
		headerHeight = 1
		headerText := app.formatHeader()
		c.WriteANSI(r.X, r.Y, headerText)
	}

	promptLines := app.Prompt.Lines(r.W)
	promptHeight := len(promptLines) + 1 // prompt lines + top border

	chatHeight := r.H - headerHeight - promptHeight
	if chatHeight < 1 {
		chatHeight = 1
	}

	// Render Chat Thread View
	chatY := r.Y + headerHeight
	visibleChatLines := app.Chat.VisibleLines(r.W, chatHeight)
	for i, line := range visibleChatLines {
		if i >= chatHeight {
			break
		}
		c.WriteANSI(r.X, chatY+i, line)
	}

	// Render Prompt Divider Line
	dividerY := chatY + chatHeight
	if dividerY < r.Y+r.H {
		divider := strings.Repeat("─", r.W)
		c.WriteANSI(r.X, dividerY, divider)
	}

	// Render Prompt Box
	promptY := dividerY + 1
	for i, line := range promptLines {
		if promptY+i >= r.Y+r.H {
			break
		}
		c.WriteANSI(r.X, promptY+i, line)
	}
}

// ConsumeKey handles key events according to keybindings in spec.
func (app *AppWidget) ConsumeKey(e loom.KeyEvent) loom.EventResult {
	key := strings.ToLower(e.Key)

	// Ctrl+C or Esc -> Quit
	if e.Is("ctrl-c", "esc") || key == "ctrl-c" || key == "esc" {
		app.QuitRequested = true
		return loom.Quit()
	}

	// Ctrl+J -> Insert Newline in prompt
	if e.Is("ctrl-j") || key == "ctrl-j" || e.Text == "\n" {
		if app.Spec.Input.CtrlJNewline {
			app.Prompt.InsertNewline()
			return loom.Handled()
		}
	}

	// Enter -> Send prompt
	if e.Is("enter") || key == "enter" || key == "return" {
		if app.Spec.Input.EnterSend {
			text := strings.TrimSpace(app.Prompt.Text())
			if text != "" {
				app.Prompt.AddHistory(text)
				AddTurn(app.Chat, RoleUser, "", text)
				if app.OnSubmit != nil {
					app.OnSubmit(text)
				}
				app.Prompt.Clear()
			}
			return loom.Handled()
		}
	}

	// Backspace -> Delete char or join lines
	if e.Is("backspace") || key == "backspace" || key == "backspace2" {
		app.Prompt.Backspace()
		return loom.Handled()
	}

	// Left / Right / Up / Down navigation
	if e.Is("left") || key == "left" {
		app.Prompt.MoveLeft()
		return loom.Handled()
	}
	if e.Is("right") || key == "right" {
		app.Prompt.MoveRight()
		return loom.Handled()
	}
	if e.Is("up") || key == "up" {
		app.Prompt.MoveUp()
		return loom.Handled()
	}
	if e.Is("down") || key == "down" {
		app.Prompt.MoveDown()
		return loom.Handled()
	}

	// Scroll Up / PageUp -> Scroll chat thread
	if e.Is("pgup") || key == "pgup" {
		ScrollUp(app.Chat, 5)
		return loom.Handled()
	}
	// Scroll Down / PageDown -> Scroll chat thread
	if e.Is("pgdn") || key == "pgdn" {
		ScrollDown(app.Chat, 5)
		return loom.Handled()
	}

	// Text insertion
	if e.Text != "" && len(e.Text) == 1 && e.Text[0] >= 32 {
		for _, r := range e.Text {
			app.Prompt.InsertChar(r)
		}
		return loom.Handled()
	}

	return loom.Ignored()
}

// ConsumeMouse handles mouse events.
func (app *AppWidget) ConsumeMouse(_ loom.MouseEvent) loom.EventResult {
	return loom.Ignored()
}

func (app *AppWidget) formatHeader() string {
	fmtStr := app.Spec.Header.Format
	statusStr := app.Status
	if ind, ok := app.Spec.Header.StatusIndicators[app.Status]; ok {
		statusStr = ind
	}

	res := fmtStr
	res = strings.ReplaceAll(res, "{{ .Model }}", app.Model)
	res = strings.ReplaceAll(res, "{{ .SessionID }}", app.SessionID)
	res = strings.ReplaceAll(res, "{{ .Status }}", statusStr)
	return res
}

// RenderCanvas renders a single frame of AppWidget into ANSI line strings for testing.
func RenderCanvas(app *AppWidget, cols, rows int) []string {
	return loom.Render(app, cols, rows)
}
