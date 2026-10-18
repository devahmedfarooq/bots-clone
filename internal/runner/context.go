package runner

import (
	"bots/internal/provider"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrContextBudget = errors.New("instructions/current task exceed context after reserving output and tool space")

func fit(ctx context.Context, p provider.Provider, request provider.Request, current int) (provider.Request, int, error) {
	messages := append([]provider.Message{}, request.Messages...)
	request.Messages = messages
	for {
		if e := ctx.Err(); e != nil {
			return request, 0, e
		}
		prompt, e := p.Count(ctx, request)
		if e != nil {
			return request, 0, e
		}
		reserve := request.Settings.MaxTokens
		if len(request.Tools) > 0 {
			reserve += min(256, request.Settings.ContextSize/8)
		}
		if prompt+reserve <= request.Settings.ContextSize {
			return request, prompt, nil
		}
		if current > 1 {
			end := current
			for i := 2; i < current; i++ {
				if messages[i].Role == "user" && !strings.HasPrefix(messages[i].Content, "{\"kind\":\"untrusted_context\"") {
					end = i
					break
				}
			}
			messages = append(messages[:1], messages[end:]...)
			current -= end - 1
			request.Messages = messages
			continue
		}
		changed := false
		for i := 1; i < len(messages); i++ {
			m := &messages[i]
			if m.Role == "tool" && len(m.Content) > 256 {
				preview := m.Content[:len(m.Content)/4]
				for !utf8.ValidString(preview) {
					preview = preview[:len(preview)-1]
				}
				data, _ := json.Marshal(map[string]any{"truncated": true, "untrusted_tool_preview": preview})
				if len(data) >= len(m.Content) {
					data = []byte(`{"truncated":true,"notice":"Tool result omitted to fit context; full result retained in trace."}`)
				}
				m.Content = string(data)
				changed = true
				break
			}
			if m.Role == "user" && i == current+1 && strings.HasPrefix(m.Content, "{\"kind\":\"untrusted_context\"") && len(m.Content) > 256 {
				m.Content = `{"kind":"untrusted_context","truncated":true,"sources":[]}`
				changed = true
				break
			}
		}
		if !changed {
			return request, 0, ErrContextBudget
		}
	}
}
