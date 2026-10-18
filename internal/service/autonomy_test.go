package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"bots/internal/provider"
	"bots/internal/state"
)

type boardEngine struct {
	*engine
	mu           sync.Mutex
	invalidPlan  bool
	fixOnRetry   bool
	bigReview    bool
	alwaysBig    bool
	echoFirst    bool
	arrayPlan    bool
	workerStatus string
	reject       bool
	plain        string
	requests     []provider.Request
}

func (e *boardEngine) Generate(_ context.Context, r provider.Request, _ func(string)) (provider.Response, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requests = append(e.requests, r)
	prompt := r.Messages[len(r.Messages)-1].Content
	output := `{"status":"result","output":"Verified answer: 42","message":"Task complete with evidence."}`
	if strings.Contains(prompt, "Break the goal") {
		output = `{"tasks":[{"title":"Find answer","requirements":"Calculate answer","acceptance":"Answer equals 42","owner":"echo"}]}`
		if e.invalidPlan && !(e.fixOnRetry && strings.Contains(prompt, "previous plan was rejected")) {
			output = `{"tasks":[{"title":"Unauthorized","requirements":"work","acceptance":"done","owner":"stranger"}]}`
		}
		if e.arrayPlan {
			output = strings.TrimSuffix(strings.TrimPrefix(output, `{"tasks":`), `}`)
		}
	} else if strings.Contains(prompt, "Verify the card") {
		output = `{"approved":true,"reason":"Output contains the required answer 42."}`
		if e.reject {
			output = `{"approved":false,"reason":"Missing evidence."}`
		}
	} else if e.plain != "" {
		output = e.plain
		if e.echoFirst && strings.Contains(prompt, "previous reply was rejected") {
			output = "Verified answer: 42"
		}
	} else if e.workerStatus != "" {
		data, _ := json.Marshal(map[string]string{"status": e.workerStatus, "output": "Partial work", "message": "Need user input"})
		output = string(data)
	}
	return provider.Response{Message: provider.Message{Role: "assistant", Content: output}, Finish: "stop", Usage: provider.Usage{Prompt: 10, Completion: 1, Total: 11}}, nil
}

func testAutonomy(t *testing.T) (*Autonomy, *boardEngine) {
	t.Helper()
	q, e, _ := testQueue(t, 4)
	model := &boardEngine{engine: e}
	q.runner.Models.Provider = model
	a, err := NewAutonomy(q)
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	return a, model
}

func goalRequest() GoalRequest {
	return GoalRequest{Requirements: "Find the answer 42", Manager: "echo", Workers: []string{"echo"}, GoalSettings: GoalSettings{Priority: 3, TokenLimit: 1000, RunLimit: 10, RunTokens: 100, Seconds: 5, ToolCalls: 0}}
}

func driveBoard(t *testing.T, a *Autonomy, until func(Board) bool) Board {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		err := a.tick()
		a.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := a.Snapshot()
		if until(b) {
			return b
		}
		time.Sleep(time.Millisecond)
	}
	b, _ := a.Snapshot()
	t.Fatalf("board did not settle: %+v", b)
	return Board{}
}

