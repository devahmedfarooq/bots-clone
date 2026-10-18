package service

import (
	"bots/internal/config"
	"bots/internal/provider"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerReservationsGrowWithinGoalAndPersist(t *testing.T) {
	a, _ := testAutonomy(t)
	req := goalRequest()
	req.Direct = true
	if _, err := a.Create(req); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := a.tick(); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := a.Snapshot()
	key := b.Cards[0].Key
	granted, err := a.allocateTokens(key, 250)
	if err != nil || granted != 250 {
		t.Fatal(granted, err)
	}
	granted, err = a.allocateTokens(key, 2000)
	b, _ = a.Snapshot()
	if err != nil || granted != req.TokenLimit || b.Goals[0].Tokens != granted || b.Cards[0].Reserved != granted {
		t.Fatal(b, err)
	}
	saved, err := a.q.runner.Store.Checkpoints("autonomy")
	if err != nil {
		t.Fatal(err)
	}
	var persisted Board
	if err = json.Unmarshal(saved[0], &persisted); err != nil || persisted.Cards[0].Reserved != granted {
		t.Fatal("reservation not durable", err)
	}
	if _, err = a.allocateTokens("unknown", 1); err == nil {
		t.Fatal("inactive dispatch allocated tokens")
	}
}

func TestManagedJobCanUseMoreThanPackageTokenBudget(t *testing.T) {
	a, e := testAutonomy(t)
	bot, err := a.q.catalog.Registry.Get("echo")
	if err != nil {
		t.Fatal(err)
	}
	bot.Budgets.Tokens = 10
	raw, _ := json.Marshal(bot)
	if err = os.WriteFile(filepath.Join(a.q.catalog.Registry.Dir, "echo.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	req := goalRequest()
	req.Direct = true
	e.plain = "Finished answer"
	if _, err = a.Create(req); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	if b.Goals[0].Tokens != 11 || b.Cards[0].Output != "Finished answer" {
		t.Fatal(b)
	}
	job, err := a.q.Submit(Request{Bot: "echo", Input: "fixed CLI-style budget"})
	if err != nil {
		t.Fatal(err)
	}
	if got := terminal(t, a.q, job.ID); got.Status == "completed" {
		t.Fatal("unmanaged package limit bypassed")
	}
}

func TestManagerCanAssignStartingOutputTokens(t *testing.T) {
	var plan struct {
		Tasks []planTask `json:"tasks"`
	}
	if err := decodeBotOutput(`{"tasks":[{"title":"Math","requirements":"Calculate","acceptance":"Correct","owner":"echo","output_tokens":64}]}`, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Tasks[0].OutputTokens != 64 {
		t.Fatal(plan)
	}
	for _, n := range []int{-1, 131072} {
		if planProblem([]TaskPlan{{Title: "Task", Requirements: "Work", Acceptance: "Done", Owner: "echo", OutputTokens: n}}, []string{"echo"}) == "" {
			t.Fatal("invalid output allocation accepted")
		}
	}
}

type growingBoardEngine struct{ *boardEngine }

func (e *growingBoardEngine) Generate(ctx context.Context, request provider.Request, emit func(string)) (provider.Response, error) {
	if request.Settings.MaxTokens < 64 {
		return provider.Response{Message: provider.Message{Role: "assistant", Content: "Partial"}, Finish: "length", Usage: provider.Usage{Prompt: 10, Completion: request.Settings.MaxTokens, Total: 10 + request.Settings.MaxTokens}}, nil
	}
	return e.boardEngine.Generate(ctx, request, emit)
}

func TestRunningManagerExtendsReservationAndRefundsUnusedTokens(t *testing.T) {
	a, e := testAutonomy(t)
	bot, err := a.q.catalog.Registry.Get("echo")
	if err != nil {
		t.Fatal(err)
	}
	bot.Budgets.Tokens = 20
	bot.Settings = config.Settings{ContextSize: 512, MaxTokens: 32}
	raw, _ := json.Marshal(bot)
	if err = os.WriteFile(filepath.Join(a.q.catalog.Registry.Dir, "echo.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	a.q.runner.ContextLimit = 512
	a.q.runner.Models.Provider = &growingBoardEngine{e}
	e.plain = "Finished answer"
	req := goalRequest()
	req.Direct, req.RunTokens = true, 30
	if _, err = a.Create(req); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	if b.Goals[0].Tokens != 53 || b.Goals[0].Runs != 1 || b.Cards[0].Reserved <= 30 || b.Cards[0].Output != "Finished answer" {
		t.Fatal("allocation/refund mismatch", b)
	}
}
