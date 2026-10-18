package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bots/internal/config"
	"bots/internal/contextsource"
	"bots/internal/provider"
	"bots/internal/state"
	"bots/internal/tools"
	"slices"
)

type Runner struct {
	ContextLimit int
	// WorkspaceDir is the parent of per-goal workspaces; empty keeps bot sources only.
	WorkspaceDir string
	Models       *provider.Manager
	Store        *state.Store
	Plugins      []tools.Tool
}
type Options struct {
	AdaptiveContext bool
	Schema          json.RawMessage
	Workspace       string
	Session         string
	Settings        config.Settings
	Emit            func(string)
	Started         func(state.Run) error
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
	var registry *tools.Registry
	var closeTools func()
	if o.Workspace != "" {
		registry, closeTools, e = tools.WithWorkspace(o.Workspace, b.ContextSources, r.Plugins)
	} else {
		registry, closeTools, e = tools.WithPlugins(b.ContextSources, r.Plugins)
	}
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
	run = state.Run{ID: state.ID(), Session: session, Bot: b.ID, Version: b.Version, Model: b.BaseModel, Adapter: b.Adapter, Status: "running", Started: started}
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
		if traceErr := r.Store.Event(run.ID, map[string]any{"kind": "termination", "status": run.Status, "reason": run.Reason, "error": run.Error, "duration_ms": run.DurationMS, "usage": run.Usage}); traceErr != nil {
			err = errors.Join(err, traceErr)
		}
		if saveErr := r.Store.Save(run); saveErr != nil {
			err = errors.Join(err, saveErr)
		}
		run.Output = state.Redact(run.Output)
		run.Error = state.Redact(run.Error)
	}()
	if o.Started != nil {
		if e = o.Started(run); e != nil {
			return run, e
		}
	}
	if e = r.Store.Event(run.ID, map[string]any{"kind": "run_start", "base_model": b.BaseModel, "adapter": b.Adapter, "bot_version": b.Version, "effective_settings": settings}); e != nil {
		return run, e
	}
	acquireStarted := time.Now()
	release, e = r.Models.Acquire(ctx, provider.Model{ID: b.BaseModel, ContextSize: settings.ContextSize, Adapter: b.Adapter})
	if e != nil {
		return run, e
	}
	if e = r.Store.Event(run.ID, map[string]any{"kind": "model_ready", "acquire_ms": time.Since(acquireStarted).Milliseconds()}); e != nil {
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
	current := len(messages)
	if e = add(provider.Message{Role: "user", Content: input}); e != nil {
		return run, e
	}
	excerpts, e := (contextsource.Files{Roots: b.ContextSources, Sources: b.Retrieval, Allowed: slices.Contains(b.Tools, "file-read")}).Retrieve(ctx, input, min(16384, settings.ContextSize*2))
	if e != nil {
		return run, e
	}
	if len(excerpts) > 0 {
		if e = add(provider.Message{Role: "user", Content: contextsource.Envelope(excerpts)}); e != nil {
			return run, e
		}
	}
	toolCount := 0
	repeats, results := map[string]int{}, map[string]tools.Result{}
	for {
		if e = ctx.Err(); e != nil {
			return run, e
		}
		request := provider.Request{Model: b.BaseModel, Messages: messages, Tools: definitions, Settings: settings, Schema: o.Schema}
		request, prompt, e := fit(ctx, r.Models.Provider, request, current)
		if errors.Is(e, ErrContextBudget) && o.AdaptiveContext {
			limit := min(r.ContextLimit, b.Budgets.Tokens-run.Usage.Total)
			if limit > settings.ContextSize {
				previous := settings.ContextSize
				settings.ContextSize = min(settings.ContextSize*2, limit)
				release()
				release = nil
				release, e = r.Models.Acquire(ctx, provider.Model{ID: b.BaseModel, ContextSize: settings.ContextSize, Adapter: b.Adapter})
				if e != nil {
					return run, fmt.Errorf("context allocation failed: %w", e)
				}
				if e = r.Store.Event(run.ID, map[string]any{"kind": "context_allocated", "previous": previous, "context_size": settings.ContextSize, "max_tokens": settings.MaxTokens}); e != nil {
					return run, e
				}
				continue
			}
			e = fmt.Errorf("%w (context %d, engine maximum %d, remaining run tokens %d). Increase engine.context_size or Tokens per run, or shorten the task", e, settings.ContextSize, r.ContextLimit, b.Budgets.Tokens-run.Usage.Total)
		}
		if e != nil {
			run.Reason = "context_budget"
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
		m.Content = state.Redact(m.Content)
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
			toolStarted := time.Now()
			if e = r.Store.Event(run.ID, map[string]any{"kind": "tool_started", "call": call, "at": toolStarted.UTC()}); e != nil {
				return run, e
			}
			key := call.Function.Name + "\x00" + call.Function.Arguments
			repeats[key]++
			result, seen := results[key]
			if !seen || result.Error != nil {
				result = registry.Call(ctx, b.Tools, call)
				results[key] = result
			} else {
				result = tools.Result{Output: "Duplicate call ignored: this exact call already succeeded. Reply now with the finished result and make no more tool calls."}
			}
			if e = r.Store.Event(run.ID, map[string]any{"kind": "tool_finished", "call": call, "result": result, "duration_ms": time.Since(toolStarted).Milliseconds()}); e != nil {
				return run, e
			}
			data, e := json.Marshal(result)
			if e != nil {
				return run, e
			}
			if e = add(provider.Message{Role: "tool", Content: string(data), ToolCallID: call.ID}); e != nil {
				return run, e
			}
			if repeats[key] > maxRepeatedCalls {
				run.Status, run.Reason = "completed", "repeated_tool_call"
				return run, nil
			}
		}
	}
}

// maxRepeatedCalls ends a run whose model keeps issuing the same tool call instead of replying; the work already done stands.
const maxRepeatedCalls = 2