func TestAutonomousGoalCompletesAndPersists(t *testing.T) {
	a, e := testAutonomy(t)
	g, err := a.Create(goalRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Chat(g.ID, "Use direct evidence."); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	if len(b.Cards) != 2 || len(b.Cards[1].Jobs) != 2 || b.Cards[1].Output != "Verified answer: 42" || b.Goals[0].Runs != 3 || b.Goals[0].Tokens != 33 {
		t.Fatalf("bad result: %+v", b)
	}
	e.mu.Lock()
	if len(e.requests) != 3 {
		t.Fatal(len(e.requests))
	}
	for _, r := range e.requests {
		if r.Settings.MaxTokens > 90 {
			t.Fatal("run token cap not enforced")
		}
		if !strings.Contains(r.Messages[len(r.Messages)-1].Content, "Use direct evidence.") {
			t.Fatal("instruction missing")
		}
	}
	e.mu.Unlock()
	restored, err := NewAutonomy(a.q)
	if err != nil {
		t.Fatal(err)
	}
	restored.Close()
	saved, _ := restored.Snapshot()
	if saved.Goals[0].Status != Done || len(saved.Cards[1].History) < 4 || len(saved.Goals[0].Chat) < 4 {
		t.Fatal("lost persistent history")
	}
}

func TestManagerTaskArray(t *testing.T) {
	a, e := testAutonomy(t)
	e.arrayPlan = true
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
}

func TestBotOutputFormattingStillRequiresStrictJSON(t *testing.T) {
	for _, raw := range []string{"{\"approved\":true}", "```json\n{\"approved\":true}\n```", "```\n{\"approved\":true}\n```"} {
		var result struct {
			Approved bool `json:"approved"`
		}
		if err := decodeBotOutput(raw, &result); err != nil || !result.Approved {
			t.Fatal(raw, err)
		}
	}
	for _, raw := range []string{"Sure: {\"approved\":true}", "```json\n{\"approved\":true}\n```extra", "{\"approved\":true,\"extra\":1}", "{\"approved\":true} {}"} {
		var result struct {
			Approved bool `json:"approved"`
		}
		if err := decodeBotOutput(raw, &result); err == nil {
			t.Fatal("invalid response accepted", raw)
		}
	}
}

func TestToolApprovalAndManagerHasNoTools(t *testing.T) {
	a, e := testAutonomy(t)
	bot, err := a.q.catalog.Registry.Get("echo")
	if err != nil {
		t.Fatal(err)
	}
	bot.Tools = []string{"file-read"}
	data, _ := json.Marshal(bot)
	if err = os.WriteFile(filepath.Join(a.q.catalog.Registry.Dir, "echo.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	req := goalRequest()
	req.Rules.Tools = true
	if _, err = a.Create(req); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return len(b.Cards) == 2 && b.Cards[1].Status == Approval })
	if b.Goals[0].Runs != 1 {
		t.Fatal("tools bypassed approval")
	}
	if err = a.Action(b.Cards[1].ID, "approve", "Allowed"); err != nil {
		t.Fatal(err)
	}
	driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.requests) != 3 || len(e.requests[0].Tools) != 0 || len(e.requests[1].Tools) != 1 || len(e.requests[2].Tools) != 0 {
		t.Fatal("manager tools not stripped or worker grant lost")
	}
}

func TestCompletedDispatchReconcilesOnce(t *testing.T) {
	a, _ := testAutonomy(t)
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Cards[0].Job != "" })
	terminal(t, a.q, b.Cards[0].Job)
	restored, err := NewAutonomy(a.q)
	if err != nil {
		t.Fatal(err)
	}
	restored.Close()
	b = driveBoard(t, restored, func(b Board) bool { return len(b.Cards) == 2 })
	if len(a.q.List()) != 1 || b.Goals[0].Tokens != 11 || b.Goals[0].Runs != 1 {
		t.Fatal("completed dispatch replayed or double charged")
	}
}

func TestAutonomousApprovalAndRejection(t *testing.T) {
	a, _ := testAutonomy(t)
	req := goalRequest()
	req.Rules = ApprovalRules{BeforeWork: true, BeforeDone: true}
	_, err := a.Create(req)
	if err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return len(b.Cards) == 2 && b.Cards[1].Status == Approval })
	card := b.Cards[1]
	if b.Goals[0].Runs != 1 {
		t.Fatal("worker ran before approval")
	}
	if err = a.Action(card.ID, "approve", "Proceed"); err != nil {
		t.Fatal(err)
	}
	b = driveBoard(t, a, func(b Board) bool { return b.Cards[1].Status == Approval && b.Cards[1].Gate == "done" })
	if err = a.Action(card.ID, "reject", "Need more evidence"); err != nil {
		t.Fatal(err)
	}
	b, _ = a.Snapshot()
	if b.Cards[1].Status != Blocked {
		t.Fatal("rejection ignored")
	}
	if err = a.Action(card.ID, "approve", ""); err == nil {
		t.Fatal("approved blocked card")
	}
	if err = a.Action(card.ID, "retry", "Recheck evidence"); err != nil {
		t.Fatal(err)
	}
	driveBoard(t, a, func(b Board) bool { return b.Cards[1].Status == Approval })
	if err = a.Action(card.ID, "approve", "Evidence accepted"); err != nil {
		t.Fatal(err)
	}
	driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
}

