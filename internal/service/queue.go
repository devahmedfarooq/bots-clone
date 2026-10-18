package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"bots/internal/application"
	"bots/internal/config"
	"bots/internal/diagnostics"
	"bots/internal/runner"
	"bots/internal/state"
)

type Request struct {
	AdaptiveContext bool            `json:"adaptive_context,omitempty"`
	Bot             string          `json:"bot"`
	Input           string          `json:"input"`
	Settings        config.Settings `json:"settings,omitempty"`
	Version         string          `json:"bot_version,omitempty"`
	Budgets         *config.Budgets `json:"budgets,omitempty"`
	ReadOnly        bool            `json:"read_only,omitempty"`
}

var ErrQueueFull = errors.New("queue full")

type Job struct {
	DispatchKey string              `json:"dispatch_key,omitempty"`
	ID          string              `json:"id"`
	Bot         string              `json:"bot"`
	Status      string              `json:"status"`
	Created     time.Time           `json:"created"`
	Run         *state.Run          `json:"run,omitempty"`
	Error       string              `json:"error,omitempty"`
	Resources   *diagnostics.Sample `json:"resources,omitempty"`
}

type queued struct {
	id      string
	bot     config.Bot
	request Request
	ctx     context.Context
}

type Queue struct {
	mu      sync.Mutex
	jobs    map[string]Job
	cancels map[string]context.CancelFunc
	pending chan queued
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	runner  *runner.Runner
	catalog application.Catalog
}

func NewQueue(parent context.Context, r *runner.Runner, c application.Catalog, limit int) (*Queue, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("queue limit must be 1..100")
	}
	ctx, cancel := context.WithCancel(parent)
	q := &Queue{jobs: map[string]Job{}, cancels: map[string]context.CancelFunc{}, pending: make(chan queued, limit), ctx: ctx, cancel: cancel, done: make(chan struct{}), runner: r, catalog: c}
	if err := r.Store.Recover(); err != nil {
		cancel()
		return nil, err
	}
	data, err := r.Store.Checkpoints("job")
	if err != nil {
		cancel()
		return nil, err
	}
	for _, raw := range data {
		var j Job
		if err = json.Unmarshal(raw, &j); err != nil {
			cancel()
			return nil, err
		}
		if j.Status == "running" || j.Status == "queued" {
			j.Status = "interrupted"
			j.Error = "Service restarted. No automatic replay; inspect tool trace before submitting again."
			if j.Run != nil {
				if recovered, e := r.Store.Get(j.Run.ID); e == nil {
					j.Run = &recovered
				}
			}
			if err = r.Store.Checkpoint("job", j.ID, j); err != nil {
				cancel()
				return nil, err
			}
		}
		q.jobs[j.ID] = j
	}
	go q.work()
	return q, nil
}

func (q *Queue) Submit(req Request) (Job, error) {
	return q.submitOnce(req, "")
}

