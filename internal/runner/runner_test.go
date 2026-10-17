package runner

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"bots/internal/config"
	"bots/internal/provider"
	"bots/internal/state"
)

type fake struct {
	mu             sync.Mutex
	loaded         bool
	loads, unloads int
	responses      []provider.Response
	requests       []provider.Request
	failures       int
	block          bool
}

func (f *fake) Load(context.Context, provider.Model) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.loaded {
		f.loads++
		f.loaded = true
	}
	return nil
}
func (f *fake) Count(context.Context, provider.Request) (int, error) { return 20, nil }
func (f *fake) Generate(ctx context.Context, r provider.Request, emit func(string)) (provider.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	block := f.block
	if f.failures > 0 {
		f.failures--
		f.mu.Unlock()
		return provider.Response{}, errors.New("model failed")
	}
	if block {
		f.mu.Unlock()
		<-ctx.Done()
		return provider.Response{}, ctx.Err()
	}
	var response provider.Response
	if len(f.responses) > 0 {
		response = f.responses[0]
		f.responses = f.responses[1:]
	} else {
		response = final("done")
	}
	f.mu.Unlock()
	return response, nil
}
func (f *fake) Cancel()                      {}
func (f *fake) Health(context.Context) error { return nil }
func (f *fake) Unload(context.Context) error { f.unloads++; f.loaded = false; return nil }
func final(text string) provider.Response {
	return provider.Response{Message: provider.Message{Role: "assistant", Content: text}, Usage: provider.Usage{Prompt: 20, Completion: 2, Total: 22}, Finish: "stop"}
}
func tool(name, args string) provider.Response {
	return provider.Response{Message: provider.Message{Role: "assistant", ToolCalls: []provider.Call{{ID: "call", Type: "function", Function: provider.Function{Name: name, Arguments: args}}}}, Usage: provider.Usage{Prompt: 20, Completion: 2, Total: 22}, Finish: "tool_calls"}
}
func bot(root string) config.Bot {
	return config.Bot{Schema: 1, ID: "one", Version: "1.0.0", Description: "test", Instructions: "help", BaseModel: "base", Tools: []string{"file-write", "file-read"}, ContextSources: []string{root}, Budgets: config.Budgets{Tokens: 1000, ToolCalls: 3, Seconds: 5}}
}
func setup(t *testing.T, f *fake) (*Runner, config.Bot) {
	t.Helper()
	root := t.TempDir()
	store, e := state.Open(filepath.Join(root, "db.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	m := provider.NewManager(f)
	t.Cleanup(func() { m.Close(context.Background()); store.Close() })
	return &Runner{m, store}, bot(root)
}
func TestToolLoopAndIsolation(t *testing.T) {
	f := &fake{responses: []provider.Response{tool("file-write", `{"root":0,"path":"answer.txt","content":"42"}`), tool("file-read", `{"root":0,"path":"answer.txt"}`), final("42")}}
	r, b := setup(t, f)
	first, e := r.Run(context.Background(), b, "private-one", Options{})
	if e != nil || first.Status != "completed" || first.Usage.Total != 66 {
		t.Fatal(first, e)
	}
	count, e := r.Store.ToolEvents(first.ID)
	if e != nil || count != 2 {
		t.Fatal(count, e)
	}
	b.ID = "two"
	second, e := r.Run(context.Background(), b, "private-two", Options{})
	if e != nil || second.Session == first.Session {
		t.Fatal(second, e)
	}
	for _, m := range f.requests[len(f.requests)-1].Messages {
		if strings.Contains(m.Content, "private-one") {
			t.Fatal("history leaked")
		}
	}
	if _, e = r.Run(context.Background(), b, "steal", Options{Session: first.Session}); e == nil {
		t.Fatal("cross-bot resume allowed")
	}
	if f.loads != 1 {
		t.Fatal("weights loaded repeatedly", f.loads)
	}
}
func TestBudgetsAndRecovery(t *testing.T) {
	for _, kind := range []string{"tools", "tokens", "invalid", "failure", "length", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			f := &fake{}
			r, b := setup(t, f)
			ctx := context.Background()
			switch kind {
			case "tools":
				b.Budgets.ToolCalls = 0
				f.responses = []provider.Response{tool("file-read", `{"root":0,"path":"x"}`)}
			case "tokens":
				b.Budgets.Tokens = 20
			case "invalid":
				f.responses = []provider.Response{{}}
			case "failure":
				f.failures = 2
			case "length":
				v := final("partial")
				v.Finish = "length"
				f.responses = []provider.Response{v}
			case "cancel":
				f.block = true
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 20*time.Millisecond)
				defer cancel()
			}
			result, e := r.Run(ctx, b, "test", Options{})
			if e == nil || result.Status == "completed" {
				t.Fatal(result, e)
			}
			saved, e := r.Store.Get(result.ID)
			if e != nil || saved.Status != result.Status {
				t.Fatal(saved, e)
			}
			f.block = false
			b.Budgets.Tokens = 1000
			b.Budgets.ToolCalls = 3
			result, e = r.Run(context.Background(), b, "later", Options{})
			if e != nil || result.Status != "completed" {
				t.Fatal("later run failed", result, e)
			}
		})
	}
}
func TestQueueAndCancellation(t *testing.T) {
	f := &fake{}
	m := provider.NewManager(f)
	release, e := m.Acquire(context.Background(), provider.Model{ID: "base", ContextSize: 4096})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, e = m.Acquire(ctx, provider.Model{ID: "base", ContextSize: 4096}); e == nil {
		t.Fatal("cancelled queue acquired slot")
	}
	release()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, e := m.Acquire(context.Background(), provider.Model{ID: "base", ContextSize: 4096})
			if e != nil {
				t.Error(e)
				return
			}
			defer release()
		}()
	}
	wg.Wait()
	if f.loads != 1 {
		t.Fatal(f.loads)
	}
	if e = m.Close(context.Background()); e != nil || f.unloads != 1 {
		t.Fatal(e, f.unloads)
	}
}
func TestRunDeadline(t *testing.T) {
	f := &fake{block: true}
	r, b := setup(t, f)
	b.Budgets.Seconds = 1
	run, e := r.Run(context.Background(), b, "wait", Options{})
	if e == nil || run.Reason != "time_budget" {
		t.Fatal(run, e)
	}
}
