package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bots/internal/provider"
	"bots/internal/tools"
)

func writeEvidence(t *testing.T, a *Autonomy, run, kind, path, content string, failed bool) {
	t.Helper()
	args, _ := json.Marshal(tools.FileArgs{Root: 0, Path: path, Content: content})
	result := tools.Result{Output: "created " + path}
	if failed {
		result.Error = &tools.Failure{Code: "tool_failure", Message: "failed"}
	}
	err := a.q.runner.Store.Event(run, map[string]any{
		"kind": kind, "call": provider.Call{Function: provider.Function{Name: "file-write", Arguments: string(args)}}, "result": result,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeliverableUsesOnlySuccessfulWrites(t *testing.T) {
	for _, tc := range []struct {
		name, kind, content string
		failed, wantError   bool
	}{
		{"saved poem", "tool_finished", "Waves rise.\n\nWinds sing.\n\nTides fall.", false, false},
		{"failed write", "tool_finished", "Not saved", true, true},
		{"unfinished write", "tool_started", "Not confirmed", false, true},
		{"empty file", "tool_finished", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testAutonomy(t)
			writeEvidence(t, a, "run", tc.kind, "poem.md", tc.content, tc.failed)
			got, err := a.deliverable("run", "created poem.md")
			if (err != nil) != tc.wantError {
				t.Fatalf("got %q, %v", got, err)
			}
			if !tc.wantError && got != tc.content {
				t.Fatalf("receipt displayed instead of poem: %q", got)
			}
		})
	}
}

func TestDeliverableLatestWriteAndMultipleFiles(t *testing.T) {
	a, _ := testAutonomy(t)
	writeEvidence(t, a, "run", "tool_finished", "poem.md", "Draft", false)
	writeEvidence(t, a, "run", "tool_finished", "poem.md", "Final poem", false)
	writeEvidence(t, a, "run", "tool_finished", "poem.md", "Failed overwrite", true)
	writeEvidence(t, a, "run", "tool_finished", "notes.txt", "Notes", false)
	got, err := a.deliverable("run", "created notes.txt")
	if err != nil || got != "## poem.md\n\nFinal poem\n\n## notes.txt\n\nNotes" {
		t.Fatal(got, err)
	}
}

func TestReceiptCannotFinishDirectOrManagedTask(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, output := range []string{"created poem.md", "poem.md"} {
			t.Run(output+map[bool]string{true: " direct", false: " managed"}[direct], func(t *testing.T) {
				a, e := testAutonomy(t)
				req := goalRequest()
				req.Direct = direct
				if direct {
					e.plain = `{"output":"` + output + `"}`
				} else {
					e.plain = `{"status":"result","output":"` + output + `","message":"Saved"}`
				}
				if _, err := a.Create(req); err != nil {
					t.Fatal(err)
				}
				b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Blocked })
				if !strings.Contains(b.Cards[len(b.Cards)-1].History[len(b.Cards[len(b.Cards)-1].History)-1].Text, "file receipt") {
					t.Fatal("missing explicit rejection", b)
				}
			})
		}
	}
}

type writingBoardEngine struct{ *boardEngine }

func (e *writingBoardEngine) Generate(ctx context.Context, r provider.Request, emit func(string)) (provider.Response, error) {
	prompt := ""
	for _, m := range r.Messages {
		if m.Role == "user" {
			prompt = m.Content
		}
	}
	if !strings.Contains(prompt, "Break the goal") && !strings.Contains(prompt, "Verify the card") && r.Messages[len(r.Messages)-1].Role != "tool" {
		args, _ := json.Marshal(tools.FileArgs{Root: 0, Path: "poem.md", Content: "Waves rise.\n\nWinds sing.\n\nTides fall."})
		return provider.Response{Message: provider.Message{Role: "assistant", ToolCalls: []provider.Call{{ID: "write-poem", Type: "function", Function: provider.Function{Name: "file-write", Arguments: string(args)}}}}, Finish: "tool_calls", Usage: provider.Usage{Prompt: 10, Completion: 1, Total: 11}}, nil
	}
	return e.boardEngine.Generate(ctx, r, emit)
}

func TestWrittenPoemReachesCardAndSurvivesReceiptRecovery(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(map[bool]string{true: "direct", false: "managed"}[direct], func(t *testing.T) {
			a, e := testAutonomy(t)
			bot, err := a.q.catalog.Registry.Get("echo")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			bot.Tools = []string{"file-write"}
			bot.ContextSources = []string{dir}
			bot.Budgets.ToolCalls = 1
			raw, _ := json.Marshal(bot)
			if err := os.WriteFile(filepath.Join(a.q.catalog.Registry.Dir, "echo.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			req := goalRequest()
			req.Direct, req.ToolCalls = direct, 1
			e.plain = `{"status":"result","output":"created poem.md","message":"Saved"}`
			if direct {
				e.plain = "created poem.md"
			}
			a.q.runner.Models.Provider = &writingBoardEngine{e}
			if _, err := a.Create(req); err != nil {
				t.Fatal(err)
			}
			b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
			i := len(b.Cards) - 1
			poem := "Waves rise.\n\nWinds sing.\n\nTides fall."
			if b.Cards[i].Output != poem {
				t.Fatalf("missing poem: %+v", b.Cards[i])
			}
			if !direct {
				e.mu.Lock()
				verification := e.requests[len(e.requests)-1].Messages
				prompt := verification[len(verification)-1].Content
				e.mu.Unlock()
				if !strings.Contains(prompt, "Waves rise.") {
					t.Fatal("manager never saw the deliverable")
				}
			}
			runs := b.Goals[0].Runs
			b.Cards[i].Output = "created poem.md"
			if err := a.save(b); err != nil {
				t.Fatal(err)
			}
			restored, err := NewAutonomy(a.q)
			if err != nil {
				t.Fatal(err)
			}
			restored.Close()
			repaired, _ := restored.Snapshot()
			if repaired.Cards[i].Output != poem || repaired.Goals[0].Runs != runs {
				t.Fatal("receipt repair lost content or replayed work", repaired)
			}
		})
	}
}

func TestNumericAndOrdinaryAnswersAreNotFileReceipts(t *testing.T) {
	for _, text := range []string{"1.23", "The sea is blue.", "print('hello')", "Waves rise.\nWaves fall."} {
		if isFileReceipt(text) {
			t.Fatal("valid answer mistaken for a filename:", text)
		}
	}
}

func TestLegacyReceiptWithoutWorkerEvidenceIsBlocked(t *testing.T) {
	a, _ := testAutonomy(t)
	req := goalRequest()
	req.Direct = true
	if _, err := a.Create(req); err != nil {
		t.Fatal(err)
	}
	b, _ := a.Snapshot()
	b.Cards[0].Status, b.Cards[0].Output = Done, "created poem.md"
	if err := a.save(b); err != nil {
		t.Fatal(err)
	}
	restored, err := NewAutonomy(a.q)
	if err != nil {
		t.Fatal(err)
	}
	restored.Close()
	b, _ = restored.Snapshot()
	if b.Cards[0].Status != Blocked || b.Goals[0].Runs != 0 {
		t.Fatal("unsupported completion survived recovery", b)
	}
}
