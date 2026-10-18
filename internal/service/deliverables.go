package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"bots/internal/provider"
	"bots/internal/tools"
)

var fileReceipt = regexp.MustCompile("(?i)^(?:(?:created|saved|wrote|written|updated)\\s+(?:file\\s+)?)?[`\"']?[a-z0-9_./\\\\ -]+\\.(?:md|txt|html?|css|[cm]?js|tsx?|jsx|py|go|json|csv|ya?ml|sql|sh|ps1|xml|svg)[`\"']?[.!]?$")

func isFileReceipt(output string) bool {
	return !strings.ContainsAny(strings.TrimSpace(output), "\r\n") && fileReceipt.MatchString(strings.TrimSpace(output))
}

// Use the successful write's recorded payload, never a model-supplied path or a later filesystem read.
func (a *Autonomy) deliverable(runID, output string) (string, error) {
	var envelope struct {
		Output string `json:"output"`
	}
	if decodeBotOutput(output, &envelope) == nil && strings.TrimSpace(envelope.Output) != "" {
		output = envelope.Output
	}
	events, err := a.q.runner.Store.Trace(runID)
	if err != nil {
		return "", fmt.Errorf("Cannot verify deliverable evidence: %w", err)
	}
	type file struct {
		root          int
		path, content string
	}
	files := []file{}
	for _, raw := range events {
		var event struct {
			Kind   string        `json:"kind"`
			Call   provider.Call `json:"call"`
			Result tools.Result  `json:"result"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			return "", fmt.Errorf("Invalid deliverable evidence: %w", err)
		}
		if event.Kind != "tool_finished" || event.Call.Function.Name != "file-write" || event.Result.Error != nil || event.Result.Truncated {
			continue
		}
		var args tools.FileArgs
		if json.Unmarshal([]byte(event.Call.Function.Arguments), &args) != nil || event.Result.Output != "created "+args.Path {
			continue
		}
		found := false
		for i := range files {
			if files[i].root == args.Root && files[i].path == args.Path {
				files[i].content = args.Content
				found = true
				break
			}
		}
		if !found {
			files = append(files, file{args.Root, args.Path, args.Content})
		}
	}
	if len(files) == 1 && strings.TrimSpace(files[0].content) != "" {
		return files[0].content, nil
	}
	if len(files) > 1 {
		var parts []string
		for _, f := range files {
			if strings.TrimSpace(f.content) != "" {
				parts = append(parts, fmt.Sprintf("## %s\n\n%s", filepath.ToSlash(f.path), f.content))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n"), nil
		}
	}
	if isFileReceipt(output) || len(files) > 0 {
		return "", errors.New("The assistant returned a file receipt without a nonempty saved deliverable. Retry and ask for the complete text or code.")
	}
	return output, nil
}

func (a *Autonomy) recoverDeliverables() error {
	b := cloneBoard(a.board)
	changed := false
	for i := range b.Cards {
		c := &b.Cards[i]
		finished := c.Status == Done || c.Status == Review || (c.Status == Approval && c.Gate == "done")
		if c.Kind != "task" || !finished || !isFileReceipt(c.Output) {
			continue
		}
		recovered := false
		for j := len(c.Jobs) - 1; j >= 0; j-- {
			if c.Phase == "verify" && c.Jobs[j] == c.Job {
				continue
			}
			job, ok := a.q.Get(c.Jobs[j])
			if !ok || job.Bot != c.Owner || job.Status != "completed" || job.Run == nil {
				continue
			}
			text, err := a.deliverable(job.Run.ID, c.Output)
			if err != nil {
				c.Phase = "work"
				block(c, err.Error())
			} else {
				c.Output = text
				record(c, "manager", "Recovered saved deliverable from successful worker tool evidence.")
			}
			recovered = true
			break
		}
		if !recovered {
			c.Phase = "work"
			block(c, "The saved result is only a file receipt and has no completed worker evidence. Retry for the complete deliverable.")
		}
		changed = true
	}
	if changed {
		return a.save(b)
	}
	return nil
}