func TestAutonomousBudgetsAndInvalidPlan(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget", true: "owner"}[invalid], func(t *testing.T) {
			a, e := testAutonomy(t)
			e.invalidPlan = invalid
			req := goalRequest()
			req.RunLimit = 1
			g, err := a.Create(req)
			if err != nil {
				t.Fatal(err)
			}
			b := driveBoard(t, a, func(b Board) bool {
				for _, c := range b.Cards {
					if c.Status == Blocked {
						return true
					}
				}
				return false
			})
			if b.Goals[0].Runs != 1 {
				t.Fatal("budget bypassed")
			}
			if invalid && len(b.Cards) != 1 {
				t.Fatal("unauthorized owner admitted")
			}
			if !invalid {
				s := req.GoalSettings
				s.RunLimit = 5
				if err = a.Settings(g.ID, s); err != nil {
					t.Fatal(err)
				}
				if err = a.Action(b.Cards[1].ID, "retry", "Budget increased"); err != nil {
					t.Fatal(err)
				}
				driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
			}
		})
	}
}

func TestAutonomousQuestionsAndFailedReview(t *testing.T) {
	for _, question := range []bool{false, true} {
		t.Run(map[bool]string{false: "review", true: "question"}[question], func(t *testing.T) {
			a, e := testAutonomy(t)
			if question {
				e.workerStatus = "question"
			} else {
				e.reject = true
			}
			if _, err := a.Create(goalRequest()); err != nil {
				t.Fatal(err)
			}
			b := driveBoard(t, a, func(b Board) bool { return len(b.Cards) == 2 && b.Cards[1].Status == Blocked })
			if b.Goals[0].Status == Done || len(b.Cards[1].History) < 3 {
				t.Fatal("failure hidden")
			}
		})
	}
}

func TestAutonomousPriorityAndUncertainRestart(t *testing.T) {
	a, _ := testAutonomy(t)
	req := goalRequest()
	req.Priority = 1
	if _, err := a.Create(req); err != nil {
		t.Fatal(err)
	}
	req.Priority = 5
	g, err := a.Create(req)
	if err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool {
		for _, c := range b.Cards {
			if c.Status == InProgress {
				return true
			}
		}
		return false
	})
	if b.Cards[1].Goal != g.ID || b.Cards[1].Status != InProgress {
		t.Fatal("priority ignored")
	}
	c := b.Cards[1]
	if err = a.q.runner.Store.Checkpoint("job", "uncertain", Job{ID: "uncertain", DispatchKey: c.Key, Status: "interrupted"}); err != nil {
		t.Fatal(err)
	}
	// Load the durable queue and board as a restarted process would.
	a.q.Close()
	q, err := NewQueue(context.Background(), a.q.runner, a.q.catalog, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	restored, err := NewAutonomy(q)
	if err != nil {
		t.Fatal(err)
	}
	restored.Close()
	b = driveBoard(t, restored, func(b Board) bool { return b.Cards[1].Status == Blocked })
	if len(q.List()) != 1 || b.Goals[1].Tokens != c.Reserved {
		t.Fatal("uncertain work replayed or reservation lost")
	}
}

