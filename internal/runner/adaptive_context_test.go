package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bots/internal/config"
	"bots/internal/provider"
)

type allocatingProvider struct {
	*fake
	contexts []int
}

func (p *allocatingProvider) Count(_ context.Context, r provider.Request) (int, error) {
	n := 0
	for _, m := range r.Messages {
		n += len(m.Content)
		for _, call := range m.ToolCalls {
			n += len(call.Function.Arguments)
		}
	}
	return n, nil
}

func (p *allocatingProvider) Load(ctx context.Context, m provider.Model) error {
	p.contexts = append(p.contexts, m.ContextSize)
	return p.fake.Load(ctx, m)
}

func TestAdaptiveContextHonorsEngineAndRunLimits(t *testing.T) {
	for _, tc := range []struct {
		name                string
		limit, budget, want int
		adaptive, fail      bool
	}{
		{"grows when needed", 8192, 10000, 8192, true, false},
		{"bounded by reserved tokens", 8192, 6000, 6000, true, false},
		{"engine cap", 4096, 10000, 4096, true, true},
		{"run cap", 8192, 4500, 4500, true, true},
		{"explicit CLI settings stay fixed", 8192, 10000, 4096, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fake{}
			r, b := setup(t, f)
			p := &allocatingProvider{fake: f}
			r.Models.Provider = p
			r.ContextLimit = tc.limit
			b.Settings = config.Settings{ContextSize: 4096, MaxTokens: 2048}
			b.Budgets.Tokens = tc.budget
			run, err := r.Run(context.Background(), b, strings.Repeat("x", 2500), Options{AdaptiveContext: tc.adaptive})
			if tc.fail {
				if !errors.Is(err, ErrContextBudget) || run.Reason != "context_budget" || len(f.requests) != 0 {
					t.Fatal(run, err)
				}
			} else if err != nil || run.Status != "completed" || f.requests[0].Settings.ContextSize != tc.want {
				t.Fatal(run, err)
			}
			if p.contexts[len(p.contexts)-1] != tc.want {
				t.Fatal("wrong context allocation", p.contexts)
			}
		})
	}
}

func TestContextGrowthAfterToolDoesNotReplayWrite(t *testing.T) {
	args := `{"root":0,"path":"draft.txt","content":"` + strings.Repeat("a", 3000) + `"}`
	f := &fake{responses: []provider.Response{tool("file-write", args), final("Delivered")}}
	r, b := setup(t, f)
	p := &allocatingProvider{fake: f}
	r.Models.Provider = p
	r.ContextLimit = 8192
	b.Settings = config.Settings{ContextSize: 4096, MaxTokens: 512}
	b.Budgets.Tokens = 12000
	run, err := r.Run(context.Background(), b, strings.Repeat("x", 1000), Options{AdaptiveContext: true})
	if err != nil || run.Status != "completed" || len(f.requests) != 2 || f.requests[1].Settings.ContextSize != 8192 {
		t.Fatal(run, err)
	}
	count, err := r.Store.ToolEvents(run.ID)
	if err != nil || count != 1 {
		t.Fatal("tool replayed while growing context", count, err)
	}
	events, err := r.Store.Trace(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if strings.Contains(string(event), `"kind":"context_allocated"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("allocation missing from trace")
	}
}
