package provider

import (
	"context"
	"encoding/json"

	"bots/internal/config"
)

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type Call struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCalls  []Call `json:"tool_calls,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}
type Tool struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
type Usage struct {
	Prompt     int `json:"prompt_tokens"`
	Completion int `json:"completion_tokens"`
	Total      int `json:"total_tokens"`
}
type Request struct {
	Model    string
	Messages []Message
	Tools    []Tool
	Settings config.Settings
}
type Response struct {
	Message Message
	Usage   Usage
	Finish  string
}
type Provider interface {
	Load(context.Context, string) error
	Count(context.Context, Request) (int, error)
	Generate(context.Context, Request, func(string)) (Response, error)
	Cancel()
	Health(context.Context) error
	Unload(context.Context) error
}
