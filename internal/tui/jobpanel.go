package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ubunatic.com/harnez/internal/tui/jobs"
	"ubunatic.com/loom"
)

// NewJobManager returns the background job registry for spec; post must run
// callbacks on the UI loop (loom Pane.Post).
func NewJobManager(spec JobsSpec, dir string, post func(func())) *jobs.Manager {
	return jobs.NewManager(jobs.Options{
		Shell:       spec.Shell,
		Dir:         dir,
		LogBytes:    spec.LogBufferBytes,
		CancelGrace: time.Duration(spec.CancelGraceMS) * time.Millisecond,
	}, post)
}

// AttachJobs enables "<prefix><command>" prompts and the job panel.
func (app *AppWidget) AttachJobs(m *jobs.Manager) {
	app.jobs = m
	m.OnDone = app.jobDone
	m.OnOutput = func(j *jobs.Job) {
		if app.panel.logID == j.ID {
			app.panel.dirty = true
		}
	}
}

// jobPanel is the job overlay: a job list, or the log of one job.
type jobPanel struct {
	open   bool
	sel    int // index into the job list
	logID  int // job shown in the log view; 0 shows the list
	log    *loom.View
	dirty  bool // log lines need a refresh
	follow bool // log view sticks to the tail
	logH   int  // log rows at the last draw
}

// parseJobPrompt reports whether text starts with prefix (after leading
// spaces) and returns the trimmed command after it.
func parseJobPrompt(text, prefix string) (string, bool) {
	t := strings.TrimLeftFunc(text, unicode.IsSpace)
	if prefix == "" || !strings.HasPrefix(t, prefix) {
		return "", false
	}
	return strings.TrimSpace(t[len(prefix):]), true
}

func (app *AppWidget) startJob(command string) {
	labels := app.Spec.Jobs.Labels
	if command == "" {
		AddTurn(app.Chat, RoleError, "", labels.Usage)
		return
	}
	if app.jobs == nil {
		AddTurn(app.Chat, RoleError, "", "background jobs need an interactive terminal")
		return
	}
	j, err := app.jobs.Start(command)
	if err != nil {
		AddTurn(app.Chat, RoleError, "", err.Error())
		return
	}
	AddTurn(app.Chat, RoleNote, "", app.jobLabel(labels.Started, j))
}

func (app *AppWidget) jobDone(j *jobs.Job) {
	AddTurn(app.Chat, RoleNote, "", app.jobLabel(app.Spec.Jobs.Labels.Done, j))
	if app.panel.logID == j.ID {
		app.panel.dirty = true
	}
}

