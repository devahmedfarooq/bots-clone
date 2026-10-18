package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bots/internal/config"
	"bots/internal/provider"
	"bots/internal/state"
	"bots/internal/tools"
)

type Runner struct {
	Models *provider.Manager
	Store  *state.Store
}
type Options struct {
	Session  string
	Settings config.Settings
	Emit     func(string)
	Started  func(state.Run) error
}

func (r *Runner) Run(parent context.Context, b config.Bot, input string, o Options) (run state.Run, err error) {
	if e := b.Validate(); e != nil {
		return run, e
	}
	if len(input) == 0 || len(input) > 1<<20 {
		return run, errors.New("input must be 1..1048576 bytes")
	}
	settings, e := config.Resolve(b.Settings, o.Settings)
	if e != nil {
		return run, e
	}
	registry, closeTools, e := tools.Files(b.ContextSources)
	if e != nil {
		return run, e
	}
	defer closeTools()
	definitions, e := registry.Definitions(b.Tools)
	if e != nil {
		return run, e
	}
	session, history, e := r.Store.Session(o.Session, b.ID, b.Version)
	if e != nil {
		return run, e
	}
	started := time.Now()
	run = state.Run{ID: state.ID(), Session: session, Bot: b.ID, Version: b.Version, Status: "running", Started: started}
	if e = r.Store.Save(run); e != nil {
		return run, e
	}
	ctx, cancel := context.WithTimeout(parent, b.Budgets.Duration())
	defer cancel()
	var release func()
	defer func() {
		defer func() {
			if release != nil {
				release()
			}
		}()
		run.DurationMS = time.Since(started).Milliseconds()
		if err != nil {
			run.Status = "failed"
			run.Error = state.Redact(err.Error())
			if run.Reason == "" {
				run.Reason = "failure"
			}
		}
		if ctx.Err() != nil {
			if parent.Err() != nil {
				run.Status = "cancelled"
				run.Reason = "cancelled"
			} else {
				run.Status = "failed"
				run.Reason = "time_budget"
			}
			if err == nil {
				err = ctx.Err()
				run.Error = err.Error()
			}
		}
		if saveErr := r.Store.Save(run); saveErr != nil {
			err = errors.Join(err, saveErr)
		}
	}()
	if o.Started != nil {
		if e = o.Started(run); e != nil {
			return run, e
		}
	}
	release, e = r.Models.Acquire(ctx, provider.Model{ID: b.BaseModel, ContextSize: settings.ContextSize})
	if e != nil {
		return run, e
	}
	_, history, e = r.Store.Session(session, b.ID, b.Version)
	if e != nil {
		return run, e
	}
	messages := []provider.Message{{Role: "system", Content: b.Instructions}}
	messages = append(messages, history...)
	add := func(m provider.Message) error {
		messages = append(messages, m)
		return r.Store.Message(run.ID, session, m)
	}
	if e = add(provider.Message{Role: "user", Content: input}); e != nil {
		return run, e
	}
	toolCount := 0
	for {
		if e = ctx.Err(); e != nil {
			return run, e
		}
		request := provider.Request{Model: b.BaseModel, Messages: messages, Tools: definitions, Settings: settings}
		prompt, e := r.Models.Provider.Count(ctx, request)
		if e != nil {
			return run, e
		}
		remaining := b.Budgets.Tokens - run.Usage.Total - prompt
		if remaining < 1 {
			run.Reason = "token_budget"
			return run, errors.New("token budget exhausted before generation")
		}
		space := settings.ContextSize - prompt
		if space < 1 {
			run.Reason = "context_budget"
			return run, errors.New("prompt exceeds context budget")
		}
		request.Settings.MaxTokens = min(settings.MaxTokens, remaining, space)
		var response provider.Response
		for attempt := 0; attempt < 2; attempt++ {
			response, e = r.Models.Provider.Generate(ctx, request, o.Emit)
			if e == nil || ctx.Err() != nil || o.Emit != nil {
				break
			}
			if attempt == 0 {
				select {
				case <-ctx.Done():
					return run, ctx.Err()
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
		if e != nil {
			return run, fmt.Errorf("generation failed: %w", e)
		}
		if response.Usage.Total <= 0 || response.Usage.Prompt < 0 || response.Usage.Completion < 0 || response.Usage.Total != response.Usage.Prompt+response.Usage.Completion {
			return run, errors.New("invalid model usage")
		}
		run.Usage.Prompt += response.Usage.Prompt
		run.Usage.Completion += response.Usage.Completion
		run.Usage.Total += response.Usage.Total
		if run.Usage.Total > b.Budgets.Tokens {
			run.Reason = "token_budget"
			return run, errors.New("provider exceeded token budget")
		}
		m := response.Message
		if m.Role != "assistant" {
			return run, errors.New("invalid assistant role")
		}
		if response.Finish == "length" {
			run.Output = m.Content
			run.Reason = "generation_limit"
			return run, errors.New("generation limit reached")
		}
		if len(m.ToolCalls) == 0 {
			if response.Finish != "stop" || m.Content == "" {
				return run, errors.New("invalid final model response")
			}
			if e = add(m); e != nil {
				return run, e
			}
			run.Output = m.Content
			run.Status = "completed"
			run.Reason = "stop"
			return run, nil
		}
		if response.Finish != "tool_calls" && response.Finish != "stop" {
			return run, errors.New("invalid tool finish reason")
		}
		seen := map[string]bool{}
		for _, c := range m.ToolCalls {
			if c.ID == "" || seen[c.ID] || c.Type != "function" {
				return run, errors.New("invalid tool call envelope")
			}
			seen[c.ID] = true
		}
		if e = add(m); e != nil {
			return run, e
		}
		for _, call := range m.ToolCalls {
			if toolCount >= b.Budgets.ToolCalls {
				run.Reason = "tool_budget"
				return run, errors.New("tool-call budget exhausted")
			}
			toolCount++
			result := registry.Call(ctx, b.Tools, call)
			if e = r.Store.Event(run.ID, map[string]any{"call": call, "result": result}); e != nil {
				return run, e
			}
			data, e := json.Marshal(result)
			if e != nil {
				return run, e
			}
			if e = add(provider.Message{Role: "tool", Content: string(data), ToolCallID: call.ID}); e != nil {
				return run, e
			}
		}
	}
}
