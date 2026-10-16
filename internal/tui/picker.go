package tui

import (
	"sort"
	"strings"
	"unicode"

	"ubunatic.com/harnez/internal/find"
	"ubunatic.com/loom"
)

type pickKind int

const (
	pickNone pickKind = iota
	pickFile
	pickCommand
)

// picker is the inline completion popover. It edits the prompt directly:
// the query is the text between the trigger rune and the cursor.
type picker struct {
	kind     pickKind
	row, col int // buffer position of the trigger rune
	query    string
	matches  []pickItem
	sel      int
}

func (p *picker) isOpen() bool { return p.kind != pickNone }

func (app *AppWidget) closePicker() { app.picker = picker{} }

func (app *AppWidget) trigger(k pickKind) string {
	if k == pickCommand {
		return app.Spec.Picker.CommandTrigger
	}
	return app.Spec.Picker.FileTrigger
}

// maybeOpenPicker opens a picker when typed is a trigger that was just
// inserted at a valid spot: the file trigger at the start of a word, the
// command trigger as the first character of the prompt.
func (app *AppWidget) maybeOpenPicker(typed string) {
	if app.picker.isOpen() {
		return
	}
	row, col := app.Prompt.Cursor()
	at := col - 1
	line := []rune(app.Prompt.Line(row))
	if at < 0 || at >= len(line) || string(line[at]) != typed {
		return
	}
	var kind pickKind
	switch {
	case typed == app.Spec.Picker.CommandTrigger && row == 0 && at == 0:
		kind = pickCommand
	case typed == app.Spec.Picker.FileTrigger && app.sources != nil && (at == 0 || unicode.IsSpace(line[at-1])):
		kind = pickFile
	default:
		return
	}
	app.picker = picker{kind: kind, row: row, col: at}
	if s := app.sources; s != nil {
		if kind == pickFile {
			s.ensure(&s.files, s.loadFiles, app.syncPicker)
		} else {
			s.ensure(&s.skills, s.loadSkills, app.syncPicker)
		}
	}
	app.refilter()
}

// syncPicker re-reads the query after an edit and closes the picker when the
// cursor left the trigger's word (Backspace past the trigger, a space, a
// cursor move to another line).
func (app *AppWidget) syncPicker() {
	p := &app.picker
	if !p.isOpen() {
		return
	}
	row, col := app.Prompt.Cursor()
	line := []rune(app.Prompt.Line(row))
	if row != p.row || col <= p.col || col > len(line) || string(line[p.col]) != app.trigger(p.kind) {
		app.closePicker()
		return
	}
	q := string(line[p.col+1 : col])
	if strings.IndexFunc(q, unicode.IsSpace) >= 0 {
		app.closePicker()
		return
	}
	if q != p.query {
		p.query, p.sel = q, 0
	}
	app.refilter()
}

// pickerLoading reports whether the open picker still waits for its source.
func (app *AppWidget) pickerLoading() bool {
	s := app.sources
	if s == nil {
		return false
	}
	if app.picker.kind == pickFile {
		return s.files.state != lazyReady
	}
	return s.skills.state != lazyReady
}

// pickerNote is the read error of the open picker's source, if any.
func (app *AppWidget) pickerNote() string {
	if s := app.sources; s != nil && app.picker.kind == pickFile {
		return s.files.note
	}
	return ""
}

func (app *AppWidget) refilter() {
	p := &app.picker
	var items []pickItem
	if p.kind == pickFile {
		if app.sources != nil {
			items = app.sources.files.items
		}
	} else {
		items = app.commandItems()
	}
	p.matches = rankItems(items, p.query, app.Spec.Picker.MaxItems)
	p.sel = min(max(p.sel, 0), max(len(p.matches)-1, 0))
}

