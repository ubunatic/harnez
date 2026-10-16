// Package jobs runs background shell commands for harnez tui.
//
// Each job runs in its own process group (subagent.IsolateProcess) with
// stdout and stderr captured into a bounded Ring. Job state is owned by the
// UI goroutine: worker goroutines never touch a Job, they hand every change
// to the post function (loom Pane.Post), which runs it on the UI loop.
package jobs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

// Status is the lifecycle state of a job.
type Status int

// Job states; every job starts Running and ends in one of the others.
const (
	Running Status = iota
	Succeeded
	Failed
	Cancelled
)

func (s Status) String() string {
	switch s {
	case Running:
		return "running"
	case Succeeded:
		return "succeeded"
	case Failed:
		return "failed"
	case Cancelled:
		return "cancelled"
	}
	return "unknown"
}

// Options configures a Manager; the TUI fills it from spec/tui.yaml.
type Options struct {
	Shell       string        // shell binary run as <Shell> -c <command>
	Dir         string        // working directory of every job
	LogBytes    int           // ring buffer size per job
	CancelGrace time.Duration // SIGTERM to SIGKILL delay; also bounds output drain
	MaxFinished int           // ended jobs kept in the registry; older ones are pruned
}

// Job is one background command. Read its fields on the UI goroutine only.
type Job struct {
	ID       int
	Command  string
	PID      int
	Start    time.Time
	End      time.Time
	Status   Status
	ExitCode int
	Err      error // start or wait error other than a plain exit status

	out       *Ring
	pending   *Ring // output read but not yet on the UI loop; guarded by mu
	mu        sync.Mutex
	posted    bool // a drain callback is queued; guarded by mu
	cancelled bool
	done      chan struct{} // closed when the process group is gone
}

// Lines returns the captured output as display-safe lines.
func (j *Job) Lines() []string { return j.out.Lines() }

// Duration is the run time so far, or the final run time once ended.
func (j *Job) Duration(now time.Time) time.Duration {
	if j.Status == Running {
		return now.Sub(j.Start)
	}
	return j.End.Sub(j.Start)
}

// Manager is the job registry. Its methods must run on the UI goroutine,
// except Shutdown, which runs after the UI loop has stopped.
type Manager struct {
	opt    Options
	post   func(func())
	jobs   []*Job
	nextID int
	wg     sync.WaitGroup
	now    func() time.Time

	// OnDone runs on the UI goroutine when a job ends.
	OnDone func(*Job)
	// OnOutput runs on the UI goroutine after output was appended to j.
	OnOutput func(*Job)
}

// NewManager returns an empty registry; post must run callbacks on the UI loop.
func NewManager(opt Options, post func(func())) *Manager {
	return &Manager{opt: opt, post: post, nextID: 1, now: time.Now}
}

// Jobs returns all jobs, oldest first.
func (m *Manager) Jobs() []*Job { return m.jobs }

// Get returns the job with id, or nil.
func (m *Manager) Get(id int) *Job {
	for _, j := range m.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// Running counts jobs that have not ended.
func (m *Manager) Running() int {
	n := 0
	for _, j := range m.jobs {
		if j.Status == Running {
			n++
		}
	}
	return n
}

// Start runs command in the background and registers it as Running.
func (m *Manager) Start(command string) (*Job, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("job output pipe: %w", err)
	}
	cmd := exec.Command(m.opt.Shell, "-c", command)
	cmd.Dir = m.opt.Dir
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = w, w
	subagent.IsolateProcess(cmd)
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return nil, fmt.Errorf("start job: %w", err)
	}
	w.Close() // the children hold the write end now
	j := &Job{
		ID:      m.nextID,
		Command: command,
		PID:     cmd.Process.Pid,
		Start:   m.now(),
		Status:  Running,
		out:     NewRing(m.opt.LogBytes),
		pending: NewRing(m.opt.LogBytes),
		done:    make(chan struct{}),
	}
	m.nextID++
	m.jobs = append(m.jobs, j)

	readDone := make(chan struct{})
	m.wg.Add(2)
	go func() {
		defer m.wg.Done()
		defer close(readDone)
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				m.queueOutput(j, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer m.wg.Done()
		werr := cmd.Wait()
		// The job ends with its leader; leftover group members would be
		// orphans, so they go too.
		_ = subagent.KillGroup(j.PID, syscall.SIGKILL)
		close(j.done)
		select {
		case <-readDone:
		case <-time.After(m.opt.CancelGrace):
			// A member escaped the group (setsid) and still holds the pipe.
		}
		r.Close()
		<-readDone
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		var exitErr *exec.ExitError
		if errors.As(werr, &exitErr) {
			werr = nil
		}
		end := m.now()
		m.post(func() { m.finish(j, code, werr, end) })
	}()
	return j, nil
}

