package service

import (
	"testing"
	"time"

	"bots/internal/workflow"
)

func awaitFlow(t *testing.T, w *Workflows, id string) Flow {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		f, _ := w.Get(id)
		if f.Status != "running" {
			return f
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("workflow did not finish")
	return Flow{}
}
func chain() workflow.Definition {
	return workflow.Definition{Schema: 1, ID: "chain", Steps: []workflow.Step{{ID: "first", Bot: "echo", Input: "hello", InputType: "text", OutputType: "text"}, {ID: "second", Bot: "echo", DependsOn: []string{"first"}, InputFrom: "first", InputType: "text", OutputType: "text"}}}
}
func TestWorkflowExecutionIsolationAndBlockedDependencies(t *testing.T) {
	q, e, _ := testQueue(t, 4)
	close(e.gate)
	w, err := NewWorkflows(q)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	f, err := w.Start(chain())
	if err != nil {
		t.Fatal(err)
	}
	f = awaitFlow(t, w, f.ID)
	if f.Status != "completed" || f.Steps["second"].Output != "hello" {
		t.Fatal(f)
	}
	j1, _ := q.Get(f.Steps["first"].Job)
	j2, _ := q.Get(f.Steps["second"].Job)
	if j1.Run.Session == j2.Run.Session {
		t.Fatal("shared history")
	}
	d := chain()
	d.Steps[0].OutputType = "json"
	d.Steps[1].InputType = "json"
	f, err = w.Start(d)
	if err != nil {
		t.Fatal(err)
	}
	f = awaitFlow(t, w, f.ID)
	if f.Status != "failed" || f.Steps["second"].Status != "blocked" {
		t.Fatal(f)
	}
}
