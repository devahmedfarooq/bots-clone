package service

import (
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

// citesOutput reports whether the reviewer's evidence is a phrase actually present in the output.
func citesOutput(evidence, output string) bool {
	e, o := squash(evidence), squash(output)
	return e != "" && len(e) >= min(8, len(o)) && strings.Contains(o, e)
}

// workerList describes each allowed worker so the planner can match tasks to specialists.
func (a *Autonomy) workerList(g Goal) string {
	lines := []string{}
	for _, w := range a.capabilities(g) {
		description := w.Requirements
		if len(description) > 110 {
			description = strings.ToValidUTF8(description[:110], "") + "..."
		}
		lines = append(lines, "- "+w.Owner+": "+description)
	}
	return strings.Join(lines, "\n")
}
