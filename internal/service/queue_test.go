package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"bots/internal/application"
	"bots/internal/config"
	"bots/internal/provider"
	"bots/internal/registry"
	"bots/internal/runner"
	"bots/internal/state"
)

type engine struct {
	started       chan struct{}
	gate          chan struct{}
	active        atomic.Int32
	peak          atomic.Int32
	failRemaining atomic.Int32
}

func (e *engine) Load(context.Context, provider.Model) error           { return nil }
func (e *engine) Count(context.Context, provider.Request) (int, error) { return 10, nil }
func (e *engine) Generate(ctx context.Context, r provider.Request, emit func(string)) (provider.Response, error) {
	n := e.active.Add(1)
	defer e.active.Add(-1)
	if n > e.peak.Load() {
		e.peak.Store(n)
	}
	select {
	case e.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return provider.Response{}, ctx.Err()
	case <-e.gate:
	}
	if e.failRemaining.Load() > 0 && e.failRemaining.Add(-1) >= 0 {
		return provider.Response{}, errors.New("out of memory fixture")
	}
	return provider.Response{Message: provider.Message{Role: "assistant", Content: r.Messages[len(r.Messages)-1].Content}, Finish: "stop", Usage: provider.Usage{Prompt: 10, Completion: 1, Total: 11}}, nil
}
func (e *engine) Cancel()                      {}
func (e *engine) Health(context.Context) error { return nil }
func (e *engine) Unload(context.Context) error { return nil }

func testQueue(t *testing.T, limit int) (*Queue, *engine, *state.Store) {
	t.Helper()
	root := t.TempDir()
	b := config.Bot{Schema: 1, ID: "echo", Version: "1.0.0", Description: "Echo", Instructions: "echo", BaseModel: "base", Budgets: config.Budgets{Tokens: 1000, Seconds: 10}}
	data, _ := json.Marshal(b)
	if err := os.WriteFile(filepath.Join(root, "bot.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	c := application.Catalog{Registry: registry.Registry{Dir: filepath.Join(root, "registry")}, Model: "base"}
	if _, err := c.Install(root); err != nil {
		t.Fatal(err)
	}
	s, err := state.Open(filepath.Join(root, "db"))
	if err != nil {
		t.Fatal(err)
	}
	e := &engine{started: make(chan struct{}, 10), gate: make(chan struct{})}
	m := provider.NewManager(e)
	q, err := NewQueue(context.Background(), &runner.Runner{Models: m, Store: s}, c, limit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { q.Close(); m.Close(context.Background()); s.Close() })
	return q, e, s
}
func terminal(t *testing.T, q *Queue, id string) Job {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		j, _ := q.Get(id)
		if j.Status != "queued" && j.Status != "running" {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}
func TestBoundedQueueCancellationAndIsolation(t *testing.T) {
	q, e, _ := testQueue(t, 1)
	first, err := q.Submit(Request{Bot: "echo", Input: "first"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-e.started:
	case <-time.After(5 * time.Second):
		t.Fatal("not started")
	}
	second, err := q.Submit(Request{Bot: "echo", Input: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.Submit(Request{Bot: "echo", Input: "overflow"}); err == nil {
		t.Fatal("unbounded queue")
	}
	if err = q.Cancel(second.ID); err != nil {
		t.Fatal(err)
	}
	if terminal(t, q, second.ID).Status != "cancelled" {
		t.Fatal("queued cancel failed")
	}
	if err = q.Cancel(first.ID); err != nil {
		t.Fatal(err)
	}
	if terminal(t, q, first.ID).Status != "cancelled" {
		t.Fatal("active cancel failed")
	}
	close(e.gate)
	var third Job
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		third, err = q.Submit(Request{Bot: "echo", Input: "third"})
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	j := terminal(t, q, third.ID)
	if j.Run == nil || j.Run.Output != "third" || e.peak.Load() != 1 {
		t.Fatal(j, e.peak.Load())
	}
}
func TestRestartNeverReplaysAndRedacts(t *testing.T) {
	q, e, s := testQueue(t, 2)
	close(e.gate)
	j, err := q.Submit(Request{Bot: "echo", Input: "api_key=fixture-secret-value"})
	if err != nil {
		t.Fatal(err)
	}
	j = terminal(t, q, j.ID)
	data, _ := json.Marshal(j)
	if strings.Contains(string(data), "fixture-secret-value") {
		t.Fatal("secret leaked")
	}
	q.Close()
	if err = s.Checkpoint("job", "unfinished", Job{ID: "unfinished", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	q2, err := NewQueue(context.Background(), q.runner, q.catalog, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer q2.Close()
	recovered, _ := q2.Get("unfinished")
	if recovered.Status != "interrupted" {
		t.Fatal(recovered)
	}
	if e.active.Load() != 0 {
		t.Fatal("replayed action")
	}
}

func TestGenerationFailureDoesNotPoisonNextJob(t *testing.T) {
	q, e, _ := testQueue(t, 4)
	close(e.gate)
	e.failRemaining.Store(2)
	j, err := q.Submit(Request{Bot: "echo", Input: "fails"})
	if err != nil {
		t.Fatal(err)
	}
	if terminal(t, q, j.ID).Status != "failed" {
		t.Fatal("failure hidden")
	}
	j, err = q.Submit(Request{Bot: "echo", Input: "recovers"})
	if err != nil {
		t.Fatal(err)
	}
	if result := terminal(t, q, j.ID); result.Status != "completed" || result.Run.Output != "recovers" {
		t.Fatal(result)
	}
}