func TestDispatchKeyDedupAcrossRestart(t *testing.T) {
	q, e, _ := testQueue(t, 4)
	close(e.gate)
	req := Request{Bot: "echo", Input: "once"}
	j, err := q.submitOnce(req, "stable-key")
	if err != nil {
		t.Fatal(err)
	}
	terminal(t, q, j.ID)
	q.Close()
	q2, err := NewQueue(context.Background(), q.runner, q.catalog, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer q2.Close()
	again, err := q2.submitOnce(req, "stable-key")
	if err != nil || again.ID != j.ID || len(q2.List()) != 1 {
		t.Fatal("duplicate dispatch", again, err)
	}
}

func TestBoardAPIValidation(t *testing.T) {
	a, _ := testAutonomy(t)
	s := &Server{Autonomy: a}
	mux := http.NewServeMux()
	s.autonomyRoutes(mux)
	handler := Secure(mux, "127.0.0.1:8080")
	for _, test := range []struct {
		path, method, auth, payload string
		status                      int
	}{
		{"/api/board", "GET", "Bearer test", "", 200},
		{"/board", "GET", "", "", 200},
		{"/api/goals", "POST", "Bearer test", `{"requirements":"missing settings"}`, 400},
		{"/api/cards/missing/approve", "POST", "Bearer test", `{}`, 400},
	} {
		r := httptest.NewRequest(test.method, "http://127.0.0.1:8080"+test.path, strings.NewReader(test.payload))
		r.Header.Set("Authorization", test.auth)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatal(test.path, w.Code, w.Body.String())
		}
	}
}

func TestCardOutputPreservedOnMalformedWorkerOutput(t *testing.T) {
	a, e := testAutonomy(t)
	e.workerStatus = "invalid" // causes invalid JSON or non-standard format
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool {
		return len(b.Cards) >= 2 && b.Cards[1].Status == Blocked
	})
	if b.Cards[1].Output == "" {
		t.Fatal("expected malformed worker output to be preserved in c.Output")
	}
}

func TestBoardReaderAssets(t *testing.T) {
	htmlBytes, err := os.ReadFile("web/board.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlBytes)
	for _, required := range []string{"reader-formatted", "reader-raw", "reader-copy", "reader-download", "reader-toggle-raw", "chat-form"} {
		if !strings.Contains(html, required) {
			t.Fatalf("missing %s in board.html", required)
		}
	}
	jsBytes, err := os.ReadFile("web/board.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	for _, required := range []string{"extractResult", "formatContent", "statusBadge", "reader-copy", "reader-download"} {
		if !strings.Contains(js, required) {
			t.Fatalf("missing %s in board.js", required)
		}
	}
}

func TestDirectGoalSkipsPlanningAndAcceptsPlainText(t *testing.T) {
	a, e := testAutonomy(t)
	e.plain = "Roses are red"
	req := goalRequest()
	req.Direct, req.Manager = true, ""
	if _, err := a.Create(req); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	if len(b.Cards) != 1 || b.Cards[0].Output != "Roses are red" || b.Goals[0].Runs != 1 {
		t.Fatalf("bad direct result: %+v", b)
	}
}

func TestInvalidPlanIsCorrectedOnceWithSpecificFeedback(t *testing.T) {
	a, e := testAutonomy(t)
	e.invalidPlan, e.fixOnRetry = true, true
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	if b.Cards[0].Feedback == "" || !strings.Contains(b.Cards[0].Feedback, `"stranger" is not allowed; use exactly one of: echo`) {
		t.Fatalf("missing specific feedback: %q", b.Cards[0].Feedback)
	}
}

func TestPlanStillInvalidAfterCorrectionBlocksWithReason(t *testing.T) {
	a, e := testAutonomy(t)
	e.invalidPlan = true
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Cards[0].Status == Blocked })
	if len(b.Cards) != 1 || b.Goals[0].Runs != 2 || !strings.Contains(b.Cards[0].History[len(b.Cards[0].History)-1].Text, "not allowed") {
		t.Fatalf("bad block: %+v", b)
	}
}

