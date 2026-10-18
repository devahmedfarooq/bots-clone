package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type Settings struct {
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	ContextSize int      `json:"context_size,omitempty"`
}
type Budgets struct {
	Tokens    int `json:"tokens"`
	ToolCalls int `json:"tool_calls"`
	Seconds   int `json:"seconds"`
}
type Bot struct {
	Schema         int      `json:"schema_version"`
	ID             string   `json:"id"`
	Version        string   `json:"version"`
	Description    string   `json:"description"`
	Instructions   string   `json:"instructions"`
	BaseModel      string   `json:"base_model"`
	Adapter        string   `json:"adapter,omitempty"`
	Tools          []string `json:"tools"`
	ContextSources []string `json:"context_sources"`
	Settings       Settings `json:"generation"`
	Budgets        Budgets  `json:"budgets"`
	Evaluation     string   `json:"evaluation_suite"`
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var version = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func ValidID(s string) bool { return identifier.MatchString(s) }
func Decode(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}
func Load(path string) (Bot, error) {
	var b Bot
	data, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	if err = Decode(data, &b); err != nil {
		return b, fmt.Errorf("bot definition: %w", err)
	}
	if err = b.Validate(); err != nil {
		return b, err
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return b, err
	}
	for i, p := range b.ContextSources {
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		p, err = filepath.EvalSymlinks(p)
		if err != nil {
			return b, fmt.Errorf("context_sources: %w", err)
		}
		info, e := os.Stat(p)
		if e != nil || !info.IsDir() {
			return b, errors.New("context_sources must reference existing directories")
		}
		b.ContextSources[i] = p
	}
	if b.Evaluation != "" {
		if !filepath.IsAbs(b.Evaluation) {
			b.Evaluation = filepath.Join(base, b.Evaluation)
		}
		if _, err = os.Stat(b.Evaluation); err != nil {
			return b, fmt.Errorf("evaluation_suite: %w", err)
		}
	}
	return b, nil
}
func (b Bot) Validate() error {
	switch {
	case b.Schema != 1:
		return errors.New("schema_version must be 1")
	case !ValidID(b.ID):
		return errors.New("id must match [a-z][a-z0-9-]{0,63}")
	case !version.MatchString(b.Version):
		return errors.New("version must use major.minor.patch")
	case b.Description == "" || b.Instructions == "" || b.BaseModel == "":
		return errors.New("description, instructions and base_model are required")
	case b.Adapter != "":
		return errors.New("adapters require the unfinished Phase 2 compatibility and provider operations")
	case b.Budgets.Tokens < 1 || b.Budgets.Tokens > 131072 || b.Budgets.ToolCalls < 0 || b.Budgets.ToolCalls > 100 || b.Budgets.Seconds < 1 || b.Budgets.Seconds > 3600:
		return errors.New("budgets: tokens 1..131072, tool_calls 0..100, seconds 1..3600 required")
	}
	seen := map[string]bool{}
	for _, t := range b.Tools {
		if !ValidID(t) || seen[t] {
			return fmt.Errorf("invalid or duplicate tool grant %q", t)
		}
		seen[t] = true
	}
	_, err := Resolve(b.Settings, Settings{})
	return err
}
func Resolve(bot, override Settings) (Settings, error) {
	t := 0.0
	s := Settings{Temperature: &t, MaxTokens: 512, ContextSize: 4096}
	for _, v := range []Settings{bot, override} {
		if v.Temperature != nil {
			s.Temperature = v.Temperature
		}
		if v.MaxTokens != 0 {
			s.MaxTokens = v.MaxTokens
		}
		if v.ContextSize != 0 {
			s.ContextSize = v.ContextSize
		}
	}
	if math.IsNaN(*s.Temperature) || math.IsInf(*s.Temperature, 0) || *s.Temperature < 0 || *s.Temperature > 2 || s.MaxTokens < 1 || s.ContextSize < 512 || s.ContextSize > 131072 || s.MaxTokens >= s.ContextSize {
		return s, errors.New("generation: temperature 0..2, context_size 512..131072, max_tokens 1..<context_size required")
	}
	return s, nil
}
func (b Budgets) Duration() time.Duration { return time.Duration(b.Seconds) * time.Second }
