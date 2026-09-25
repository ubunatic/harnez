package bench

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/subagent"
)

// Options selects one bench matrix cell to run over a set of tasks.
type Options struct {
	Agent, Model string
	Cond         Condition
	RepoRoot     string
	Repeat       int
	Run          CommandRunner // defaults to ExecRunner
	ReadMeter    func(sessionID string) ([]agymeter.Record, error)
	OnStart      func(Run)
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
	var model subagent.Model
	var err error
	if o.Model == "" {
		model, err = DefaultModelForProvider(o.Agent)
	} else {
		model, err = subagent.ResolveModel(o.Model)
	}
	if err != nil {
		return err
	}
	if model.Provider != o.Agent {
		return fmt.Errorf("bench: model %q belongs to provider %q, not %q", o.Model, model.Provider, o.Agent)
	}
	modelSpec := model.Spec()
	if modelSpec == "" {
		return fmt.Errorf("bench: unresolved model spec for %q", o.Model)
	}
	for _, task := range tasks {
		for i := 0; i < o.Repeat; i++ {
			run := Run{Task: task.ID, Agent: o.Agent, Model: modelSpec, Docs: o.Cond.Docs, Cards: o.Cond.Cards, ReadMode: o.Cond.ReadVariant(), Order: o.Cond.Order}
			if o.Agent == AgentAgy {
				run.SessionID = newMeterSessionID()
			}
			dir, err := os.MkdirTemp("", "harnez-bench.*")
			if err != nil {
				return err
			}
			start := time.Now()
			callCtx, cancel := context.WithTimeout(ctx, taskTimeout)
			if run.SessionID != "" {
				callCtx = context.WithValue(callCtx, agySessionContextKey{}, run.SessionID)
			}
			if o.OnStart != nil {
				o.OnStart(run)
			}
			if _, err := StageWorkspace(dir, o.RepoRoot, spec, task, o.Cond); err != nil {
				run.Error = err.Error()
			} else if res, err := Invoke(callCtx, o.Run, o.Agent, modelSpec, dir, TaskPrompt(task, o.Cond.Read, o.Cond.Order)); err != nil {
				run.Error = err.Error()
			} else {
				run.Response, run.InputTokens, run.OutputTokens, run.CostUSD = res.Text, res.InputTokens, res.OutputTokens, res.CostUSD
				run.Turns = res.Turns
				run.CachedInputTokens, run.CachedEstimated = res.CachedInputTokens, res.CachedEstimated
				run.TotalTokens = res.InputTokens + res.OutputTokens
				if o.Agent == AgentAgy {
					reader := o.ReadMeter
					if reader == nil {
						reader = readAgyUsage
					}
					rows, err := reader(run.SessionID)
					if err != nil {
						run.Error = fmt.Sprintf("read agy meter: %v", err)
					} else if len(rows) == 0 {
						run.Error = "agy meter recorded no usage for this session"
					} else {
						input, total, turns, calls, helpers := splitAgyUsage(rows)
						run.InputTokens, run.TotalTokens, run.Turns = input, total, turns
						run.MainCallTokens, run.HelperUsage = calls, helpers
						cached := int64(0)
						mainModel := mainUsageModel(rows)
						for _, row := range rows {
							if row.Model == mainModel {
								cached += row.Cached
							}
						}
						if cached > 0 {
							run.CachedInputTokens = intPointer(int(cached))
							run.CachedEstimated = false
						} else if estimate, ok := estimateCached(calls); ok {
							run.CachedInputTokens = intPointer(estimate)
							run.CachedEstimated = true
						}
						run.OutputTokens = total - input
						if run.OutputTokens < 0 {
							run.OutputTokens = 0
						}
					}
				}
				if run.Error == "" {
					run.Pass, run.Detail = task.Score(res.Text)
				}
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

// TaskPrompt returns the prompt for a read mode, or the task's generic prompt.
func TaskPrompt(task Task, mode string, order ...string) string {
	prompt := task.Prompt
	if modePrompt, ok := task.ReadPrompts[mode]; ok {
		prompt = modePrompt
	}
	if len(order) > 0 {
		if sentence := task.ReadOrders[order[0]]; sentence != "" {
			prompt = strings.TrimSpace(prompt + " " + sentence)
		}
	}
	return prompt
}
