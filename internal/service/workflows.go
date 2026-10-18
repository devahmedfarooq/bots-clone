package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"bots/internal/state"
	"bots/internal/workflow"
)

type StepResult struct {
	Status  string `json:"status"`
	Version string `json:"bot_version"`
	Job     string `json:"job,omitempty"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}
type Flow struct {
	ID         string                `json:"id"`
	Definition workflow.Definition   `json:"definition"`
	Status     string                `json:"status"`
	Steps      map[string]StepResult `json:"steps"`
	Created    time.Time             `json:"created"`
	Error      string                `json:"error,omitempty"`
}
type Workflows struct {
	mu      sync.Mutex
	flows   map[string]Flow
	cancels map[string]context.CancelFunc
	q       *Queue
	wg      sync.WaitGroup
	closed  bool
}

func NewWorkflows(q *Queue) (*Workflows, error) {
	w := &Workflows{flows: map[string]Flow{}, cancels: map[string]context.CancelFunc{}, q: q}
	data, err := q.runner.Store.Checkpoints("workflow")
	if err != nil {
		return nil, err
	}
	for _, raw := range data {
		var f Flow
		if err = json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		if f.Status == "running" {
			f.Status = "interrupted"
			for id, step := range f.Steps {
				if step.Status == "running" {
					job, ok := q.Get(step.Job)
					if ok && job.Status == "completed" && job.Run != nil {
						step.Status = "completed"
						step.Output = job.Run.Output
					} else {
						step.Status = "uncertain"
						step.Error = "Inspect the saved job and tool trace; this step cannot be replayed automatically."
					}
					f.Steps[id] = step
				}
			}
			if err = q.runner.Store.Checkpoint("workflow", f.ID, f); err != nil {
				return nil, err
			}
		}
		w.flows[f.ID] = f
	}
	return w, nil
}

func (w *Workflows) save(f Flow) error {
	if err := w.q.runner.Store.Checkpoint("workflow", f.ID, f); err != nil {
		f.Status = "failed"
		f.Error = state.Redact(err.Error())
		w.mu.Lock()
		w.flows[f.ID] = cloneFlow(f)
		w.mu.Unlock()
		return err
	}
	w.mu.Lock()
	w.flows[f.ID] = cloneFlow(f)
	w.mu.Unlock()
	return nil
}

func (w *Workflows) Start(d workflow.Definition) (Flow, error) {
	ordered, err := d.Validate(w.q.catalog.Registry)
	if err != nil {
		return Flow{}, err
	}
	f := Flow{ID: state.ID(), Definition: d, Status: "running", Created: time.Now().UTC(), Steps: map[string]StepResult{}}
	for _, step := range ordered {
		b, err := w.q.catalog.Registry.Get(step.Bot)
		if err != nil {
			return Flow{}, err
		}
		f.Steps[step.ID] = StepResult{Status: "pending", Version: b.Version}
	}
	w.mu.Lock()
	if w.closed || len(w.cancels) >= 8 {
		w.mu.Unlock()
		return Flow{}, errors.New("workflow capacity reached or service stopped")
	}
	ctx, cancel := context.WithCancel(w.q.ctx)
	w.cancels[f.ID] = cancel
	w.wg.Add(1)
	w.mu.Unlock()
	if err = w.save(f); err != nil {
		cancel()
		w.mu.Lock()
		delete(w.cancels, f.ID)
		w.mu.Unlock()
		w.wg.Done()
		return Flow{}, err
	}
	initial := cloneFlow(f)
	go w.execute(ctx, f, ordered)
	return initial, nil
}

func cloneFlow(f Flow) Flow {
	data, _ := state.Sanitized(f)
	var copy Flow
	_ = json.Unmarshal(data, &copy)
	return copy
}

func (w *Workflows) execute(ctx context.Context, f Flow, ordered []workflow.Step) {
	defer w.wg.Done()
	defer func() {
		w.mu.Lock()
		if cancel := w.cancels[f.ID]; cancel != nil {
			cancel()
		}
		delete(w.cancels, f.ID)
		w.mu.Unlock()
	}()
	for _, step := range ordered {
		result := f.Steps[step.ID]
		if result.Status == "completed" {
			continue
		}
		if ctx.Err() != nil {
			f.Status = "interrupted"
			f.Error = "Workflow stopped before next step."
			_ = w.save(f)
			return
		}
		blocked := false
		for _, id := range step.DependsOn {
			if f.Steps[id].Status != "completed" {
				blocked = true
			}
		}
		if blocked {
			result.Status = "blocked"
			f.Steps[step.ID] = result
			if err := w.save(f); err != nil {
				return
			}
			continue
		}
		b, err := w.q.catalog.Registry.Get(step.Bot)
		if err != nil || b.Version != result.Version {
			f.Status = "failed"
			f.Error = "Bot changed or is unavailable; submit a new workflow."
			_ = w.save(f)
			return
		}
		input := step.Input
		if step.InputFrom != "" {
			input = f.Steps[step.InputFrom].Output
		}
		if err = workflow.Value(step.InputType, input); err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			f.Steps[step.ID] = result
			_ = w.save(f)
			continue
		}
		result.Status = "running"
		f.Steps[step.ID] = result
		if err = w.save(f); err != nil {
			return
		}
		for attempt := 0; attempt <= step.Retries; attempt++ {
			var job Job
			for {
				job, err = w.q.Submit(Request{Bot: step.Bot, Input: input})
				if err == nil || err.Error() != "queue full" {
					break
				}
				select {
				case <-ctx.Done():
					err = ctx.Err()
				case <-time.After(50 * time.Millisecond):
					continue
				}
				break
			}
			if err != nil {
				result.Status = "failed"
				result.Error = state.Redact(err.Error())
				break
			}
			result.Job = job.ID
			f.Steps[step.ID] = result
			if err = w.save(f); err != nil {
				_ = w.q.Cancel(job.ID)
				return
			}
			for job.Status == "queued" || job.Status == "running" {
				select {
				case <-ctx.Done():
					_ = w.q.Cancel(job.ID)
					result.Status = "uncertain"
					result.Error = "Cancelled; inspect tool actions before resubmitting."
					f.Steps[step.ID] = result
					f.Status = "interrupted"
					_ = w.save(f)
					return
				case <-time.After(50 * time.Millisecond):
					job, _ = w.q.Get(job.ID)
				}
			}
			if job.Status == "completed" && job.Run != nil {
				if err = workflow.Value(step.OutputType, job.Run.Output); err == nil {
					result.Status = "completed"
					result.Output = job.Run.Output
					result.Error = ""
					break
				}
				result.Error = err.Error()
			} else {
				result.Error = job.Error
			}
			result.Status = "failed"
		}
		f.Steps[step.ID] = result
		if err = w.save(f); err != nil {
			return
		}
	}
	f.Status = "completed"
	for _, result := range f.Steps {
		if result.Status != "completed" {
			f.Status = "failed"
		}
	}
	if err := w.save(f); err != nil {
		f.Status = "failed"
		f.Error = state.Redact(err.Error())
		w.mu.Lock()
		w.flows[f.ID] = cloneFlow(f)
		w.mu.Unlock()
	}
}

func (w *Workflows) List() []Flow {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []Flow{}
	for _, f := range w.flows {
		out = append(out, cloneFlow(f))
	}
	return out
}
func (w *Workflows) Get(id string) (Flow, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, ok := w.flows[id]
	return cloneFlow(f), ok
}
func (w *Workflows) Cancel(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.flows[id]; !ok {
		return errors.New("workflow not found")
	}
	if cancel := w.cancels[id]; cancel != nil {
		cancel()
	}
	return nil
}
func (w *Workflows) Close() {
	w.mu.Lock()
	w.closed = true
	for _, cancel := range w.cancels {
		cancel()
	}
	w.mu.Unlock()
	w.wg.Wait()
}

func (w *Workflows) Resume(id string) (Flow, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, ok := w.flows[id]
	if !ok {
		return Flow{}, errors.New("workflow not found")
	}
	if w.closed || w.q.ctx.Err() != nil || len(w.cancels) >= 8 {
		return Flow{}, errors.New("service stopped or workflow capacity reached")
	}
	if f.Status != "interrupted" {
		return Flow{}, errors.New("only interrupted workflows can resume")
	}
	ordered, err := f.Definition.Validate(w.q.catalog.Registry)
	if err != nil {
		return Flow{}, err
	}
	for _, step := range ordered {
		result := f.Steps[step.ID]
		if result.Status != "pending" && result.Status != "completed" {
			return Flow{}, errors.New("workflow has uncertain or failed steps; inspect traces and submit a new definition")
		}
		b, err := w.q.catalog.Registry.Get(step.Bot)
		if err != nil || b.Version != result.Version {
			return Flow{}, errors.New("bot version changed")
		}
		if result.Status == "completed" {
			if err = workflow.Value(step.OutputType, result.Output); err != nil {
				return Flow{}, err
			}
		}
	}
	f = cloneFlow(f)
	f.Status = "running"
	f.Error = ""
	if err = w.q.runner.Store.Checkpoint("workflow", f.ID, f); err != nil {
		return Flow{}, err
	}
	w.flows[id] = cloneFlow(f)
	ctx, cancel := context.WithCancel(w.q.ctx)
	w.cancels[id] = cancel
	w.wg.Add(1)
	initial := cloneFlow(f)
	go w.execute(ctx, f, ordered)
	return initial, nil
}
