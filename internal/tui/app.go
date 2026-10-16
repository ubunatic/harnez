package tui

import (
	"strings"
	"unicode"

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

	run     TurnFunc
	post    func(func())
	resolve func(string) (string, error)
	turn    *turnState
}

// NewAppWidget instantiates a new Loom AppWidget for harnez tui.
// spec must come from LoadSpec or ParseSpec; it is the only source of defaults.
func NewAppWidget(spec *Spec, model, sessionID string, onSubmit func(prompt string)) *AppWidget {
	return &AppWidget{
		Spec:      spec,
		Model:     model,
		SessionID: sessionID,
		Status:    "idle",
		Chat:      NewChatThread(spec.Chat),
		Prompt:    NewPromptBuffer(spec.Input.MinLines, spec.Input.MaxLines, spec.Input.HistorySize, spec.Input.PromptSymbol),
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
	kb := app.Spec.Keybindings
	name := keyName(e)
	switch {
	case app.Busy() && bound(name, kb.CancelTurn):
		app.CancelTurn()
	case bound(name, kb.Quit):
		app.QuitRequested = true
		return loom.Quit()
	case bound(name, kb.Newline):
		app.Prompt.InsertNewline()
	case bound(name, kb.SendPrompt):
		// Terminals send the same byte for Enter and Ctrl+J, so a trailing
		// backslash is the portable way to continue a prompt on a new line.
		if app.Prompt.TrimContinuation() {
			app.Prompt.InsertNewline()
			return loom.Handled()
		}
		app.submit()
	case bound(name, kb.ScrollUp):
		ScrollUp(app.Chat, 5)
	case bound(name, kb.ScrollDown):
		ScrollDown(app.Chat, 5)
	case name == "backspace":
		app.Prompt.Backspace()
	case name == "left":
		app.Prompt.MoveLeft()
	case name == "right":
		app.Prompt.MoveRight()
	case name == "up":
		app.Prompt.MoveUp()
	case name == "down":
		app.Prompt.MoveDown()
	case e.Key == "" && e.Text != "":
		app.insertText(e.Text)
	default:
		return loom.Ignored()
	}
	return loom.Handled()
}

// ConsumePaste inserts bracketed-paste text, keeping its newlines.
func (app *AppWidget) ConsumePaste(p loom.PasteEvent) loom.EventResult {
	app.insertText(strings.ReplaceAll(p.Text, "\r\n", "\n"))
	return loom.Handled()
}

func (app *AppWidget) submit() {
	text := strings.TrimSpace(app.Prompt.Text())
	if text == "" {
		return
	}
	app.Prompt.AddHistory(text)
	app.Prompt.Clear()
	if app.handleCommand(text) {
		return
	}
	AddTurn(app.Chat, RoleUser, "", text)
	if app.OnSubmit != nil {
		app.OnSubmit(text)
	}
	if app.run != nil {
		app.startTurn(text)
	}
}

func (app *AppWidget) insertText(s string) {
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			app.Prompt.InsertNewline()
		case unicode.IsPrint(r):
			app.Prompt.InsertChar(r)
		}
	}
}

// keyName maps a loom key event to the spec's key vocabulary.
func keyName(e loom.KeyEvent) string {
	switch e.Key {
	case "alt-\r", "alt-\n":
		return "alt-enter"
	}
	return e.Key
}

// bound reports whether name matches a spec binding; "ctrl+j" and "ctrl-j" are equal.
func bound(name string, keys []string) bool {
	if name == "" {
		return false
	}
	for _, k := range keys {
		if strings.ReplaceAll(strings.ToLower(k), "+", "-") == name {
			return true
		}
	}
	return false
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