// queueOutput coalesces output read off the UI loop: it lands in the
// bounded pending ring and at most one drain callback is queued per job, so
// a fast producer cannot grow the Post queue while the loop is paused.
func (m *Manager) queueOutput(j *Job, p []byte) {
	j.mu.Lock()
	_, _ = j.pending.Write(p)
	queue := !j.posted
	j.posted = true
	j.mu.Unlock()
	if queue {
		m.post(func() { m.drainOutput(j) })
	}
}

// drainOutput moves pending output into the job's ring on the UI loop.
func (m *Manager) drainOutput(j *Job) {
	j.mu.Lock()
	data, dropped := j.pending.Bytes(), j.pending.Truncated()
	j.pending.Reset()
	j.posted = false
	j.mu.Unlock()
	if dropped {
		j.out.dropped = true
	}
	_, _ = j.out.Write(data)
	if m.OnOutput != nil {
		m.OnOutput(j)
	}
}

func (m *Manager) finish(j *Job, code int, err error, end time.Time) {
	j.End, j.ExitCode, j.Err = end, code, err
	switch {
	case j.cancelled:
		j.Status = Cancelled
	case code == 0 && err == nil:
		j.Status = Succeeded
	default:
		j.Status = Failed
	}
	if m.OnDone != nil {
		m.OnDone(j)
	}
	m.prune()
}

// prune drops the oldest ended jobs beyond MaxFinished; running jobs stay.
func (m *Manager) prune() {
	ended := 0
	for _, j := range m.jobs {
		if j.Status != Running {
			ended++
		}
	}
	kept := m.jobs[:0]
	for _, j := range m.jobs {
		if j.Status != Running && ended > m.opt.MaxFinished {
			ended--
			continue
		}
		kept = append(kept, j)
	}
	clear(m.jobs[len(kept):])
	m.jobs = kept
}

// Cancel stops a running job: SIGTERM to its process group, then SIGKILL
// after the grace period. The job turns Cancelled when its end is posted.
func (m *Manager) Cancel(id int) bool {
	j := m.Get(id)
	if j == nil || j.Status != Running || j.cancelled {
		return false
	}
	j.cancelled = true
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.terminate(j)
	}()
	return true
}

func (m *Manager) terminate(j *Job) {
	select {
	case <-j.done:
		return
	default:
	}
	_ = subagent.KillGroup(j.PID, syscall.SIGTERM)
	select {
	case <-j.done:
	case <-time.After(m.opt.CancelGrace):
		_ = subagent.KillGroup(j.PID, syscall.SIGKILL)
		<-j.done
	}
}

// Shutdown terminates every running job and waits until all process groups
// and worker goroutines are gone. Run it after the UI loop has stopped;
// posted callbacks that never run are fine.
func (m *Manager) Shutdown() {
	var wg sync.WaitGroup
	for _, j := range m.jobs {
		if j.Status != Running {
			continue
		}
		j.cancelled = true
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.terminate(j)
		}()
	}
	wg.Wait()
	m.wg.Wait()
}