// commandItems lists the TUI built-ins, then installed skills not shadowed by one.
func (app *AppWidget) commandItems() []pickItem {
	var items []pickItem
	seen := map[string]bool{}
	for _, b := range app.Spec.Picker.Builtins {
		seen[b.Name] = true
		items = append(items, newPickItem(b.Name, b.Description, b.Description))
	}
	if app.sources != nil {
		for _, it := range app.sources.skills.items {
			if !seen[it.Name] {
				items = append(items, it)
			}
		}
	}
	return items
}

// rankItems filters items by query with find's match classes and returns
// at most limit, best first: name class, then a hit at a word start of the
// name above a mid-word hit, then a hit in the basename (files) or
// description (commands, as a weaker class), then shorter, then A-Z.
// An empty query keeps the source order.
func rankItems(items []pickItem, query string, limit int) []pickItem {
	q := find.Normalize(query)
	if q == "" {
		return items[:min(len(items), limit)]
	}
	type ranked struct {
		it          pickItem
		class, base int
		mid         bool // query hits only mid-word in the name
	}
	var hits []ranked
	for _, it := range items {
		c := find.MatchClass(q, it.norm)
		b := find.MatchClass(q, it.baseNorm)
		switch {
		case c != find.ClassNone:
		case b != find.ClassNone && it.Desc != "":
			c = b + 3 // description-only hit ranks below any name hit
		default:
			continue
		}
		if b == find.ClassNone {
			b = 4
		}
		hits = append(hits, ranked{it, c, b, !wordStartHit(it.Name, q)})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.class != b.class {
			return a.class < b.class
		}
		if a.mid != b.mid {
			return !a.mid // a word-start hit beats a mid-word one
		}
		if a.base != b.base {
			return a.base < b.base
		}
		if len(a.it.Name) != len(b.it.Name) {
			return len(a.it.Name) < len(b.it.Name)
		}
		return a.it.Name < b.it.Name
	})
	out := make([]pickItem, 0, min(len(hits), limit))
	for _, h := range hits[:min(len(hits), limit)] {
		out = append(out, h.it)
	}
	return out
}