func TestPlanAcceptsStructuredRequirementsFromSmallModels(t *testing.T) {
	var plan struct {
		Tasks []planTask `json:"tasks"`
	}
	raw := `{"tasks":[{"title":"Poem","requirements":{"poem":"3 stanzas","tone":["calm","vivid"]},"acceptance":"A poem exists","owner":"echo"}]}`
	if err := decodeBotOutput(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if got := string(plan.Tasks[0].Requirements); got != "poem: 3 stanzas; tone: calm; vivid" {
		t.Fatal(got)
	}
}

func TestDelegatedWorkerPlainTextIsTheResult(t *testing.T) {
	a, e := testAutonomy(t)
	e.plain = "Waves fold like pages"
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	if b.Cards[1].Output != "Waves fold like pages" {
		t.Fatalf("bad output: %q", b.Cards[1].Output)
	}
}

func TestManagerPromptOmitsOperationalHistory(t *testing.T) {
	a, _ := testAutonomy(t)
	req := goalRequest()
	if _, err := a.Create(req); err != nil {
		t.Fatal(err)
	}
	b, _ := a.Snapshot()
	g, c := b.Goals[0], b.Cards[0]
	c.Phase = "work"
	c.History = append(c.History, message("manager", strings.Repeat("dispatch telemetry ", 1000)))
	g.Chat = append(g.Chat, message("user", "Use three stanzas."))
	prompt := a.prompt(b, g, c)
	if strings.Contains(prompt, "Return ONLY the requested JSON") {
		t.Fatal("worker deliverable forced into JSON")
	}
	if strings.Contains(prompt, "dispatch telemetry") || !strings.Contains(prompt, "Use three stanzas.") || !strings.Contains(prompt, g.Requirements) {
		t.Fatal("irrelevant history or missing task context")
	}
}

func TestGoalGetsPrivateWorkspaceAndVersionedDeliverableCounts(t *testing.T) {
	a, _ := testAutonomy(t)
	a.q.runner.WorkspaceDir = t.TempDir()
	g, err := a.Create(goalRequest())
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(a.q.runner.WorkspaceDir, g.ID); g.Workspace != want {
		t.Fatal(g.Workspace)
	}
	if info, err := os.Stat(g.Workspace); err != nil || !info.IsDir() {
		t.Fatal("workspace not created", err)
	}
	if b, _ := json.Marshal(Request{Workspace: "C:/evil"}); strings.Contains(string(b), "evil") {
		t.Fatal("clients must not be able to set the workspace")
	}
}

func TestReviewEvidenceDoesNotRepeatFileContents(t *testing.T) {
	big := strings.Repeat("<p>assignment answer</p>", 400)
	args, _ := json.Marshal(map[string]any{"path": "out/index.html", "content": big})
	event, _ := json.Marshal(map[string]any{"kind": "tool_finished", "call": map[string]any{"function": map[string]any{"name": "file-write", "arguments": string(args)}}, "result": map[string]any{"output": "created out/index.html"}})
	got := toolEvidence(event)
	if len(got) > 100 || !strings.Contains(got, "out/index.html") || !strings.Contains(got, "9600 bytes") || strings.Contains(got, "assignment answer") {
		t.Fatalf("evidence repeats content or lacks facts: %q", got)
	}
}

func TestVerifyPromptCarriesFileOnceNotInToolEvidence(t *testing.T) {
	a, _ := testAutonomy(t)
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b, _ := a.Snapshot()
	big := strings.Repeat("<p>assignment answer</p>", 400)
	args, _ := json.Marshal(map[string]any{"path": "out/index.html", "content": big})
	run := state.Run{ID: "run-big", Session: "s", Bot: "echo", Status: "completed"}
	event := map[string]any{"kind": "tool_finished", "call": map[string]any{"function": map[string]any{"name": "file-write", "arguments": string(args)}}, "result": map[string]any{"output": "created out/index.html"}}
	if err := a.q.runner.Store.Event(run.ID, event); err != nil {
		t.Fatal(err)
	}
	a.q.mu.Lock()
	a.q.jobs["job-big"] = Job{ID: "job-big", Bot: "echo", Run: &run}
	a.q.mu.Unlock()
	c := b.Cards[0]
	c.Phase, c.Owner, c.Jobs, c.Output = "verify", "echo", []string{"job-big"}, big
	prompt := a.prompt(b, b.Goals[0], c)
	if strings.Count(prompt, "assignment answer") != 400 || !strings.Contains(prompt, "file-write out/index.html: ok (9600 bytes)") {
		t.Fatalf("file content duplicated or evidence missing (copies=%d)", strings.Count(prompt, "assignment answer"))
	}
}

// Count simulates a review prompt that cannot fit unless it is the excerpt version.
func (e *boardEngine) Count(_ context.Context, r provider.Request) (int, error) {
	prompt := r.Messages[len(r.Messages)-1].Content
	if strings.Contains(prompt, "Verify the card") && (e.alwaysBig || e.bigReview && !strings.Contains(prompt, "only its beginning and end are shown")) {
		return 1 << 20, nil
	}
	return 10, nil
}

func TestExcerptKeepsHeadAndTailAndStatesOmission(t *testing.T) {
	text := strings.Repeat("a", 3000) + strings.Repeat("é", 1000) + strings.Repeat("z", 3000)
	got := excerpt(text, 3000)
	if !strings.HasPrefix(got, strings.Repeat("a", 2000)) || !strings.HasSuffix(got, strings.Repeat("z", 1000)) || !strings.Contains(got, "characters omitted for review") || !utf8.ValidString(got) {
		t.Fatalf("bad excerpt: %.80q", got)
	}
	if excerpt("short", 3000) != "short" {
		t.Fatal("short text must be unchanged")
	}
}

func TestOversizedReviewFallsBackToExcerptAndNeedsSignOff(t *testing.T) {
	a, e := testAutonomy(t)
	e.bigReview = true
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return len(b.Cards) == 2 && b.Cards[1].Status == Approval })
	c := b.Cards[1]
	if c.Gate != "done" || c.Shrink != 1 || b.Goals[0].Runs != 4 {
		t.Fatalf("bad fallback state: gate=%q shrink=%d runs=%d", c.Gate, c.Shrink, b.Goals[0].Runs)
	}
	for i := 0; i < 5; i++ {
		a.mu.Lock()
		_ = a.tick()
		a.mu.Unlock()
	}
	if after, _ := a.Snapshot(); after.Cards[1].Status != Approval {
		t.Fatal("excerpt-reviewed result was auto-approved")
	}
	if err := a.Action(c.ID, "approve", "read it"); err != nil {
		t.Fatal(err)
	}
	if done, _ := a.Snapshot(); done.Cards[1].Status != Done {
		t.Fatal("user approval ignored")
	}
}