func (app *AppWidget) jobLabel(tmpl string, j *jobs.Job) string {
	exit := strconv.Itoa(j.ExitCode)
	if j.Status != jobs.Running && j.ExitCode < 0 {
		exit = "signal"
	}
	if j.Err != nil {
		exit = j.Err.Error()
	}
	r := strings.NewReplacer(
		"{{ .ID }}", strconv.Itoa(j.ID),
		"{{ .Command }}", j.Command,
		"{{ .Status }}", j.Status.String(),
		"{{ .Exit }}", exit,
		"{{ .Duration }}", formatDuration(j.Duration(time.Now())),
	)
	return r.Replace(tmpl)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

func (app *AppWidget) runningJobs() int {
	if app.jobs == nil {
		return 0
	}
	return app.jobs.Running()
}

func (app *AppWidget) jobList() []*jobs.Job {
	if app.jobs == nil {
		return nil
	}
	return app.jobs.Jobs()
}

func (app *AppWidget) toggleJobs() {
	p := &app.panel
	p.open = !p.open
	p.logID = 0
	if n := len(app.jobList()); p.open && n > 0 {
		p.sel = n - 1 // newest job
	}
}

// panelKey handles a key while the job panel is open and reports whether
// it was used.
func (app *AppWidget) panelKey(e loom.KeyEvent) bool {
	kb := app.Spec.Keybindings
	p := &app.panel
	name := keyName(e)
	if name == "" {
		name = e.Text
	}
	list := app.jobList()
	switch {
	case bound(name, kb.ToggleJobs):
		app.toggleJobs()
	case bound(name, kb.JobClose):
		if p.logID != 0 {
			p.logID = 0
		} else {
			p.open = false
		}
	case p.logID != 0:
		return app.logKey(name)
	case name == "up":
		if p.sel > 0 {
			p.sel--
		}
	case name == "down":
		if p.sel < len(list)-1 {
			p.sel++
		}
	case bound(name, kb.JobOpen):
		if p.sel < len(list) {
			p.logID = list[p.sel].ID
			p.log = loom.NewView(nil)
			p.dirty, p.follow = true, true
		}
	case bound(name, kb.JobCancel):
		if p.sel < len(list) {
			app.jobs.Cancel(list[p.sel].ID)
		}
	default:
		return false
	}
	return true
}

func (app *AppWidget) logKey(name string) bool {
	kb := app.Spec.Keybindings
	p := &app.panel
	page := max(p.logH-1, 1)
	switch {
	case bound(name, kb.JobCancel):
		app.jobs.Cancel(p.logID)
	case name == "up":
		app.scrollLog(-1)
	case name == "down":
		app.scrollLog(1)
	case bound(name, kb.ScrollUp):
		app.scrollLog(-page)
	case bound(name, kb.ScrollDown):
		app.scrollLog(page)
	case name == "end":
		p.follow = true
	case name == "home":
		p.follow = false
		p.log.Scroll = 0
	default:
		return false
	}
	return true
}

func (app *AppWidget) scrollLog(delta int) {
	p := &app.panel
	bottom := max(len(p.log.Lines)-p.logH, 0)
	if p.follow {
		p.log.Scroll = bottom
	}
	p.log.Scroll = min(max(p.log.Scroll+delta, 0), bottom)
	p.follow = p.log.Scroll == bottom
}

// drawJobCount right-aligns the running-jobs label on the header row, so a
// long header never clips it.
func (app *AppWidget) drawJobCount(c *loom.Canvas, r loom.Rect) {
	n := app.runningJobs()
	if n == 0 {
		return
	}
	label := " " + strings.ReplaceAll(app.Spec.Jobs.Labels.HeaderRunning, "{{ .Running }}", strconv.Itoa(n))
	w := loom.StringWidth(label)
	if w >= r.W {
		return
	}
	x := r.X + r.W - w
	c.ClearRect(loom.Rect{X: x, Y: r.Y, W: w, H: 1})
	c.WriteANSI(x, r.Y, label)
}

// drawJobPanel paints the job overlay over area (the chat region).
func (app *AppWidget) drawJobPanel(c *loom.Canvas, area loom.Rect) {
	if area.W < 8 || area.H < 4 {
		return
	}
	labels := app.Spec.Jobs.Labels
	p := &app.panel
	var job *jobs.Job
	if p.logID != 0 && app.jobs != nil {
		job = app.jobs.Get(p.logID)
	}
	title, hint := labels.PanelTitle, labels.PanelHint
	if job != nil {
		title, hint = app.jobLabel(labels.LogTitle, job), labels.LogHint
	}
	style, err := loom.ParseBoxBorderStyle(app.Spec.Theme.BorderStyle)
	if err != nil {
		style = loom.BoxBorderStyleSharp
	}
	c.ClearRect(area)
	c.DrawBox(area, style, title, loom.Style{})
	inner := loom.Rect{X: area.X + 1, Y: area.Y + 1, W: area.W - 2, H: area.H - 2}
	body := loom.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: inner.H - 1}
	dim := loom.Style{Dim: true}
	c.WriteANSI(inner.X, inner.Y+inner.H-1, dim.ANSI()+loom.TruncateText(hint, inner.W, "…")+"\x1b[0m")
	if job != nil {
		app.drawJobLog(c, body, job)
		return
	}
	list := app.jobList()
	if len(list) == 0 {
		c.WriteANSI(body.X, body.Y, loom.TruncateText(labels.PanelEmpty, body.W, "…"))
		return
	}
	p.sel = min(max(p.sel, 0), len(list)-1)
	first := max(p.sel-body.H+1, 0)
	now := time.Now()
	for i := first; i < len(list) && i-first < body.H; i++ {
		row := jobRow(list[i], body.W, now)
		if i == p.sel {
			row = loom.Style{Invert: true}.ANSI() + row + "\x1b[0m"
		}
		c.WriteANSI(body.X, body.Y+i-first, row)
	}
}

// jobRow formats one list row padded to width display columns.
func jobRow(j *jobs.Job, width int, now time.Time) string {
	head := fmt.Sprintf("%3d  %-9s %7s  ", j.ID, j.Status, formatDuration(j.Duration(now)))
	row := head + loom.TruncateText(j.Command, max(width-loom.StringWidth(head), 0), "…")
	row = loom.TruncateText(row, width, "…")
	return row + strings.Repeat(" ", max(width-loom.StringWidth(row), 0))
}

func (app *AppWidget) drawJobLog(c *loom.Canvas, body loom.Rect, j *jobs.Job) {
	p := &app.panel
	if p.log == nil {
		p.log = loom.NewView(nil)
	}
	if p.dirty {
		p.log.Lines = j.Lines()
		p.dirty = false
	}
	p.logH = body.H
	if p.follow {
		p.log.Scroll = max(len(p.log.Lines)-body.H, 0)
	}
	p.log.Draw(c, body)
}