// wordStartHit reports whether the normalized query q begins at a word start
// of name: its start, after one of "/_-. ", or at a lower-to-upper boundary
// (fooBar). A query with several tokens is compared with spaces dropped.
func wordStartHit(name, q string) bool {
	qr := []rune(strings.ReplaceAll(q, " ", ""))
	if len(qr) == 0 {
		return false
	}
	nr := []rune(name)
	for i := range nr {
		if i > 0 {
			prev, cur := nr[i-1], nr[i]
			if !strings.ContainsRune("/_-. ", prev) && !(unicode.IsLower(prev) && unicode.IsUpper(cur)) {
				continue
			}
		}
		if len(nr)-i < len(qr) {
			break
		}
		ok := true
		for j, r := range qr {
			if unicode.ToLower(nr[i+j]) != r {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// pickerKey handles a key while a picker is open and reports whether it was used.
func (app *AppWidget) pickerKey(name string) bool {
	kb := app.Spec.Keybindings
	p := &app.picker
	switch {
	case bound(name, kb.PickerClose):
		app.closePicker()
	case bound(name, kb.PickerUp):
		p.sel = max(p.sel-1, 0)
	case bound(name, kb.PickerDown):
		p.sel = min(p.sel+1, max(len(p.matches)-1, 0))
	case bound(name, kb.PickerSelect):
		// Enter never sends while a picker is open; with nothing to pick it
		// just closes the picker and keeps the text.
		if p.sel < len(p.matches) {
			app.insertPick(p.matches[p.sel].Name)
		}
		app.closePicker()
	default:
		return false
	}
	return true
}

// insertPick replaces the whole token under the caret, from the trigger to the
// next whitespace (a mid-word pick @fo|o.go swaps the tail too), with the
// trigger and name followed by one space.
func (app *AppWidget) insertPick(name string) {
	p := &app.picker
	line := []rune(app.Prompt.Line(p.row))
	_, col := app.Prompt.Cursor()
	end := col
	for end < len(line) && !unicode.IsSpace(line[end]) {
		end++
	}
	text := app.trigger(p.kind) + quoteName(name)
	if end < len(line) && line[end] == ' ' {
		// Reuse the space that follows instead of doubling it.
		app.Prompt.Replace(p.row, p.col, end, text)
		app.Prompt.MoveRight()
		return
	}
	app.Prompt.Replace(p.row, p.col, end, text+" ")
}

// quoteName wraps names with whitespace in double quotes: @"my file.go".
// This is the form Claude Code reads for @-mentions with spaces and the
// shell-like one other agents (codex) take as a single path argument.
func quoteName(name string) string {
	if strings.IndexFunc(name, unicode.IsSpace) < 0 {
		return name
	}
	return `"` + name + `"`
}

// drawPicker paints the open picker above the prompt divider, its left
// border at the trigger column. area is the chat region it may cover.
func (app *AppWidget) drawPicker(c *loom.Canvas, area loom.Rect) {
	p := &app.picker
	ps := app.Spec.Picker
	title := ps.Labels.FilesTitle
	if p.kind == pickCommand {
		title = ps.Labels.CommandsTitle
	}
	var rows []string
	for _, it := range p.matches {
		if p.kind == pickFile {
			rows = append(rows, it.Name)
		} else {
			rows = append(rows, strings.TrimSpace(ps.CommandTrigger+it.Name+"  "+it.Desc))
		}
	}
	status := ""
	switch {
	case app.pickerLoading():
		status = ps.Labels.Indexing
	case len(rows) == 0 && app.pickerNote() != "":
		status = strings.ReplaceAll(ps.Labels.ReadError, "{{ .Error }}", app.pickerNote())
	case len(rows) == 0:
		status = ps.Labels.NoMatches
	}
	if status != "" && len(rows) < ps.MaxItems {
		rows = append(rows, status)
	} else {
		status = ""
	}
	n := min(len(rows), area.H-2)
	if n < 1 || area.W < 6 {
		return
	}
	w := loom.StringWidth(title) + 4
	for _, r := range rows[:n] {
		w = max(w, loom.StringWidth(r)+2)
	}
	w = min(w, ps.MaxWidth, area.W)
	inner := w - 2
	x := area.X + min(max(app.triggerX(area.W)-1, 0), area.W-w)
	box := loom.Rect{X: x, Y: area.Y + area.H - n - 2, W: w, H: n + 2}
	style, err := loom.ParseBoxBorderStyle(app.Spec.Theme.BorderStyle)
	if err != nil {
		style = loom.BoxBorderStyleSharp
	}
	c.ClearRect(box)
	c.DrawBox(box, style, loom.TruncateText(title, max(inner-2, 1), "…"), loom.Style{})
	first := max(p.sel-n+1, 0)
	for i := 0; i < n && first+i < len(rows); i++ {
		idx := first + i
		var row string
		switch {
		case status != "" && idx == len(rows)-1:
			row = loom.Style{Dim: true}.ANSI() + loom.TruncateText(rows[idx], inner, "…") + "\x1b[0m"
		case p.kind == pickFile:
			row = padTo(loom.TruncateTextLeft(rows[idx], inner, "…"), inner)
		default:
			row = padTo(loom.TruncateText(rows[idx], inner, "…"), inner)
		}
		if idx == p.sel && idx < len(p.matches) {
			row = loom.Style{Invert: true}.ANSI() + row + "\x1b[0m"
		}
		c.WriteANSI(box.X+1, box.Y+1+i, row)
	}
}

func padTo(s string, w int) string {
	return s + strings.Repeat(" ", max(w-loom.StringWidth(s), 0))
}

// triggerX is the screen column of the trigger rune in the prompt box.
func (app *AppWidget) triggerX(width int) int {
	line := []rune(app.Prompt.Line(app.picker.row))
	col := min(app.picker.col, len(line))
	x := loom.StringWidth(app.Spec.Input.PromptSymbol) + loom.StringWidth(string(line[:col]))
	if width > 0 {
		x %= width
	}
	return x
}
