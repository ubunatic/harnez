package tui

import (
	"slices"
	"strings"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/loom"
)

// The model picker is the pickModel kind of the inline picker: same state,
// ranking and picker_* keys, but its query is typed into the overlay instead
// of the prompt, and Enter switches the model through setModel, the code path
// of "/model <spec>".

// AttachSessions lets the model picker mark models that already have a
// backend session in this tui session; lookup is router.Router.Session.
func (app *AppWidget) AttachSessions(lookup func(spec string) string) { app.sessionOf = lookup }

// modelEntries lists the spec/agent.yaml models, one entry per tier.
func (app *AppWidget) modelEntries() []subagent.ModelEntry {
	if app.models != nil {
		return app.models()
	}
	return subagent.KnownModelEntries()
}

func (app *AppWidget) modelItems() []pickItem {
	entries := app.modelEntries()
	items := make([]pickItem, len(entries))
	for i, e := range entries {
		items[i] = newPickItem(e.Spec, oneLine(e.Use), e.Use)
	}
	return items
}

func (app *AppWidget) openModelPicker() {
	app.picker = picker{kind: pickModel}
	app.refilter()
	if i := slices.IndexFunc(app.picker.matches, func(it pickItem) bool { return it.Name == app.Model }); i >= 0 {
		app.picker.sel = i
	}
}

// filterModels ranks items by query, then regroups the hits by provider in
// spec order; the rank order is kept inside each group.
func filterModels(items []pickItem, query string) []pickItem {
	hits := rankItems(items, query, len(items))
	order := map[string]int{}
	for _, it := range items {
		if _, ok := order[providerOf(it.Name)]; !ok {
			order[providerOf(it.Name)] = len(order)
		}
	}
	slices.SortStableFunc(hits, func(a, b pickItem) int { return order[providerOf(a.Name)] - order[providerOf(b.Name)] })
	return hits
}

func providerOf(spec string) string {
	p, _, _ := strings.Cut(spec, ":")
	return p
}

// modelKey handles a key the picker_* bindings left over while the model
// picker is open. It reports false for quit and cancel-turn keys, which pass
// through to the prompt handler; every other key stays in the modal overlay.
func (app *AppWidget) modelKey(e loom.KeyEvent, name string) bool {
	kb := app.Spec.Keybindings
	p := &app.picker
	switch {
	case bound(name, kb.OpenModels):
		app.closePicker()
	case bound(name, kb.Quit) || bound(name, kb.CancelTurn):
		return false
	case name == "backspace":
		if q := []rune(p.query); len(q) > 0 {
			app.setModelQuery(string(q[:len(q)-1]))
		}
	case e.Key == "" && e.Text != "":
		app.setModelQuery(p.query + oneLine(e.Text))
	}
	return true
}

func (app *AppWidget) setModelQuery(q string) {
	app.picker.query, app.picker.sel = q, 0
	app.refilter()
}

// setModel validates spec and makes it the active model; the header reads
// app.Model, and a chat note confirms the switch. A running turn keeps the
// model it started with, so a switch while busy applies to the next turn.
func (app *AppWidget) setModel(spec string) {
	if app.resolve != nil {
		resolved, err := app.resolve(spec)
		if err != nil {
			AddTurn(app.Chat, RoleError, "", err.Error())
			return
		}
		spec = resolved
	}
	app.Model = spec
	labels := app.Spec.ModelPicker.Labels
	label := labels.Switched
	if app.Busy() {
		label = labels.SwitchedBusy
	}
	AddTurn(app.Chat, RoleTool, "model", strings.ReplaceAll(label, "{{ .Model }}", spec))
}

// modelRow formats one model row: active and session markers, spec padded to
// specW columns, then the short description.
func (app *AppWidget) modelRow(it pickItem, specW int) string {
	mp := app.Spec.ModelPicker
	mark := func(on bool, m string) string {
		if on {
			return m
		}
		return strings.Repeat(" ", loom.StringWidth(m))
	}
	hasSession := app.sessionOf != nil && app.sessionOf(it.Name) != ""
	row := "  " + mark(it.Name == app.Model, mp.ActiveMarker) + " " + mark(hasSession, mp.SessionMarker) + " " + padTo(it.Name, specW)
	if it.Desc != "" {
		row += "  " + it.Desc
	}
	return row
}

// drawModelPicker paints the model overlay at the bottom of area (the chat
// region): query row, provider-grouped model rows, hint row.
func (app *AppWidget) drawModelPicker(c *loom.Canvas, area loom.Rect) {
	mp := app.Spec.ModelPicker
	p := &app.picker
	w := min(mp.MaxWidth, area.W)
	avail := area.H - 4 // borders, query row, hint row
	if avail < 1 || w < 20 {
		return
	}
	type line struct {
		text string
		item int // index into p.matches; -1 for a heading or status row
	}
	specW := 0
	for _, it := range p.matches {
		specW = max(specW, loom.StringWidth(it.Name))
	}
	var lines []line
	selLine, prev := 0, ""
	for i, it := range p.matches {
		if prov := providerOf(it.Name); prov != prev {
			lines = append(lines, line{prov, -1})
			prev = prov
		}
		if i == p.sel {
			selLine = len(lines)
		}
		lines = append(lines, line{app.modelRow(it, specW), i})
	}
	if len(lines) == 0 {
		lines = append(lines, line{mp.Labels.NoMatches, -1})
	}
	n := min(len(lines), avail)
	inner := w - 2
	box := loom.Rect{X: area.X, Y: area.Y + area.H - n - 4, W: w, H: n + 4}
	style, err := loom.ParseBoxBorderStyle(app.Spec.Theme.BorderStyle)
	if err != nil {
		style = loom.BoxBorderStyleSharp
	}
	c.ClearRect(box)
	c.DrawBox(box, style, loom.TruncateText(mp.Title, max(inner-2, 1), "…"), loom.Style{})
	query := loom.TruncateTextLeft(app.Spec.Input.PromptSymbol+p.query, inner, "…")
	c.WriteANSI(box.X+1, box.Y+1, padTo(query, inner))
	dim, invert := loom.Style{Dim: true}.ANSI(), loom.Style{Invert: true}.ANSI()
	first := min(max(selLine-n/2, 0), len(lines)-n) // keep the selection mid-list
	for i := 0; i < n; i++ {
		l := lines[first+i]
		row := padTo(loom.TruncateText(l.text, inner, "…"), inner)
		switch {
		case l.item < 0:
			row = dim + row + "\x1b[0m"
		case l.item == p.sel:
			row = invert + row + "\x1b[0m"
		}
		c.WriteANSI(box.X+1, box.Y+2+i, row)
	}
	c.WriteANSI(box.X+1, box.Y+box.H-2, dim+padTo(loom.TruncateText(mp.Hint, inner, "…"), inner)+"\x1b[0m")
}
