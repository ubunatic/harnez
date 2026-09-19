package bench

import (
	"context"
	"os"
	"time"
)

// Options selects one bench matrix cell to run over a set of tasks.
type Options struct {
	Agent, Model string
	Cond         Condition
	RepoRoot     string
	Repeat       int
	Run          CommandRunner // defaults to ExecRunner
}

// taskTimeout bounds one agent invocation.
const taskTimeout = 5 * time.Minute

// RunTasks stages a fresh workspace per task, invokes the agent, scores the
// response and records each run in store. Invocation failures are recorded
// with Error set and do not abort the remaining tasks.
func RunTasks(ctx context.Context, store *Store, spec *Spec, tasks []Task, o Options, progress func(Run)) error {
	if o.Run == nil {
		o.Run = ExecRunner
	}
	if o.Repeat < 1 {
		o.Repeat = 1
	}
	model := ResolveModel(o.Agent, o.Model)
	for _, task := range tasks {
		for i := 0; i < o.Repeat; i++ {
			run := Run{Task: task.ID, Agent: o.Agent, Model: model, Docs: o.Cond.Docs, Cards: o.Cond.Cards, ReadMode: o.Cond.Read}
			dir, err := os.MkdirTemp("", "harnez-bench.*")
			if err != nil {
				return err
			}
			start := time.Now()
			callCtx, cancel := context.WithTimeout(ctx, taskTimeout)
			if _, err := StageWorkspace(dir, o.RepoRoot, spec, task, o.Cond); err != nil {
				run.Error = err.Error()
			} else if res, err := Invoke(callCtx, o.Run, o.Agent, model, dir, task.Prompt); err != nil {
				run.Error = err.Error()
			} else {
				run.Response, run.InputTokens, run.OutputTokens, run.CostUSD = res.Text, res.InputTokens, res.OutputTokens, res.CostUSD
				run.Turns = res.Turns
				run.Pass, run.Detail = task.Score(res.Text)
			}
			cancel()
			run.DurationMS = time.Since(start).Milliseconds()
			_ = os.RemoveAll(dir)
			if err := store.Insert(run); err != nil {
				return err
			}
			if progress != nil {
				progress(run)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	return nil
}