func TestReviewStillTooLargeAfterEveryExcerptBlocksWithReason(t *testing.T) {
	a, e := testAutonomy(t)
	e.alwaysBig = true
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return len(b.Cards) == 2 && b.Cards[1].Status == Blocked })
	c := b.Cards[1]
	if c.Shrink != maxReviewShrink || b.Goals[0].Runs != 2+1+maxReviewShrink || !strings.Contains(c.History[len(c.History)-1].Text, "Job failed") {
		t.Fatalf("bad block: shrink=%d runs=%d last=%q", c.Shrink, b.Goals[0].Runs, c.History[len(c.History)-1].Text)
	}
}

func TestRestatesTask(t *testing.T) {
	card := Card{TaskPlan: TaskPlan{Title: "Write a complex task plan", Requirements: "Suggest a complicated task that will generate at least 15 sub-tasks", Acceptance: "A detailed task plan with at least 15 sub-tasks, each assigned to a specific worker, and acceptance criteria for each sub-task"}}
	for output, want := range map[string]bool{
		"A detailed task plan with at least 15 sub-tasks, each assigned to a specific worker, and acceptance criteria for each sub-task":        true,
		"  a detailed TASK plan with at least 15 sub-tasks,\n each assigned to a specific worker, and acceptance criteria for each sub-task.  ": true,
		"Write a complex task plan":                          true,
		"1. Design the schema\n2. Build the API\n3. Test it": false,
		"42": false,
		"":   false,
	} {
		if got := restatesTask(output, card); got != want {
			t.Errorf("restatesTask(%.40q) = %v, want %v", output, got, want)
		}
	}
}

func TestWorkerRepeatingItsTaskIsRetriedOnceThenBlocked(t *testing.T) {
	a, e := testAutonomy(t)
	e.plain = "Answer equals 42"
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return len(b.Cards) == 2 && b.Cards[1].Status == Blocked })
	c := b.Cards[1]
	if b.Goals[0].Runs != 3 || c.Feedback == "" || !strings.Contains(c.History[len(c.History)-1].Text, "kept repeating the task") {
		t.Fatalf("bad block: runs=%d last=%q", b.Goals[0].Runs, c.History[len(c.History)-1].Text)
	}
}

