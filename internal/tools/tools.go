package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bots/internal/config"
	"bots/internal/provider"
	"bots/internal/state"
)

type Tool interface {
	Definition() provider.ToolDefinition
	Validate(json.RawMessage) error
	Execute(context.Context, json.RawMessage) (string, error)
}
type Result struct {
	Output    string   `json:"output,omitempty"`
	Error     *Failure `json:"error,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func failure(code string, e error) Result {
	return Result{Error: &Failure{code, state.Redact(e.Error())}}
}

type Registry struct {
	items       map[string]Tool
	Timeout     time.Duration
	OutputLimit int
}

func New() *Registry {
	return &Registry{items: map[string]Tool{}, Timeout: 5 * time.Second, OutputLimit: 16384}
}
func (r *Registry) Register(t Tool) error {
	d := t.Definition()
	if !config.ValidID(d.Name) || r.items[d.Name] != nil {
		return errors.New("invalid or duplicate tool")
	}
	r.items[d.Name] = t
	return nil
}
func (r *Registry) Definitions(grants []string) ([]provider.Tool, error) {
	out := []provider.Tool{}
	for _, g := range grants {
		t := r.items[g]
		if t == nil {
			return nil, fmt.Errorf("unknown tool grant %q", g)
		}
		out = append(out, provider.Tool{Type: "function", Function: t.Definition()})
	}
	return out, nil
}
func (r *Registry) Call(ctx context.Context, grants []string, c provider.Call) Result {
	t := r.items[c.Function.Name]
	if t == nil {
		return failure("unknown_tool", errors.New("unknown tool"))
	}
	allowed := false
	for _, g := range grants {
		if g == c.Function.Name {
			allowed = true
		}
	}
	if !allowed {
		return failure("permission_denied", errors.New("tool is not granted"))
	}
	args := json.RawMessage(c.Function.Arguments)
	if e := t.Validate(args); e != nil {
		return failure("invalid_arguments", e)
	}
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	ch := make(chan Result, 1)
	go func() {
		defer func() {
			if recover() != nil {
				ch <- failure("tool_failure", errors.New("tool panicked"))
			}
		}()
		output, e := t.Execute(ctx, args)
		if e != nil {
			ch <- failure("tool_failure", e)
			return
		}
		output = state.Redact(output)
		result := Result{Output: output}
		if len(output) > r.OutputLimit {
			result.Output = output[:r.OutputLimit]
			result.Truncated = true
		}
		ch <- result
	}()
	select {
	case <-ctx.Done():
		return failure("timeout", ctx.Err())
	case result := <-ch:
		if ctx.Err() != nil {
			return failure("timeout", ctx.Err())
		}
		return result
	}
}
