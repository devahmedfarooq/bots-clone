package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// squash lower-cases text and collapses whitespace so comparisons ignore formatting.
func squash(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func trimTask(s string) string { return squash(strings.Trim(s, " \t\r\n.\"'`")) }

// restatesTask reports whether a worker only repeated its own task text instead of doing the task.
func restatesTask(output string, c Card) bool {
	out := trimTask(output)
	if out == "" {
		return false
	}
	for _, reference := range []string{c.Acceptance, c.Requirements, c.Title} {
		ref := trimTask(reference)
		switch {
		case ref == "":
		case out == ref:
			return true
		case len(ref) >= 12 && len(out) >= 20 && strings.Contains(ref, out):
			return true
		case len(ref) >= 12 && strings.Contains(out, ref) && len(out) < len(ref)*3/2:
			return true
		}
	}
	return false
}

// rework gives a worker that repeated its task one more attempt, then blocks with a clear reason.
func (a *Autonomy) rework(b Board, i int, output string) error {
	c := &b.Cards[i]
	c.Output = output
	if c.Feedback != "" {
		block(c, "The assistant kept repeating the task instead of doing it. Retry with a clearer request or choose another assistant.")
		return a.save(b)
	}
	c.Feedback, c.Status, c.Key, c.Job = "Your reply repeated the task description. Do the task itself and reply with the finished result.", Ready, "", ""
	record(c, "manager", "The assistant repeated the task instead of doing it. Asking once more.")
	return a.save(b)
}

// mergeSameOwner joins consecutive tasks for one worker: separate runs of the same worker only repeat work.
func mergeSameOwner(tasks []TaskPlan) []TaskPlan {
	var merged []TaskPlan
	var steps []int
	for _, t := range tasks {
		last := len(merged) - 1
		if last < 0 || merged[last].Owner != t.Owner {
			merged, steps = append(merged, t), append(steps, 1)
			continue
		}
		steps[last]++
		if steps[last] == 2 {
			merged[last].Requirements = "1. " + merged[last].Requirements
			merged[last].Acceptance = "1. " + merged[last].Acceptance
		}
		merged[last].Requirements += fmt.Sprintf("\n%d. %s", steps[last], t.Requirements)
		merged[last].Acceptance += fmt.Sprintf("\n%d. %s", steps[last], t.Acceptance)
	}
	return merged
}

// roleHints are short keyword lists; full descriptions were copied by the planner into task requirements.
var roleHints = map[string]string{
	"writer":       "text, ideas, suggestions, plans, posts, articles, poems",
	"landing-page": "web pages, HTML and CSS",
	"code-writer":  "programs and scripts",
	"math-solver":  "math problems",
	"qa-assistant": "reviews, tests and audits",
}

// workerList names each allowed worker with a short role so the planner can match tasks to specialists.
func (a *Autonomy) workerList(g Goal) string {
	lines := []string{}
	for _, w := range a.capabilities(g) {
		hint, ok := roleHints[w.Owner]
		if !ok {
			words := strings.Fields(w.Requirements)
			hint = strings.Join(words[:min(len(words), 5)], " ")
		}
		lines = append(lines, "- "+w.Owner+" ("+hint+")")
	}
	return strings.Join(lines, "\n")
}

// outputSchema constrains the manager's decoding: valid JSON, bounded fields, owners limited to real workers.
// Property order is the generation order, so each field is written after the ones it depends on.
func outputSchema(c Card, g Goal) json.RawMessage {
	if g.Direct {
		return nil
	}
	switch c.Phase {
	case "plan":
		owners, _ := json.Marshal(g.Workers)
		return json.RawMessage(fmt.Sprintf(`{"type":"object","additionalProperties":false,"required":["tasks"],"properties":{"tasks":{"type":"array","minItems":1,"maxItems":12,"items":{"type":"object","additionalProperties":false,"required":["title","requirements","acceptance","owner"],"properties":{"title":{"type":"string","maxLength":200},"requirements":{"type":"string","maxLength":2000},"acceptance":{"type":"string","maxLength":600},"owner":{"type":"string","enum":%s}}}}}}`, owners))
	case "verify":
		return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["reason","approved"],"properties":{"reason":{"type":"string","maxLength":400},"approved":{"type":"boolean"}}}`)
	}
	return nil
}