func TestWorkerRepeatingOnceThenDoingTheTaskSucceeds(t *testing.T) {
	a, e := testAutonomy(t)
	e.plain, e.echoFirst = "Answer equals 42", true
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b := driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	if c := b.Cards[1]; c.Output != "Verified answer: 42" || c.Feedback != "" || b.Goals[0].Runs != 4 {
		t.Fatalf("bad result: output=%q feedback=%q runs=%d", c.Output, c.Feedback, b.Goals[0].Runs)
	}
}

func TestPlanPromptDescribesWorkersAndForbidsManagerOwner(t *testing.T) {
	a, _ := testAutonomy(t)
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b, _ := a.Snapshot()
	prompt := a.prompt(b, b.Goals[0], b.Cards[0])
	for _, want := range []string{"- echo: ", "never use manager", "ONE task", "use writer when unsure"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("plan prompt missing %q", want)
		}
	}
}

func TestMergeSameOwnerJoinsRepeatedWorkerSteps(t *testing.T) {
	got := mergeSameOwner([]TaskPlan{
		{Title: "Design", Requirements: "design it", Acceptance: "designed", Owner: "landing-page"},
		{Title: "Code", Requirements: "code it", Acceptance: "coded", Owner: "landing-page"},
		{Title: "Announce", Requirements: "write post", Acceptance: "post exists", Owner: "writer"},
		{Title: "Edit", Requirements: "polish", Acceptance: "polished", Owner: "writer"},
		{Title: "Review", Requirements: "check", Acceptance: "checked", Owner: "qa-assistant"},
	})
	if len(got) != 3 || got[0].Title != "Design" || got[0].Requirements != "1. design it\n2. code it" || got[1].Acceptance != "1. post exists\n2. polished" || got[2].Requirements != "check" {
		t.Fatalf("bad merge: %+v", got)
	}
	if again := mergeSameOwner(got); len(again) != 3 {
		t.Fatal("different owners must stay separate")
	}
}

func TestManagerOutputIsSchemaConstrainedAndWorkersAreNot(t *testing.T) {
	a, e := testAutonomy(t)
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	driveBoard(t, a, func(b Board) bool { return b.Goals[0].Status == Done })
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.requests) != 3 {
		t.Fatalf("expected plan, work, verify requests, got %d", len(e.requests))
	}
	var plan, verdict struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if len(e.requests[0].Schema) == 0 || json.Unmarshal(e.requests[0].Schema, &plan) != nil || plan.Properties["tasks"] == nil || !strings.Contains(string(e.requests[0].Schema), `"enum":["echo"]`) {
		t.Fatalf("plan schema missing or does not restrict owners: %s", e.requests[0].Schema)
	}
	if len(e.requests[1].Schema) != 0 {
		t.Fatal("worker replies must stay free text")
	}
	if json.Unmarshal(e.requests[2].Schema, &verdict) != nil || verdict.Properties["approved"] == nil || !strings.Contains(string(e.requests[2].Schema), `"maxLength"`) {
		t.Fatalf("verdict schema missing or unbounded: %s", e.requests[2].Schema)
	}
}

func TestDirectGoalsAreNotSchemaConstrained(t *testing.T) {
	if outputSchema(Card{Phase: "plan"}, Goal{GoalRequest: GoalRequest{Direct: true}}) != nil {
		t.Fatal("direct goals have no manager output")
	}
}

func TestPlanPromptDoesNotShowThePlanCardsOwnFields(t *testing.T) {
	a, _ := testAutonomy(t)
	if _, err := a.Create(goalRequest()); err != nil {
		t.Fatal(err)
	}
	b, _ := a.Snapshot()
	prompt := a.prompt(b, b.Goals[0], b.Cards[0])
	if strings.Contains(prompt, "Plan goal") || strings.Contains(prompt, "Concrete tasks with assigned workers and acceptance criteria") {
		t.Fatal("the planner is shown its own housekeeping text and copies it into plans")
	}
	b.Cards[0].Phase = "verify"
	if prompt = a.prompt(b, b.Goals[0], b.Cards[0]); !strings.Contains(prompt, `"task":`) {
		t.Fatal("reviewers still need the task")
	}
}