func (q *Queue) submitOnce(req Request, key string) (Job, error) {
	b, err := q.catalog.Registry.Get(req.Bot)
	if err != nil {
		return Job{}, err
	}
	if req.Version != "" && req.Version != b.Version {
		return Job{}, errors.New("bot version changed")
	}
	if err = q.catalog.Check(b); err != nil {
		return Job{}, err
	}
	if len(req.Input) == 0 || len(req.Input) > 1<<20 {
		return Job{}, errors.New("input must be 1..1048576 bytes")
	}
	if _, err = config.Resolve(b.Settings, req.Settings); err != nil {
		return Job{}, err
	}
	if req.Budgets != nil {
		v := *req.Budgets
		if v.Tokens < 1 || v.ToolCalls < 0 || v.Seconds < 1 {
			return Job{}, errors.New("positive token/time limits and nonnegative tool limit required")
		}
		b.Budgets = config.Budgets{Tokens: min(b.Budgets.Tokens, v.Tokens), ToolCalls: min(b.Budgets.ToolCalls, v.ToolCalls), Seconds: min(b.Budgets.Seconds, v.Seconds)}
	}
	if req.ReadOnly {
		b.Tools = nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if key != "" {
		for _, j := range q.jobs {
			if j.DispatchKey == key {
				return j, nil
			}
		}
	}
	if q.ctx.Err() != nil {
		return Job{}, errors.New("service stopped")
	}
	if len(q.pending) == cap(q.pending) {
		return Job{}, ErrQueueFull
	}
	j := Job{ID: state.ID(), DispatchKey: key, Bot: b.ID, Status: "queued", Created: time.Now().UTC()}
	if err = q.runner.Store.Checkpoint("job", j.ID, j); err != nil {
		return Job{}, err
	}
	ctx, cancel := context.WithCancel(q.ctx)
	q.jobs[j.ID], q.cancels[j.ID] = j, cancel
	q.pending <- queued{j.ID, b, req, ctx}
	return j, nil
}

func (q *Queue) work() {
	defer close(q.done)
	for {
		select {
		case <-q.ctx.Done():
			return
		case task := <-q.pending:
			q.mu.Lock()
			j := q.jobs[task.id]
			if task.ctx.Err() != nil {
				q.mu.Unlock()
				continue
			}
			j.Status = "running"
			if err := q.runner.Store.Checkpoint("job", j.ID, j); err != nil {
				j.Status, j.Error = "failed", state.Redact(err.Error())
				q.jobs[j.ID] = j
				q.cancels[j.ID]()
				delete(q.cancels, j.ID)
				q.mu.Unlock()
				continue
			}
			q.jobs[j.ID] = j
			q.mu.Unlock()
			run, err := q.runner.Run(task.ctx, task.bot, task.request.Input, runner.Options{AdaptiveContext: task.request.AdaptiveContext, Settings: task.request.Settings, Started: func(r state.Run) error {
				q.mu.Lock()
				defer q.mu.Unlock()
				current := q.jobs[task.id]
				current.Run = &r
				if err := q.runner.Store.Checkpoint("job", current.ID, current); err != nil {
					return err
				}
				q.jobs[current.ID] = current
				return nil
			}})
			sample := diagnostics.Measure()
			q.mu.Lock()
			j.Resources = &sample
			j.Status = run.Status
			if run.ID != "" {
				stored, e := q.runner.Store.Get(run.ID)
				if e == nil {
					j.Run = &stored
				} else {
					err = errors.Join(err, e)
				}
			}
			if err != nil {
				j.Error = state.Redact(err.Error())
				if j.Status == "" {
					j.Status = "failed"
				}
			}
			if e := q.runner.Store.Checkpoint("job", j.ID, j); e != nil {
				j.Status = "failed"
				j.Error = state.Redact(e.Error())
			}
			q.jobs[j.ID] = j
			q.cancels[j.ID]()
			delete(q.cancels, j.ID)
			q.mu.Unlock()
		}
	}
}

func (q *Queue) List() []Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Job, 0, len(q.jobs))
	for _, j := range q.jobs {
		out = append(out, j)
	}
	return out
}

func (q *Queue) Get(id string) (Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	j, ok := q.jobs[id]
	return j, ok
}

func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	j, ok := q.jobs[id]
	if !ok {
		return errors.New("job not found")
	}
	if cancel := q.cancels[id]; cancel != nil {
		cancel()
		if j.Status == "queued" {
			j.Status = "cancelled"
			if err := q.runner.Store.Checkpoint("job", id, j); err != nil {
				return err
			}
			q.jobs[id] = j
			delete(q.cancels, id)
		}
	}
	return nil
}

func (q *Queue) Close() {
	q.cancel()
	q.mu.Lock()
	for id, cancel := range q.cancels {
		cancel()
		j := q.jobs[id]
		if j.Status == "queued" {
			j.Status = "cancelled"
			q.jobs[id] = j
			_ = q.runner.Store.Checkpoint("job", id, j)
		}
	}
	q.mu.Unlock()
	<-q.done
}
