package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"bots/internal/config"
	"bots/internal/provider"
)

func truncated(text string, completion int) provider.Response {
	return provider.Response{Message: provider.Message{Role: "assistant", Content: text}, Finish: "length", Usage: provider.Usage{Prompt: 20, Completion: completion, Total: 20 + completion}}
}

func TestManagedOutputGrowsPastBotDefault(t *testing.T) {
	f := &fake{responses: []provider.Response{truncated("Discarded partial", 64), final("Complete answer")}}
	r, b := setup(t, f)
	r.ContextLimit = 512
	b.Settings = config.Settings{ContextSize: 512, MaxTokens: 64}
	b.Budgets.Tokens = 100
	grants := []int{}
	run, err := r.Run(context.Background(), b, "answer", Options{AdaptiveContext: true, AdaptiveOutput: true, AllocateTokens: func(required int) (int, error) { grants = append(grants, required); return required, nil }})
	if err != nil || run.Output != "Complete answer" || run.Usage.Total != 106 || len(f.requests) != 2 || f.requests[1].Settings.MaxTokens <= 64 || len(grants) == 0 {
		t.Fatal(run, err, grants)
	}
}

func TestManagedOutputShrinksForLongPrompt(t *testing.T) {
	f := &fake{}
	r, b := setup(t, f)
	p := &allocatingProvider{fake: f}
	r.Models.Provider = p
	r.ContextLimit = 512
	b.Settings = config.Settings{ContextSize: 512, MaxTokens: 400}
	run, err := r.Run(context.Background(), b, strings.Repeat("x", 350), Options{AdaptiveContext: true, AdaptiveOutput: true})
	if err != nil || run.Status != "completed" || f.requests[0].Settings.MaxTokens >= 400 || f.requests[0].Settings.MaxTokens < 1 {
		t.Fatal(run, err)
	}
}

func TestManagedOutputContinuesAtContextCeiling(t *testing.T) {
	f := &fake{responses: []provider.Response{truncated("First section. ", 492), final("Last section.")}}
	r, b := setup(t, f)
	r.ContextLimit = 512
	b.Tools = nil
	b.Settings = config.Settings{ContextSize: 512, MaxTokens: 492}
	b.Budgets.Tokens = 2000
	run, err := r.Run(context.Background(), b, "answer", Options{AdaptiveContext: true, AdaptiveOutput: true})
	if err != nil || run.Output != "First section. Last section." || len(f.requests) != 2 {
		t.Fatal(run, err)
	}
	last := f.requests[1].Messages[len(f.requests[1].Messages)-1].Content
	if !strings.Contains(last, "Continue the same answer") {
		t.Fatal("missing continuation instruction")
	}
	_, history, err := r.Store.Session(run.Session, b.ID, b.Version)
	if err != nil || history[len(history)-1].Content != run.Output {
		t.Fatal("complete answer not retained", history, err)
	}
}

func TestManagedRetriesDoNotRepeatFileWrites(t *testing.T) {
	f := &fake{responses: []provider.Response{tool("file-write", `{"root":0,"path":"result.txt","content":"Saved content"}`), truncated("Partial summary", 64), final("Complete summary")}}
	r, b := setup(t, f)
	r.ContextLimit = 512
	b.Settings = config.Settings{ContextSize: 512, MaxTokens: 64}
	run, err := r.Run(context.Background(), b, "write", Options{AdaptiveContext: true, AdaptiveOutput: true})
	count, traceErr := r.Store.ToolEvents(run.ID)
	if err != nil || traceErr != nil || count != 1 || run.Output != "Complete summary" {
		t.Fatal(run, err, count, traceErr)
	}
}

func TestIncompleteToolArgumentsAreNeverExecuted(t *testing.T) {
	response := tool("file-write", `{"root":0,"path":"unsafe.txt","content":"partial"}`)
	response.Finish = "length"
	f := &fake{responses: []provider.Response{response}}
	r, b := setup(t, f)
	r.ContextLimit = 512
	b.Settings = config.Settings{ContextSize: 512, MaxTokens: 428}
	run, err := r.Run(context.Background(), b, "write", Options{AdaptiveContext: true, AdaptiveOutput: true})
	if err == nil || run.Status != "failed" {
		t.Fatal("truncated tool call accepted", run)
	}
	if _, err := os.Stat(filepath.Join(b.ContextSources[0], "unsafe.txt")); !os.IsNotExist(err) {
		t.Fatal("incomplete tool call executed", err)
	}
}

func TestManagedOutputStopsAtSharedBudget(t *testing.T) {
	f := &fake{responses: []provider.Response{truncated("Partial", 64)}}
	r, b := setup(t, f)
	r.ContextLimit = 512
	b.Settings = config.Settings{ContextSize: 512, MaxTokens: 64}
	b.Budgets.Tokens = 100
	run, err := r.Run(context.Background(), b, "answer", Options{AdaptiveContext: true, AdaptiveOutput: true, AllocateTokens: func(int) (int, error) { return 100, nil }})
	if err == nil || run.Reason != "token_budget" || run.Output != "Partial" || len(f.requests) != 1 {
		t.Fatal(run, err)
	}
}

func TestStreamingOutputNeverRegeneratesEmittedText(t *testing.T) {
	f := &fake{responses: []provider.Response{truncated("Partial", 64)}}
	r, b := setup(t, f)
	run, err := r.Run(context.Background(), b, "answer", Options{AdaptiveOutput: true, Emit: func(string) {}})
	if err == nil || run.Reason != "generation_limit" || len(f.requests) != 1 {
		t.Fatal(run, err)
	}
}

func TestContinuationTailPreservesUTF8(t *testing.T) {
	messages := continuationMessages(strings.Repeat("海", 1000))
	if len(messages[0].Content) > 1024 || !utf8.ValidString(messages[0].Content) {
		t.Fatal("broken continuation tail")
	}
}

func TestDeniedContextGrowthCanStillReduceReplyAllowance(t *testing.T) {
	f := &fake{}
	r, b := setup(t, f)
	r.Models.Provider = &allocatingProvider{fake: f}
	r.ContextLimit = 1024
	b.Settings = config.Settings{ContextSize: 512, MaxTokens: 400}
	b.Budgets.Tokens = 500
	run, err := r.Run(context.Background(), b, strings.Repeat("x", 350), Options{AdaptiveContext: true, AdaptiveOutput: true, AllocateTokens: func(int) (int, error) { return 500, nil }})
	if err != nil || run.Status != "completed" || f.requests[0].Settings.MaxTokens >= 400 {
		t.Fatal(run, err)
	}
}
